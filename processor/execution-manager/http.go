package executionmanager

import (
	"net/http"
	"strings"
)

// RegisterHTTPHandlers registers execution-manager HTTP endpoints under prefix.
// Endpoints provide plan-scoped execution state for the human-in-the-loop UI.
//
// Registered routes:
//
//	GET {prefix}plans/{slug}/stream        — SSE stream of all execution updates
//	GET {prefix}plans/{slug}/tasks         — list active task executions
//	GET {prefix}plans/{slug}/requirements  — list active requirement executions
func (c *Component) RegisterHTTPHandlers(prefix string, mux *http.ServeMux) {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	mux.HandleFunc(prefix+"plans/", c.handlePlanExecutions)
}

// handlePlanExecutions routes plan-scoped execution requests by subpath.
// Path format: {prefix}plans/{slug}/{subpath}
func (c *Component) handlePlanExecutions(w http.ResponseWriter, r *http.Request) {
	// Extract slug and subpath from URL.
	// The mux registered "plans/" so r.URL.Path starts after the prefix.
	// We need to extract: plans/{slug}/{subpath}
	path := r.URL.Path
	plansIdx := strings.Index(path, "plans/")
	if plansIdx < 0 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	remainder := path[plansIdx+len("plans/"):]

	// Split into slug and subpath.
	parts := strings.SplitN(remainder, "/", 2)
	slug := parts[0]
	if slug == "" {
		http.Error(w, "slug required", http.StatusBadRequest)
		return
	}

	subpath := ""
	if len(parts) > 1 {
		subpath = parts[1]
	}

	switch subpath {
	case "stream":
		c.handleExecutionStream(w, r, slug)
	case "tasks":
		c.handleListTasks(w, r, slug)
	case "requirements":
		c.handleListRequirements(w, r, slug)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}
