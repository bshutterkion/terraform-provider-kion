// Command kalign aligns Terraform resource schemas against the kion-sdk-go
// generated client types. See package terraform-provider-kion/internal/kalign.
//
// Usage:
//
//	kalign check [-sdk DIR] [-version v3_16] [-service NAME]   # report drift (#1)
//	kalign gen   [-sdk DIR] [-version v3_16] [-service NAME]   # emit flatten converters (#2)
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"terraform-provider-kion/internal/kalign"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// diag is a best-effort diagnostic writer: it remembers the first write error
// so call sites don't each have to check it (mirrors internal/kalign's
// errWriter). Diagnostics are non-critical, so the remembered error is only
// used to short-circuit further writes, never surfaced.
type diag struct {
	w   io.Writer
	err error
}

func (d *diag) printf(format string, a ...any) {
	if d.err != nil {
		return
	}
	_, d.err = fmt.Fprintf(d.w, format, a...)
}

func (d *diag) println(a ...any) {
	if d.err != nil {
		return
	}
	_, d.err = fmt.Fprintln(d.w, a...)
}

// run is the testable entry point: it takes the CLI arguments (excluding the
// program name) and its output streams, and returns the process exit code
// instead of calling os.Exit directly.
func run(args []string, stdout, stderr io.Writer) int {
	d := &diag{w: stderr}
	if len(args) < 1 || (args[0] != "check" && args[0] != "gen") {
		d.println("usage: kalign <check|gen> [-sdk DIR] [-version v3_16] [-service NAME]")
		return 2
	}
	mode := args[0]
	fs := flag.NewFlagSet(mode, flag.ContinueOnError)
	fs.SetOutput(stderr)
	sdkDir := fs.String("sdk", "../kion-sdk-go", "path to the kion-sdk-go checkout")
	version := fs.String("version", "v3_16", "SDK sub-package version to align against")
	only := fs.String("service", "", "limit to a single service (default: all)")
	if err := fs.Parse(args[1:]); err != nil {
		d.printf("error: %v\n", err)
		return 2
	}

	opts := kalign.Options{
		SDKDir:      *sdkDir,
		Version:     *version,
		ServiceRoot: "internal/service",
		FlexDir:     "internal/flex",
		OnlyService: *only,
	}
	src := kalign.NewFileSource()

	var (
		problems int
		err      error
	)
	switch mode {
	case "check":
		problems, err = kalign.Check(src, stdout, opts)
	case "gen":
		_, err = kalign.Gen(src, stdout, opts)
	}
	if err != nil {
		d.printf("error: %v\n", err)
		return 1
	}
	if mode == "check" && problems > 0 {
		return 1 // non-zero so CI fails on drift
	}
	return 0
}
