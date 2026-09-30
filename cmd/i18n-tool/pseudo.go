// combi/cmd/i18n-tool/pseudo.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func pseudoCmd(args []string) int {
	fs := flag.NewFlagSet("pseudo", flag.ExitOnError)
	srcFile := fs.String("src-file", "", "source JSON file to pseudo-localize")
	outFile := fs.String("out-file", "", "destination JSON file")
	localeDir := fs.String("locale-dir", "", "directory containing locale files")
	srcLocale := fs.String("src-locale", "", "source locale tag (e.g. en-NG)")
	outLocale := fs.String("out-locale", "", "destination locale tag (e.g. en-XA)")
	fs.Parse(args)

	if *srcFile == "" && *localeDir == "" {
		return failf("either -src-file or -locale-dir must be provided")
	}
	if *outFile == "" && *outLocale == "" {
		return failf("either -out-file or -out-locale must be provided")
	}

	if *localeDir != "" {
		if *srcLocale == "" || *outLocale == "" {
			return failf("-src-locale and -out-locale are required when using -locale-dir")
		}
		*srcFile = filepath.Join(*localeDir, *srcLocale+".json")
		*outFile = filepath.Join(*localeDir, *outLocale+".json")
	}

	if *srcFile == "" || *outFile == "" {
		return failf("both -src-file and -out-file are required")
	}

	catalog, err := loadJSONFile(*srcFile)
	if err != nil {
		return exitOnError(err)
	}

	pseudo := make(map[string]any, len(catalog))
	for key, value := range catalog {
		pseudo[key] = pseudoTranslateValue(value)
	}

	if err := saveJSONFile(*outFile, pseudo); err != nil {
		return exitOnError(err)
	}

	fmt.Fprintf(os.Stdout, "generated pseudo-locale %s from %s\n", *outFile, *srcFile)
	return 0
}

func pseudoTranslateValue(value any) any {
	switch v := value.(type) {
	case string:
		return pseudoTranslate(v)
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, sub := range v {
			out[key] = pseudoTranslateValue(sub)
		}
		return out
	default:
		return v
	}
}

func pseudoTranslate(text string) string {
	replacer := strings.NewReplacer(
		"a", "à",
		"A", "À",
		"e", "é",
		"E", "É",
		"i", "î",
		"I", "Î",
		"o", "ô",
		"O", "Ô",
		"u", "û",
		"U", "Û",
	)
	content := replacer.Replace(text)
	expansion := strings.Repeat("—", len([]rune(text))/3)
	return "[!!" + content + expansion + "!!]"
}
