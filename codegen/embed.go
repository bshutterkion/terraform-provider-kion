// Package codegen embeds the generator inputs that a shipped binary needs at
// runtime.
//
// Only state_upgrades.yaml is embedded, and only because kmigrate is handed to
// customers: it rewrites a configuration written for the old provider, and it
// needs this mapping to know what to rewrite. Defaulting to the repo-relative
// path meant the very first documented command failed for anyone outside a
// clone, `open codegen/state_upgrades.yaml: no such file or directory`.
//
// Embedding also removes a whole class of error: the ruleset can no longer be
// paired with a kmigrate built from a different commit. `--upgrades` remains as
// an override for maintainers iterating on the rules.
//
// The rest of codegen/ stays on disk. Those files are inputs to generators run
// from a checkout, so nothing gains from embedding them.
package codegen

import _ "embed"

// StateUpgradesYAML is codegen/state_upgrades.yaml as of the build.
//
//go:embed state_upgrades.yaml
var StateUpgradesYAML []byte
