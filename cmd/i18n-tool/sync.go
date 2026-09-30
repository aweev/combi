// combi/cmd/i18n-tool/sync.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func syncCmd(args []string) int {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	src := fs.String("src", ".", "source directory to scan for translation keys")
	localeDir := fs.String("locale-dir", "", "directory containing locale JSON files")
	fs.Parse(args)

	if *localeDir == "" {
		return failf("-locale-dir is required")
	}

	keys, err := extractKeys(*src)
	if err != nil {
		return exitOnError(err)
	}

	count, err := syncLocales(keys, *localeDir)
	if err != nil {
		return exitOnError(err)
	}

	fmt.Fprintf(os.Stdout, "synced %d locale files in %s\n", count, *localeDir)
	return 0
}

func syncLocales(keys []string, localeDir string) (int, error) {
	entries, err := os.ReadDir(localeDir)
	if err != nil {
		return 0, fmt.Errorf("read locale directory: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(localeDir, entry.Name())
		catalog, err := loadJSONFile(path)
		if err != nil {
			return 0, err
		}

		for _, key := range keys {
			if _, ok := catalog[key]; !ok {
				catalog[key] = ""
			}
		}

		if err := saveJSONFile(path, catalog); err != nil {
			return 0, err
		}
		count++
	}

	return count, nil
}
