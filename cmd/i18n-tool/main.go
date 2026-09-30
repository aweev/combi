// combi/cmd/i18n-tool/main.go
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "extract":
		os.Exit(extractCmd(os.Args[2:]))
	case "sync":
		os.Exit(syncCmd(os.Args[2:]))
	case "validate":
		os.Exit(validateCmd(os.Args[2:]))
	case "pseudo":
		os.Exit(pseudoCmd(os.Args[2:]))
	case "help", "-h", "--help":
		usage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `i18n-tool is a minimal localization CLI for the comms repo.

Usage:
  i18n-tool extract -src <path>
  i18n-tool sync -src <path> -locale-dir <dir>
  i18n-tool validate -locale-dir <dir>
  i18n-tool pseudo [-src-file <file> -out-file <file> | -locale-dir <dir> -src-locale <tag> -out-locale <tag>]

Commands:
  extract   Scan Go source and print all translation keys used by T() and TPlural().
  sync      Synchronize locale JSON files with extracted translation keys.
  validate  Fail if Tier 1 locales are missing translations from en-NG.json.
  pseudo    Generate a pseudo-localized JSON catalog for UI testing.
`)
}

func failf(format string, args ...interface{}) int {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	return 1
}

func exitOnError(err error) int {
	if err == nil {
		return 0
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}
