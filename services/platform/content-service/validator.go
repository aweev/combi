package content

import (
	"context"
	"fmt"
	"strings"

	"github.com/combi/pkg/domain/content"
)

type SimpleValidator struct{}

func NewValidator() content.Validator {
	return &SimpleValidator{}
}

func (v *SimpleValidator) Validate(ctx context.Context, fields map[string]map[string]any, model *content.Model) error {
	// 1. Check Required Fields
	for _, field := range model.Fields {
		if field.IsRequired {
			// Check if at least one locale has value, or default locale?
			// For localized fields, usually need value in default locale.
			// Simplified check: iterate all locales provided, ensure field exists.
			// Or check specifically against default locale of space? (Service would need to pass Space).

			// Basic: Just check if field key exists in the input maps
			found := false
			for _, localeFields := range fields {
				if val, ok := localeFields[field.ID]; ok && val != nil {
					// String empty check
					if str, isStr := val.(string); isStr && strings.TrimSpace(str) == "" {
						continue
					}
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("%w: field '%s' is required", content.ErrMissingRequiredField, field.Name)
			}
		}
	}
	return nil
}
