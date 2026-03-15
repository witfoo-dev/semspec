package workflow

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// commitResponse builds a GraphQL response containing a commit entity
// with the given SHA.
func commitResponse(sha string) []byte {
	return []byte(fmt.Sprintf(`{
		"data": {
			"entitiesByPredicate": [{
				"id": "acme.semsource.git.repo.commit.%s",
				"predicates": [
					{"predicate": "source.git.commit.sha", "object": %q},
					{"predicate": "source.git.commit.subject", "object": "feat: something"}
				]
			}]
		}
	}`, sha[:7], sha))
}

// emptyResponse builds a GraphQL response with no matching entities.
func emptyResponse() []byte {
	return []byte(`{"data": {"entitiesByPredicate": []}}`)
}

func newTestGate(srv *httptest.Server) *IndexingGate {
	g := NewIndexingGate(srv.URL, nil)
	g.httpClient = &http.Client{Timeout: 2 * time.Second}
	return g
}

// ---------------------------------------------------------------------------
// NewIndexingGate
// ---------------------------------------------------------------------------

func TestNewIndexingGate_EmptyURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if g := NewIndexingGate(tt.url, nil); g != nil {
				t.Errorf("expected nil for URL %q, got non-nil gate", tt.url)
			}
		})
	}
}

func TestNewIndexingGate_ValidURL(t *testing.T) {
	g := NewIndexingGate("http://localhost:8082", nil)
	if g == nil {
		t.Fatal("expected non-nil gate for valid URL")
	}
	if g.graphGatewayURL != "http://localhost:8082" {
		t.Errorf("gatewayURL = %q, want http://localhost:8082", g.graphGatewayURL)
	}
}

// ---------------------------------------------------------------------------
// AwaitCommitIndexed: nil receiver
// ---------------------------------------------------------------------------

func TestAwaitCommitIndexed_NilGate(t *testing.T) {
	var g *IndexingGate
	err := g.AwaitCommitIndexed(context.Background(), "abc123", time.Second)
	if err != nil {
		t.Errorf("nil gate should return nil, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// AwaitCommitIndexed: found immediately
// ---------------------------------------------------------------------------

func TestAwaitCommitIndexed_FoundImmediately(t *testing.T) {
	commitSHA := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(commitResponse(commitSHA))
	}))
	defer srv.Close()

	gate := newTestGate(srv)
	err := gate.AwaitCommitIndexed(context.Background(), commitSHA, 5*time.Second)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// AwaitCommitIndexed: found after retries
// ---------------------------------------------------------------------------

func TestAwaitCommitIndexed_FoundAfterRetries(t *testing.T) {
	commitSHA := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if n < 3 {
			w.Write(emptyResponse())
		} else {
			w.Write(commitResponse(commitSHA))
		}
	}))
	defer srv.Close()

	gate := newTestGate(srv)
	err := gate.AwaitCommitIndexed(context.Background(), commitSHA, 10*time.Second)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if n := callCount.Load(); n < 3 {
		t.Errorf("expected at least 3 calls, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// AwaitCommitIndexed: timeout
// ---------------------------------------------------------------------------

func TestAwaitCommitIndexed_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(emptyResponse())
	}))
	defer srv.Close()

	gate := newTestGate(srv)
	err := gate.AwaitCommitIndexed(context.Background(), "abc123", 2*time.Second)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

// ---------------------------------------------------------------------------
// AwaitCommitIndexed: cancelled context
// ---------------------------------------------------------------------------

func TestAwaitCommitIndexed_CancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(emptyResponse())
	}))
	defer srv.Close()

	gate := newTestGate(srv)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay.
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	err := gate.AwaitCommitIndexed(ctx, "abc123", 30*time.Second)
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
	if err != context.Canceled {
		t.Logf("error was %v (acceptable for timeout-style errors)", err)
	}
}

// ---------------------------------------------------------------------------
// AwaitCommitIndexed: server error (non-200) retries
// ---------------------------------------------------------------------------

func TestAwaitCommitIndexed_ServerErrorThenSuccess(t *testing.T) {
	commitSHA := "cafebabecafebabecafebabecafebabecafebabe"
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := callCount.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(commitResponse(commitSHA))
	}))
	defer srv.Close()

	gate := newTestGate(srv)
	err := gate.AwaitCommitIndexed(context.Background(), commitSHA, 10*time.Second)
	if err != nil {
		t.Fatalf("expected nil error after recovery, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// containsCommitSHA unit tests
// ---------------------------------------------------------------------------

func TestContainsCommitSHA_Found(t *testing.T) {
	sha := "abc123def456abc123def456abc123def456abc1"
	body := commitResponse(sha)
	if !containsCommitSHA(body, sha) {
		t.Error("expected true for matching SHA")
	}
}

func TestContainsCommitSHA_NotFound(t *testing.T) {
	body := commitResponse("abc123def456abc123def456abc123def456abc1")
	if containsCommitSHA(body, "different_sha_entirely") {
		t.Error("expected false for non-matching SHA")
	}
}

func TestContainsCommitSHA_EmptyResponse(t *testing.T) {
	if containsCommitSHA(emptyResponse(), "anything") {
		t.Error("expected false for empty response")
	}
}

func TestContainsCommitSHA_MalformedJSON(t *testing.T) {
	if containsCommitSHA([]byte(`{bad json`), "anything") {
		t.Error("expected false for malformed JSON")
	}
}
