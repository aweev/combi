package automation

import (
	"fmt"
)

type SimpleSchemaValidator struct {
	// Map of EventType -> Required Fields
	// This could be loaded from a config file or DB in the future
	schemas map[string][]string
}

func NewSimpleSchemaValidator() *SimpleSchemaValidator {
	return &SimpleSchemaValidator{
		schemas: map[string][]string{
			"user.created":      {"user_id", "email"},
			"donation.received": {"amount", "currency"},
		},
	}
}

func (v *SimpleSchemaValidator) Validate(eventType string, data map[string]any) error {
	requiredFields, exists := v.schemas[eventType]
	if !exists {
		// strict mode: return error?
		// lax mode: return nil
		return nil
	}

	for _, field := range requiredFields {
		if _, ok := data[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}
	return nil
}
