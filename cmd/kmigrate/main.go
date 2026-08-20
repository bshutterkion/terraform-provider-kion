// Command kmigrate rewrites Terraform .tf configuration from the old (SDKv2) Kion
// provider's attribute names to the new (Plugin Framework) provider's, driven by
// codegen/state_upgrades.yaml (the same mapping the provider's state upgraders
// use). It handles the input-attribute half of migration — attribute renames and
// block-set → id-list restructures — so a customer's config keeps working after
// the provider version bump. State migrates automatically in-provider; see
// docs/MIGRATION.md.
//
// Usage:
//
//	kmigrate [--check] [--upgrades <path>] <dir-or-file> [<dir-or-file>...]
//
//	--check      print the changes without writing (dry run)
//	--upgrades   path to state_upgrades.yaml (default codegen/state_upgrades.yaml)
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"terraform-provider-kion/internal/kgen/migrate"
)

func main() {
	check := flag.Bool("check", false, "print changes without writing (dry run)")
	upPath := flag.String("upgrades", "codegen/state_upgrades.yaml", "path to state_upgrades.yaml")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: kmigrate [--check] [--upgrades <path>] <dir-or-file>...")
		os.Exit(2)
	}

	ups, err := migrate.LoadUpgrades(*upPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kmigrate: %v\n", err)
		os.Exit(1)
	}

	var files []string
	for _, arg := range flag.Args() {
		info, err := os.Stat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "kmigrate: %v\n", err)
			os.Exit(1)
		}
		if info.IsDir() {
			if err := filepath.WalkDir(arg, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && strings.HasSuffix(p, ".tf") {
					files = append(files, p)
				}
				return nil
			}); err != nil {
				fmt.Fprintf(os.Stderr, "kmigrate: %v\n", err)
				os.Exit(1)
			}
		} else {
			files = append(files, arg)
		}
	}

	total := 0
	var manual []string
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "kmigrate: %v\n", err)
			os.Exit(1)
		}
		out, changes, actions, err := migrate.RewriteFile(src, ups)
		if err != nil {
			fmt.Fprintf(os.Stderr, "kmigrate: %s: %v\n", f, err)
			os.Exit(1)
		}
		for _, a := range actions {
			manual = append(manual, fmt.Sprintf("%s: %s", f, a))
		}
		if len(changes) == 0 {
			continue
		}
		total += len(changes)
		for _, c := range changes {
			fmt.Printf("%s: %s\n", f, c)
		}
		if !*check {
			if err := os.WriteFile(f, out, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "kmigrate: %v\n", err)
				os.Exit(1)
			}
		}
	}

	if *check {
		fmt.Printf("\n%d change(s) would be made (dry run; re-run without --check to apply).\n", total)
	} else {
		fmt.Printf("\n%d change(s) applied. Run `terraform fmt` to normalize.\n", total)
	}

	// Printed after the summary, and counted separately: these are not edits
	// kmigrate made, they are edits it cannot make. Left unresolved they surface
	// as "Missing required argument" on the next plan.
	if len(manual) > 0 {
		fmt.Printf("\n%d block(s) need attention before `terraform plan` will pass:\n", len(manual))
		for _, m := range manual {
			fmt.Printf("  %s\n", m)
		}
	}
}
