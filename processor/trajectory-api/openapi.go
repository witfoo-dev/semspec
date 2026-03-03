package trajectoryapi

import (
	"reflect"

	"github.com/c360studio/semstreams/service"

	"github.com/c360studio/semspec/llm"
)

func init() {
	service.RegisterOpenAPISpec("trajectory-api", trajectoryAPIOpenAPISpec())
}

// OpenAPISpec implements the OpenAPIProvider interface.
func (c *Component) OpenAPISpec() *service.OpenAPISpec {
	return trajectoryAPIOpenAPISpec()
}

// trajectoryAPIOpenAPISpec returns the OpenAPI specification for trajectory-api endpoints.
func trajectoryAPIOpenAPISpec() *service.OpenAPISpec {
	formatParam := service.ParameterSpec{
		Name:        "format",
		In:          "query",
		Required:    false,
		Description: "Response format: \"summary\" (default) or \"json\" (includes full entry details)",
		Schema:      service.Schema{Type: "string"},
	}

	return &service.OpenAPISpec{
		Tags: []service.TagSpec{
			{Name: "Trajectory", Description: "LLM call trajectory and observability queries"},
		},
		Paths: map[string]service.PathSpec{
			"/trajectory-api/loops/{loop_id}": {
				GET: &service.OperationSpec{
					Summary:     "Get loop trajectory",
					Description: "Returns the LLM call trajectory for a specific agent loop, including token usage and timing per step",
					Tags:        []string{"Trajectory"},
					Parameters: []service.ParameterSpec{
						{
							Name:        "loop_id",
							In:          "path",
							Required:    true,
							Description: "Agent loop identifier",
							Schema:      service.Schema{Type: "string"},
						},
						formatParam,
					},
					Responses: map[string]service.ResponseSpec{
						"200": {
							Description: "Trajectory for the given loop",
							ContentType: "application/json",
							SchemaRef:   "#/components/schemas/Trajectory",
						},
						"404": {Description: "Loop not found"},
						"503": {Description: "Trajectory service unavailable"},
					},
				},
			},
			"/trajectory-api/traces/{trace_id}": {
				GET: &service.OperationSpec{
					Summary:     "Get trace trajectory",
					Description: "Returns aggregated LLM call trajectory for all loops within a trace",
					Tags:        []string{"Trajectory"},
					Parameters: []service.ParameterSpec{
						{
							Name:        "trace_id",
							In:          "path",
							Required:    true,
							Description: "Trace correlation identifier",
							Schema:      service.Schema{Type: "string"},
						},
						formatParam,
					},
					Responses: map[string]service.ResponseSpec{
						"200": {
							Description: "Aggregated trajectory for the given trace",
							ContentType: "application/json",
							SchemaRef:   "#/components/schemas/Trajectory",
						},
						"404": {Description: "Trace not found"},
						"503": {Description: "Trajectory service unavailable"},
					},
				},
			},
			"/trajectory-api/workflows/{slug}": {
				GET: &service.OperationSpec{
					Summary:     "Get workflow trajectory",
					Description: "Returns LLM call trajectory aggregated across all phases and agent loops for an entire workflow",
					Tags:        []string{"Trajectory"},
					Parameters: []service.ParameterSpec{
						{
							Name:        "slug",
							In:          "path",
							Required:    true,
							Description: "URL-friendly plan identifier",
							Schema:      service.Schema{Type: "string"},
						},
					},
					Responses: map[string]service.ResponseSpec{
						"200": {
							Description: "Workflow-level trajectory with phase breakdown and aggregate metrics",
							ContentType: "application/json",
							SchemaRef:   "#/components/schemas/WorkflowTrajectory",
						},
						"404": {Description: "Workflow not found"},
						"503": {Description: "Trajectory service unavailable"},
					},
				},
			},
			"/trajectory-api/calls/{request_id}": {
				GET: &service.OperationSpec{
					Summary:     "Get call record",
					Description: "Returns the full LLM call record for a specific request, including messages, token usage, and timing",
					Tags:        []string{"Trajectory"},
					Parameters: []service.ParameterSpec{
						{
							Name:        "request_id",
							In:          "path",
							Required:    true,
							Description: "LLM request identifier returned in the call response",
							Schema:      service.Schema{Type: "string"},
						},
					},
					Responses: map[string]service.ResponseSpec{
						"200": {
							Description: "Full LLM call record",
							ContentType: "application/json",
							SchemaRef:   "#/components/schemas/CallRecord",
						},
						"404": {Description: "Call record not found"},
						"503": {Description: "Trajectory service unavailable"},
					},
				},
			},
			"/trajectory-api/context-stats": {
				GET: &service.OperationSpec{
					Summary:     "Get context utilization stats",
					Description: "Returns context window utilization metrics for a trace or workflow. At least one of trace_id or workflow must be provided.",
					Tags:        []string{"Trajectory"},
					Parameters: []service.ParameterSpec{
						{
							Name:        "trace_id",
							In:          "query",
							Required:    false,
							Description: "Filter stats to a specific trace",
							Schema:      service.Schema{Type: "string"},
						},
						{
							Name:        "workflow",
							In:          "query",
							Required:    false,
							Description: "Filter stats to a specific workflow slug",
							Schema:      service.Schema{Type: "string"},
						},
						{
							Name:        "capability",
							In:          "query",
							Required:    false,
							Description: "Filter stats to a specific capability (e.g. planning, coding)",
							Schema:      service.Schema{Type: "string"},
						},
						formatParam,
					},
					Responses: map[string]service.ResponseSpec{
						"200": {
							Description: "Context utilization statistics with per-capability breakdown",
							ContentType: "application/json",
							SchemaRef:   "#/components/schemas/ContextStats",
						},
						"400": {Description: "Neither trace_id nor workflow query parameter was provided"},
						"503": {Description: "Trajectory service unavailable"},
					},
				},
			},
		},
		ResponseTypes: []reflect.Type{
			reflect.TypeOf(Trajectory{}),
			reflect.TypeOf(TrajectoryEntry{}),
			reflect.TypeOf(WorkflowTrajectory{}),
			reflect.TypeOf(PhaseMetrics{}),
			reflect.TypeOf(CapabilityMetrics{}),
			reflect.TypeOf(AggregateMetrics{}),
			reflect.TypeOf(TruncationSummary{}),
			reflect.TypeOf(ContextStats{}),
			reflect.TypeOf(ContextSummary{}),
			reflect.TypeOf(CapabilityContextStats{}),
			reflect.TypeOf(CallContextDetail{}),
			reflect.TypeOf(llm.CallRecord{}),
			reflect.TypeOf(llm.Message{}),
		},
	}
}
