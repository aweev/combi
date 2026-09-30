// combi/cmd/i18n-tool/util.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

func loadJSONFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	catalog := make(map[string]any)
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return catalog, nil
}

func saveJSONFile(path string, catalog map[string]any) error {
	keys := make([]string, 0, len(catalog))
	for key := range catalog {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	ordered := make(map[string]any, len(catalog))
	for _, key := range keys {
		ordered[key] = catalog[key]
	}

	data, err := json.MarshalIndent(ordered, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
