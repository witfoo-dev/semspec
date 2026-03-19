// Package gatherers provides context gathering implementations.
package gatherers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// GraphSource represents a single graph endpoint (local or semsource).
type GraphSource struct {
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Phase       string    `json:"phase"`        // seeding, ready, degraded
	EntityCount int       `json:"entity_count"`
	LastSeen    time.Time `json:"last_seen"`
	IsLocal     bool      `json:"is_local"`
}

// sourceManifestResponse matches semsource's /source-manifest/status response.
type sourceManifestResponse struct {
	Namespace     string `json:"namespace"`
	Phase         string `json:"phase"`
	TotalEntities int    `json:"total_entities"`
	Sources       []struct {
		InstanceName string `json:"instance_name"`
		SourceType   string `json:"source_type"`
		Phase        string `json:"phase"`
		EntityCount  int    `json:"entity_count"`
		ErrorCount   int    `json:"error_count"`
	} `json:"sources"`
	Timestamp string `json:"timestamp"`
}

// GraphRegistry tracks available graph endpoints (local + semsource instances).
// Semsource instances are discovered dynamically via manifest polling.
type GraphRegistry struct {
	sources      sync.Map // name → *GraphSource
	localURL     string
	semsourceURL string // base URL for semsource manifest endpoint
	pollInterval time.Duration
	queryTimeout time.Duration
	logger       *slog.Logger
	httpClient   *http.Client

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// GraphRegistryConfig configures the graph source registry.
type GraphRegistryConfig struct {
	// LocalURL is the local graph-gateway endpoint (always present).
	LocalURL string

	// SemsourceURL is the semsource base URL for manifest discovery.
	// Empty means local-only mode (no semsource).
	SemsourceURL string

	// PollInterval is how often to poll semsource manifest (default 30s).
	PollInterval time.Duration

	// QueryTimeout is the per-source timeout for graph queries (default 3s).
	QueryTimeout time.Duration

	Logger *slog.Logger
}

// NewGraphRegistry creates a new graph source registry.
func NewGraphRegistry(cfg GraphRegistryConfig) *GraphRegistry {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 30 * time.Second
	}
	if cfg.QueryTimeout == 0 {
		cfg.QueryTimeout = 3 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	r := &GraphRegistry{
		localURL:     cfg.LocalURL,
		semsourceURL: cfg.SemsourceURL,
		pollInterval: cfg.PollInterval,
		queryTimeout: cfg.QueryTimeout,
		logger:       cfg.Logger.With("component", "graph-registry"),
		httpClient:   &http.Client{Timeout: 5 * time.Second},
	}

	// Local graph is always a source.
	if cfg.LocalURL != "" {
		r.sources.Store("local", &GraphSource{
			Name:    "local",
			URL:     cfg.LocalURL,
			Phase:   "ready",
			IsLocal: true,
		})
	}

	return r
}

// Start begins polling semsource for graph source discovery.
// No-op if no semsource URL is configured.
func (r *GraphRegistry) Start(ctx context.Context) {
	if r.semsourceURL == "" {
		r.logger.Info("No semsource URL configured, local-only mode")
		return
	}

	ctx, r.cancel = context.WithCancel(ctx)
	r.wg.Add(1)
	go r.pollLoop(ctx)
}

// Stop halts the polling loop.
func (r *GraphRegistry) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
}

// ReadySources returns all sources with phase == "ready".
func (r *GraphRegistry) ReadySources() []*GraphSource {
	var ready []*GraphSource
	r.sources.Range(func(_, value any) bool {
		src := value.(*GraphSource)
		if src.Phase == "ready" || src.IsLocal {
			ready = append(ready, src)
		}
		return true
	})
	return ready
}

// AllSources returns all known sources.
func (r *GraphRegistry) AllSources() []*GraphSource {
	var all []*GraphSource
	r.sources.Range(func(_, value any) bool {
		all = append(all, value.(*GraphSource))
		return true
	})
	return all
}

// SemsourceReady returns true if at least one semsource is in "ready" phase.
// Returns true in local-only mode (no semsource configured).
func (r *GraphRegistry) SemsourceReady() bool {
	if r.semsourceURL == "" {
		return true // local-only mode
	}
	ready := false
	r.sources.Range(func(_, value any) bool {
		src := value.(*GraphSource)
		if !src.IsLocal && src.Phase == "ready" {
			ready = true
			return false // stop iteration
		}
		return true
	})
	return ready
}

// WaitForSemsource blocks until semsource is ready or the budget expires.
// Returns nil immediately in local-only mode.
func (r *GraphRegistry) WaitForSemsource(ctx context.Context, budget time.Duration) error {
	if r.semsourceURL == "" {
		return nil
	}

	deadline := time.Now().Add(budget)
	backoff := 1 * time.Second
	maxBackoff := 8 * time.Second

	for {
		// Force a poll before checking.
		r.pollOnce()

		if r.SemsourceReady() {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("semsource not ready after %s", budget)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// SemsourceConfigured returns true if a semsource URL is configured.
func (r *GraphRegistry) SemsourceConfigured() bool {
	return r.semsourceURL != ""
}

// QueryTimeout returns the configured per-source query timeout.
func (r *GraphRegistry) QueryTimeout() time.Duration {
	return r.queryTimeout
}

// pollLoop periodically fetches the semsource manifest.
func (r *GraphRegistry) pollLoop(ctx context.Context) {
	defer r.wg.Done()

	// Initial poll immediately.
	r.pollOnce()

	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.pollOnce()
		}
	}
}

// pollOnce fetches the semsource manifest and updates the source registry.
func (r *GraphRegistry) pollOnce() {
	if r.semsourceURL == "" {
		return
	}

	url := r.semsourceURL + "/source-manifest/status"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		r.logger.Debug("Failed to create manifest request", "url", url, "error", err)
		return
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		r.logger.Debug("Failed to fetch semsource manifest", "url", url, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		r.logger.Debug("Semsource manifest returned non-200", "status", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		r.logger.Debug("Failed to read manifest body", "error", err)
		return
	}

	var manifest sourceManifestResponse
	if err := json.Unmarshal(body, &manifest); err != nil {
		r.logger.Debug("Failed to parse manifest", "error", err)
		return
	}

	// Register/update the semsource as a graph source.
	// The semsource GraphQL endpoint is at the same base URL.
	name := "semsource"
	if manifest.Namespace != "" {
		name = "semsource-" + manifest.Namespace
	}

	r.sources.Store(name, &GraphSource{
		Name:        name,
		URL:         r.semsourceURL,
		Phase:       manifest.Phase,
		EntityCount: manifest.TotalEntities,
		LastSeen:    time.Now(),
	})

	r.logger.Debug("Updated semsource source",
		"name", name,
		"phase", manifest.Phase,
		"entities", manifest.TotalEntities,
		"sources", len(manifest.Sources),
	)

	// Remove stale semsource entries (not seen for 2x poll intervals).
	staleThreshold := time.Now().Add(-2 * r.pollInterval)
	r.sources.Range(func(key, value any) bool {
		src := value.(*GraphSource)
		if !src.IsLocal && src.LastSeen.Before(staleThreshold) {
			r.sources.Delete(key)
			r.logger.Info("Removed stale graph source", "name", src.Name)
		}
		return true
	})
}
