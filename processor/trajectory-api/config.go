package trajectoryapi

import (
	"fmt"
	"reflect"

	"github.com/c360studio/semstreams/component"
)

// trajectoryAPISchema defines the configuration schema.
var trajectoryAPISchema = component.GenerateConfigSchema(reflect.TypeOf(Config{}))

// Config holds configuration for the trajectory-api component.
type Config struct {
	// LoopsBucket is the KV bucket name for agent loop state.
	LoopsBucket string `json:"loops_bucket" schema:"type:string,description:KV bucket for agent loop state,category:basic,default:AGENT_LOOPS"`

	// ContentBucket is the ObjectStore bucket name for step content (tool arguments, results, model responses).
	// When set, detailed content is fetched for format=json requests.
	ContentBucket string `json:"content_bucket,omitempty" schema:"type:string,description:ObjectStore bucket for step content,category:basic,default:AGENT_CONTENT"`

	// RepoRoot is the repository root path for accessing plan data.
	// If empty, defaults to SEMSPEC_REPO_PATH env var or current working directory.
	RepoRoot string `json:"repo_root,omitempty" schema:"type:string,description:Repository root path for plan access,category:basic"`

	// GraphGatewayURL is the URL for the graph gateway service.
	// Used to query step entities from the knowledge graph.
	GraphGatewayURL string `json:"graph_gateway_url,omitempty" schema:"type:string,description:Graph gateway URL for step entity queries,category:basic,default:http://localhost:8082"`

	// Org is the organization identifier used to construct loop and step entity IDs.
	// Must match the org value configured in the semstreams platform identity.
	// Example: "semspec"
	Org string `json:"org,omitempty" schema:"type:string,description:Organization identifier for entity ID construction,category:basic"`

	// Platform is the platform identifier used to construct loop and step entity IDs.
	// Must match the platform ID configured in the semstreams platform identity.
	// Example: "semspec-dev"
	Platform string `json:"platform,omitempty" schema:"type:string,description:Platform identifier for entity ID construction,category:basic"`

	// Ports contains input/output port definitions.
	Ports *component.PortConfig `json:"ports,omitempty" schema:"type:ports,description:Input/output port definitions,category:basic"`
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() Config {
	return Config{
		LoopsBucket:     "AGENT_LOOPS",
		ContentBucket:   "AGENT_CONTENT",
		GraphGatewayURL: "http://localhost:8082",
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.LoopsBucket == "" {
		return fmt.Errorf("loops_bucket is required")
	}
	return nil
}
