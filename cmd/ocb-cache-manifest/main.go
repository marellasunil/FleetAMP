// Command ocb-cache-manifest produces the minimal go.mod used to preload the
// reviewed FleetAMP OCB component catalog into a version-specific builder image.
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/marellasunil/FleetAMP/internal/distributions"
)

var collectorVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func main() {
	version := flag.String("version", "", "exact Collector/OCB semantic version without a v prefix")
	output := flag.String("output", "", "output go.mod path")
	flag.Parse()
	if !collectorVersionPattern.MatchString(strings.TrimSpace(*version)) {
		fatal("version must be an exact semantic version such as 0.140.0")
	}
	if strings.TrimSpace(*output) == "" {
		fatal("output path is required")
	}
	content := renderModuleFile(*version, distributions.ReviewedOCBModules())
	if err := os.WriteFile(*output, []byte(content), 0o600); err != nil {
		fatal(err.Error())
	}
}

func renderModuleFile(version string, modules []string) string {
	b := &strings.Builder{}
	fmt.Fprintln(b, "module fleetamp.local/ocb-offline-cache")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "go 1.25.0")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "require (")
	fmt.Fprintf(b, "\tgo.opentelemetry.io/collector/cmd/builder v%s\n", version)
	for _, module := range modules {
		fmt.Fprintf(b, "\t%s v%s\n", module, version)
	}
	fmt.Fprintln(b, ")")
	return b.String()
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
