// combi/cmd/i18n-tool/validate.go
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var tier1Locales = []string{"en-NG", "pcm-NG", "fr-CM", "sw-KE"}

func validateCmd(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	localeDir := fs.String("locale-dir", "", "directory containing locale JSON files")
	fs.Parse(args)

	if *localeDir == "" {
		return failf("-locale-dir is required")
	}

	if err := validateLocales(*localeDir); err != nil {
		return exitOnError(err)
	}

	fmt.Fprintln(os.Stdout, "validation passed")
	return 0
}

func validateLocales(localeDir string) error {
	sourcePath := filepath.Join(localeDir, "en-NG.json")
	source, err := loadJSONFile(sourcePath)
	if err != nil {
		return fmt.Errorf("load en-NG source catalog: %w", err)
	}

	var validationErrors []string
	for _, localeTag := range tier1Locales {
		if localeTag == "en-NG" {
			continue
		}
		path := filepath.Join(localeDir, localeTag+".json")
		catalog, err := loadJSONFile(path)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("%s: %v", localeTag, err))
			continue
		}

		missing, empty := compareKeys(source, catalog)
		if len(missing) > 0 {
			validationErrors = append(validationErrors, fmt.Sprintf("%s: missing keys: %s", localeTag, strings.Join(missing, ", ")))
		}
		if len(empty) > 0 {
			validationErrors = append(validationErrors, fmt.Sprintf("%s: empty translations: %s", localeTag, strings.Join(empty, ", ")))
		}
	}

	if len(validationErrors) > 0 {
		return errors.New(strings.Join(validationErrors, "\n"))
	}
	return nil
}

func compareKeys(source, target map[string]any) (missing []string, empty []string) {
	for key, val := range source {
		if targetVal, ok := target[key]; !ok || isMissingOrEmpty(targetVal) {
			if !ok {
				missing = append(missing, key)
			} else {
				empty = append(empty, key)
			}
			continue
		}
		if m, ok := val.(map[string]any); ok {
			if targetMap, ok := target[key].(map[string]any); ok {
				for sub, _ := range m {
					if t, ok := targetMap[sub]; !ok || isMissingOrEmpty(t) {
						empty = append(empty, fmt.Sprintf("%s.%s", key, sub))
					}
				}
			}
		}
	}
	return
}

func isMissingOrEmpty(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case map[string]any:
		for _, sub := range v {
			if isMissingOrEmpty(sub) {
				return true
			}
		}
		return false
	default:
		return false
	}
}
