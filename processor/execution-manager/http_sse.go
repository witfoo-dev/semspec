package executionmanager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// SSE event names for execution streams.
const (
	sseEventConnected          = "connected"
	sseEventHeartbeat          = "heartbeat"
	sseEventTaskUpdated        = "task_updated"
	sseEventRequirementUpdated = "requirement_updated"
	sseEventError              = "error"
)

// handleExecutionStream serves an SSE stream for all execution activity
// under a plan slug. It watches the EXECUTION_STATES KV bucket for both
// task and requirement key mutations and emits typed events.
//
// GET /execution-manager/plans/{slug}/stream
func (c *Component) handleExecutionStream(w http.ResponseWriter, r *http.Request, slug string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	c.mu.RLock()
	store := c.store
	c.mu.RUnlock()

	if store == nil || store.kvStore == nil {
		http.Error(w, "Execution state streaming not available", http.StatusServiceUnavailable)
		return
	}

	// Set SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	flusher.Flush()

	// Watch both task and requirement keys for this plan slug.
	taskPattern := "task." + slug + ".>"
	reqPattern := "req." + slug + ".>"

	taskWatcher, err := store.kvStore.Watch(ctx, taskPattern)
	if err != nil {
		c.logger.Error("Failed to create task KV watcher", "slug", slug, "error", err)
		writeSSEEvent(w, flusher, sseEventError, map[string]string{"message": "failed to watch tasks"})
		return
	}
	defer taskWatcher.Stop()

	reqWatcher, err := store.kvStore.Watch(ctx, reqPattern)
	if err != nil {
		c.logger.Error("Failed to create requirement KV watcher", "slug", slug, "error", err)
		writeSSEEvent(w, flusher, sseEventError, map[string]string{"message": "failed to watch requirements"})
		return
	}
	defer reqWatcher.Stop()

	// Send connected event.
	if err := writeSSEEvent(w, flusher, sseEventConnected, map[string]string{"slug": slug}); err != nil {
		return
	}

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	var eventID uint64
	taskSynced, reqSynced := false, false

	for {
		select {
		case <-ctx.Done():
			return

		case <-heartbeat.C:
			eventID++
			if err := writeSSEEventWithID(w, flusher, eventID, sseEventHeartbeat, nil); err != nil {
				return
			}

		case entry, ok := <-taskWatcher.Updates():
			if !ok {
				return
			}
			if entry == nil {
				taskSynced = true
				if reqSynced {
					eventID++
					writeSSEEventWithID(w, flusher, eventID, "sync_complete", nil)
				}
				continue
			}
			if entry.Operation() == jetstream.KeyValueDelete {
				continue
			}
			eventID++
			eventName := sseEventTaskUpdated
			if isTerminalKey(entry.Key(), entry.Value()) {
				eventName = "task_completed"
			}
			if err := writeSSEEventWithID(w, flusher, eventID, eventName, json.RawMessage(entry.Value())); err != nil {
				return
			}

		case entry, ok := <-reqWatcher.Updates():
			if !ok {
				return
			}
			if entry == nil {
				reqSynced = true
				if taskSynced {
					eventID++
					writeSSEEventWithID(w, flusher, eventID, "sync_complete", nil)
				}
				continue
			}
			if entry.Operation() == jetstream.KeyValueDelete {
				continue
			}
			eventID++
			eventName := sseEventRequirementUpdated
			if isTerminalReqKey(entry.Value()) {
				eventName = "requirement_completed"
			}
			if err := writeSSEEventWithID(w, flusher, eventID, eventName, json.RawMessage(entry.Value())); err != nil {
				return
			}
		}
	}
}

// isTerminalKey checks if a task execution KV entry is in a terminal stage.
func isTerminalKey(_ string, value []byte) bool {
	var partial struct {
		Stage string `json:"stage"`
	}
	if json.Unmarshal(value, &partial) == nil {
		switch partial.Stage {
		case "approved", "escalated", "error", "rejected":
			return true
		}
	}
	return false
}

// isTerminalReqKey checks if a requirement execution KV entry is in a terminal stage.
func isTerminalReqKey(value []byte) bool {
	var partial struct {
		Stage string `json:"stage"`
	}
	if json.Unmarshal(value, &partial) == nil {
		switch partial.Stage {
		case "completed", "failed", "error":
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// SSE helpers (duplicated from plan-manager — extract when a third user appears)
// ---------------------------------------------------------------------------

// writeSSEEvent writes a named SSE event with JSON data.
func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, event string, data any) error {
	return writeSSEEventWithID(w, flusher, 0, event, data)
}

// writeSSEEventWithID writes a named SSE event with an optional ID.
func writeSSEEventWithID(w http.ResponseWriter, flusher http.Flusher, id uint64, event string, data any) error {
	if id > 0 {
		if _, err := fmt.Fprintf(w, "id: %d\n", id); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}

	if data != nil {
		var jsonData []byte
		switch v := data.(type) {
		case json.RawMessage:
			jsonData = v
		default:
			var err error
			jsonData, err = json.Marshal(data)
			if err != nil {
				jsonData = []byte(`{}`)
			}
		}
		if _, err := fmt.Fprintf(w, "data: %s\n", jsonData); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprint(w, "data: {}\n"); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprint(w, "\n"); err != nil {
		return err
	}

	flusher.Flush()
	return nil
}
