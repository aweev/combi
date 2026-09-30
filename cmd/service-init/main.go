// combi/cmd/service-init/main.go — fx-module scaffolding generator.
//
// Generates a service as an fx library module (no main package) under
// combi/services/<tier>/<name>-service/. Host binaries (cmd/platform-server,
// cmd/workflow-engine) mount the modules via fx — per the consolidated
// topology in plan.md. service-init therefore never emits a main package.
//
// Usage:
//
//	service-init [flags] <service-name>
//
// Example:
//
//	service-init -tier platform payment-links
//	  → services/platform/payment-links-service/
//	      go.mod          module github.com/combi/paymentlinks
//	      fx.go           fx.Module wiring (NewService provided, RegisterHooks invoked)
//	      service.go      Service interface + skeleton impl
//	      service_test.go liveness test
//	      README.md       how to mount it in a host binary
package main

import (
	"flag"
	"fmt"
	"os"
)

func usage() {
	fmt.Fprintln(os.Stderr, `usage: service-init [flags] <service-name>

flags:
  -tier string  platform (default) | ngo | cast
  -root string  repo root (combi/) override
  -no-tidy      skip go work use + tidy + build verification
  -no-wire      skip only 'go work use' (workspace may be temporarily broken)

example:
  service-init -tier platform payment-links`)
}

func main() {
	tier := flag.String("tier", "platform", "service tier: platform|ngo|cast")
	root := flag.String("root", "", "repo root (combi/) override")
	noTidy := flag.Bool("no-tidy", false, "skip go work use + tidy + build verification")
	noWire := flag.Bool("no-wire", false, "skip only go work use")

	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		usage()
		os.Exit(2)
	}

	if err := run(args[0], *tier, *root, *noTidy, *noWire); err != nil {
		fmt.Fprintf(os.Stderr, "service-init: %v\n", err)
		os.Exit(1)
	}
}
