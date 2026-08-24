package kimport

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"terraform-provider-kion/internal/kgen/importmanifest"
)

// LoadManifest parses the embedded (or supplied) import manifest.
func LoadManifest(data []byte) (*importmanifest.Manifest, error) {
	var m importmanifest.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse import manifest: %w", err)
	}
	if m.Version != importmanifest.ManifestVersion {
		return nil, fmt.Errorf("import manifest version %d, want %d",
			m.Version, importmanifest.ManifestVersion)
	}
	return &m, nil
}

// FormatReport summarizes a run: totals, then only the rows a human must act on.
// Listing all 50 healthy kinds would bury the handful that failed.
func FormatReport(results []Result) string {
	counts := map[string]int{}
	total := 0
	var gaps []Result
	for _, r := range results {
		counts[r.Status]++
		total += len(r.Records)
		if r.Status == "unsupported" || r.Status == "error" {
			gaps = append(gaps, r)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "\nCoverage: %d resource types, %d records\n", len(results), total)

	statuses := make([]string, 0, len(counts))
	for s := range counts {
		statuses = append(statuses, s)
	}
	sort.Strings(statuses)
	parts := make([]string, 0, len(statuses))
	for _, s := range statuses {
		parts = append(parts, fmt.Sprintf("%s: %d", s, counts[s]))
	}
	fmt.Fprintf(&b, "  %s\n", strings.Join(parts, ", "))

	if len(gaps) > 0 {
		sort.Slice(gaps, func(i, j int) bool { return gaps[i].TFType < gaps[j].TFType })
		fmt.Fprintf(&b, "\nNot imported (%d):\n", len(gaps))
		for _, r := range gaps {
			fmt.Fprintf(&b, "  %-45s %-12s %s\n", r.TFType, r.Status, r.Reason)
		}
	}

	// A successful read can still carry a real gap in Reason: records skipped
	// for having no usable id, association records skipped for a missing key
	// field, or a flat list that 405'd and fell back to a parent-scoped read.
	// Those results have status "ok" or "empty", so they never land in the
	// "Not imported" section above -- but dropping the fact silently would
	// make a partial read look clean. Report them separately. Mirrors
	// RenderImports' "Read with caveats" section in emit.go.
	var caveats []Result
	for _, r := range results {
		if r.Reason == "" {
			continue
		}
		if r.Status == "unsupported" || r.Status == "error" {
			continue // already reported above
		}
		caveats = append(caveats, r)
	}
	if len(caveats) > 0 {
		sort.Slice(caveats, func(i, j int) bool { return caveats[i].TFType < caveats[j].TFType })
		fmt.Fprintf(&b, "\nRead with caveats (%d):\n", len(caveats))
		for _, r := range caveats {
			fmt.Fprintf(&b, "  %-45s %-12s %s\n", r.TFType, r.Status, r.Reason)
		}
	}

	return b.String()
}
