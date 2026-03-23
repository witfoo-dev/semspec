package questionrouter

import (
	"fmt"

	"github.com/c360studio/semstreams/component"
)

// Register registers the question-router component with the given registry.
func Register(registry interface {
	RegisterWithConfig(component.RegistrationConfig) error
}) error {
	if registry == nil {
		return fmt.Errorf("registry cannot be nil")
	}
	return registry.RegisterWithConfig(component.RegistrationConfig{
		Name:        componentName,
		Factory:     NewComponent,
		Type:        "processor",
		Protocol:    "question",
		Domain:      "agentic",
		Description: "Routes agent questions to answerers based on topic patterns",
		Version:     "0.1.0",
	})
}
