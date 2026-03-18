package trajectoryapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	agvocab "github.com/c360studio/semstreams/vocabulary/agentic"
)

const (
	// maxGraphErrorBodySize limits the size of error response bodies.
	maxGraphErrorBodySize = 4096
)

// StepQuerier queries trajectory step entities from the knowledge graph.
// Semstreams alpha.53 writes step entities with agent.step.* predicates on loop completion.
type StepQuerier struct {
	gatewayURL string
	httpClient *http.Client
}

// NewStepQuerier creates a new step querier for the given graph gateway URL.
func NewStepQuerier(gatewayURL string) *StepQuerier {
	return &StepQuerier{
		gatewayURL: gatewayURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// graphQLResponse represents a GraphQL response envelope.
type graphQLResponse struct {
	Data   map[string]any `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// graphEntity represents a graph entity with its triples.
type graphEntity struct {
	ID      string        `json:"id"`
	Triples []graphTriple `json:"triples,omitempty"`
}

// graphTriple is a predicate-object pair on a graph entity.
type graphTriple struct {
	Predicate string `json:"predicate"`
	Object    any    `json:"object"`
}

// StepRecord is a parsed trajectory step entity from the graph.
// All fields map directly to agent.step.* predicates written by semstreams alpha.53.
type StepRecord struct {
	// EntityID is the graph entity ID for this step.
	EntityID string

	// Type is the step category: "model_call" or "tool_call".
	Type string

	// Index is the zero-based position in the loop trajectory.
	Index int

	// LoopEntityID is the entity ID of the parent loop.
	LoopEntityID string

	// Timestamp is when this step occurred.
	Timestamp time.Time

	// DurationMs is the step execution time in milliseconds.
	DurationMs int64

	// --- tool_call fields ---

	// ToolName is the tool function name (tool_call steps only).
	ToolName string

	// --- model_call fields ---

	// Model is the model name (model_call steps only).
	Model string

	// TokensIn is the input token count (model_call steps only).
	TokensIn int

	// TokensOut is the output token count (model_call steps only).
	TokensOut int

	// --- common optional fields ---

	// Capability is the role or purpose of this step.
	Capability string

	// Provider is the LLM provider (model_call steps only).
	Provider string

	// Retries is the number of retries before this step succeeded.
	Retries int
}

// QueryStepsByLoopEntityID returns all trajectory step entities for a loop entity ID.
// It uses entitiesByPredicate with agent.step.loop predicate to find all steps.
func (q *StepQuerier) QueryStepsByLoopEntityID(ctx context.Context, loopEntityID string) ([]*StepRecord, error) {
	loopEntityID = sanitizeGraphQLString(loopEntityID)

	const gql = `query($predicate: String!, $value: String) {
		entitiesByPredicate(predicate: $predicate, value: $value) {
			id
			triples { predicate object }
		}
	}`

	variables := map[string]any{
		"predicate": agvocab.StepLoop,
		"value":     loopEntityID,
	}

	data, err := q.executeGraphQL(ctx, gql, variables)
	if err != nil {
		return nil, fmt.Errorf("query steps by loop entity: %w", err)
	}

	entities := parseEntitiesFromData(data, "entitiesByPredicate")
	records := make([]*StepRecord, 0, len(entities))
	for _, entity := range entities {
		records = append(records, entityToStepRecord(entity))
	}

	sortStepsByIndex(records)
	return records, nil
}

// QueryStepsByLoopRelationships fetches steps for a loop via its LoopHasStep
// relationships. This is a two-step query: first fetch the loop entity to get
// step entity IDs, then fetch each step entity individually.
//
// This is a fallback path when entitiesByPredicate with value filtering is not
// supported by the graph gateway.
func (q *StepQuerier) QueryStepsByLoopRelationships(ctx context.Context, loopEntityID string) ([]*StepRecord, error) {
	loopEntityID = sanitizeGraphQLString(loopEntityID)

	// Step 1: fetch the loop entity's relationships to find step entity IDs.
	const relQuery = `query($entityId: String!) {
		relationships(entityId: $entityId) {
			predicate
			object
		}
	}`

	relData, err := q.executeGraphQL(ctx, relQuery, map[string]any{"entityId": loopEntityID})
	if err != nil {
		return nil, fmt.Errorf("query loop relationships: %w", err)
	}

	stepEntityIDs := extractRelationshipObjects(relData, agvocab.LoopHasStep)
	if len(stepEntityIDs) == 0 {
		return nil, nil
	}

	// Step 2: fetch each step entity.
	records := make([]*StepRecord, 0, len(stepEntityIDs))
	for _, stepID := range stepEntityIDs {
		entity, err := q.fetchEntityByID(ctx, stepID)
		if err != nil {
			// Log but continue — partial results are better than none.
			continue
		}
		if entity != nil {
			records = append(records, entityToStepRecord(*entity))
		}
	}

	sortStepsByIndex(records)
	return records, nil
}

// fetchEntityByID retrieves a single entity by its exact ID.
func (q *StepQuerier) fetchEntityByID(ctx context.Context, entityID string) (*graphEntity, error) {
	const gql = `query($id: String!) {
		entity(id: $id) {
			id
			triples { predicate object }
		}
	}`

	data, err := q.executeGraphQL(ctx, gql, map[string]any{"id": entityID})
	if err != nil {
		return nil, err
	}

	entityRaw, ok := data["entity"]
	if !ok || entityRaw == nil {
		return nil, nil
	}

	entityMap, ok := entityRaw.(map[string]any)
	if !ok {
		return nil, nil
	}

	entity := parseGraphEntity(entityMap)
	return &entity, nil
}

// executeGraphQL runs a GraphQL query against the gateway and returns the data map.
func (q *StepQuerier) executeGraphQL(ctx context.Context, query string, variables map[string]any) (map[string]any, error) {
	reqBody := map[string]any{"query": query}
	if variables != nil {
		reqBody["variables"] = variables
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", q.gatewayURL+"/graphql", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxGraphErrorBodySize))
		return nil, fmt.Errorf("graph gateway returned %d: %s", resp.StatusCode, string(body))
	}

	var result graphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", result.Errors[0].Message)
	}

	return result.Data, nil
}

// entityToStepRecord converts a graph entity with agent.step.* triples to a StepRecord.
func entityToStepRecord(entity graphEntity) *StepRecord {
	record := &StepRecord{EntityID: entity.ID}

	// Build a fast lookup map; multi-valued predicates are not expected for steps.
	predicates := make(map[string]any, len(entity.Triples))
	for _, t := range entity.Triples {
		predicates[t.Predicate] = t.Object
	}

	record.Type = getString(predicates, agvocab.StepType)
	record.Index = getInt(predicates, agvocab.StepIndex)
	record.LoopEntityID = getString(predicates, agvocab.StepLoop)
	record.DurationMs = getInt64(predicates, agvocab.StepDuration)
	record.ToolName = getString(predicates, agvocab.StepToolName)
	record.Model = getString(predicates, agvocab.StepModel)
	record.TokensIn = getInt(predicates, agvocab.StepTokensIn)
	record.TokensOut = getInt(predicates, agvocab.StepTokensOut)
	record.Capability = getString(predicates, agvocab.StepCapability)
	record.Provider = getString(predicates, agvocab.StepProvider)
	record.Retries = getInt(predicates, agvocab.StepRetries)

	if ts := getString(predicates, agvocab.StepTimestamp); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			record.Timestamp = t
		}
	}

	return record
}

// parseEntitiesFromData extracts a slice of entities from a GraphQL response data map.
func parseEntitiesFromData(data map[string]any, key string) []graphEntity {
	entitiesRaw, ok := data[key].([]any)
	if !ok {
		return nil
	}

	entities := make([]graphEntity, 0, len(entitiesRaw))
	for _, e := range entitiesRaw {
		entityMap, ok := e.(map[string]any)
		if !ok {
			continue
		}
		entities = append(entities, parseGraphEntity(entityMap))
	}

	return entities
}

// parseGraphEntity parses a single entity from a raw map.
func parseGraphEntity(entityMap map[string]any) graphEntity {
	entity := graphEntity{}

	if id, ok := entityMap["id"].(string); ok {
		entity.ID = id
	}

	if triples, ok := entityMap["triples"].([]any); ok {
		for _, t := range triples {
			tripleMap, ok := t.(map[string]any)
			if !ok {
				continue
			}
			triple := graphTriple{}
			if pred, ok := tripleMap["predicate"].(string); ok {
				triple.Predicate = pred
			}
			triple.Object = tripleMap["object"]
			entity.Triples = append(entity.Triples, triple)
		}
	}

	return entity
}

// extractRelationshipObjects returns the object values for all triples with the
// given predicate in the relationships query response.
func extractRelationshipObjects(data map[string]any, predicate string) []string {
	relsRaw, ok := data["relationships"].([]any)
	if !ok {
		return nil
	}

	var objects []string
	for _, r := range relsRaw {
		relMap, ok := r.(map[string]any)
		if !ok {
			continue
		}
		pred, _ := relMap["predicate"].(string)
		if pred != predicate {
			continue
		}
		if obj, ok := relMap["object"].(string); ok && obj != "" {
			objects = append(objects, obj)
		}
	}
	return objects
}

// getTripleValue returns the string value of a specific predicate in an entity.
func getTripleValue(entity graphEntity, predicate string) string {
	for _, t := range entity.Triples {
		if t.Predicate == predicate {
			if val, ok := t.Object.(string); ok {
				return val
			}
		}
	}
	return ""
}

// getString extracts a string value from the predicates map.
func getString(predicates map[string]any, key string) string {
	if val, ok := predicates[key]; ok {
		switch v := val.(type) {
		case string:
			return v
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		}
	}
	return ""
}

// getInt extracts an int value from the predicates map.
func getInt(predicates map[string]any, key string) int {
	if val, ok := predicates[key]; ok {
		switch v := val.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	}
	return 0
}

// getInt64 extracts an int64 value from the predicates map.
func getInt64(predicates map[string]any, key string) int64 {
	if val, ok := predicates[key]; ok {
		switch v := val.(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		case int:
			return int64(v)
		case string:
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				return i
			}
		}
	}
	return 0
}

// sortStepsByIndex sorts step records by their StepIndex in ascending order.
func sortStepsByIndex(steps []*StepRecord) {
	sort.Slice(steps, func(i, j int) bool {
		if steps[i].Index != steps[j].Index {
			return steps[i].Index < steps[j].Index
		}
		// Tiebreak by timestamp for deterministic ordering.
		return steps[i].Timestamp.Before(steps[j].Timestamp)
	})
}

// sanitizeGraphQLString removes potentially dangerous characters from GraphQL string inputs.
// This provides defense-in-depth alongside parameterized queries.
func sanitizeGraphQLString(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return s
}
