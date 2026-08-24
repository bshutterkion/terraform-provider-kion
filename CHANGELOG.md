# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Unreleased changes are not listed here — they live as fragments in
`.changes/unreleased/` until a release is cut. Run `make changelog-preview` to
see them, and `make changelog-new` to add one.


## [1.0.2] - 2026-08-24


### BUG FIXES

- Regenerated the documentation and examples for the 38 data sources whose lookup mode changed. Restoring `filter` support made `id` optional on those data sources, but the registry docs and examples still described `id` as required and showed no `filter` block, so they documented behaviour the released provider no longer had.
- `kmigrate` no longer needs `state_upgrades.yaml` alongside it. The `--upgrades` flag defaulted to `codegen/state_upgrades.yaml`, a repository-relative path that does not exist in a practitioner's Terraform directory — so the first command in the migration guide failed for everyone outside a clone of this repo, with `open codegen/state_upgrades.yaml: no such file or directory`. The ruleset is now built into the binary, which also makes it impossible to pair `kmigrate` with a ruleset from a different commit. `--upgrades` remains as an override.

### NOTES

- Acceptance-test configurations are now validated against the provider schema in CI. Acceptance tests only run against a live Kion instance, so nothing had ever checked that the HCL they feed Terraform matches the schema — and it did not: twelve configs used an `owner_users` block on resources whose schema declares an `owner_user_ids` list attribute, two set a read-only `id`, and the great majority were still the `kgen` scaffold's `# TIP: Fill in required attributes` placeholder with an empty resource body. All 87 configs are now filled in and validated by a new `acctest-config` job (locally, `make ci-acctest-config`), which builds the provider and runs `terraform validate` over each one — no Kion instance and no API call required.
- The migration guide is now linked from the provider's registry landing page and from the repository README, and explains how to obtain `kmigrate` — it is attached to every release as a single dependency-free binary. Previously neither entry point mentioned migration at all, so a practitioner upgrading from the SDKv2 provider had no path to the procedure.
- `docs/` and `examples/` are now drift-gated in CI. They were the only generated surfaces without one — the README said so — which is how 1.0.1 shipped registry documentation describing `id` as required on 38 data sources whose schemas had already made it optional: `internal/service/` and `modules/` were regenerated because CI enforced them, and these were not because nothing did. `make docs-check` is the local equivalent.

## [1.0.1] - 2026-08-24


### ENHANCEMENTS

- Releases now attach the `kmigrate` binary and `codegen/state_upgrades.yaml`, so migrating a configuration no longer requires cloning this repository. Neither could be fetched any other way: the module path carries no domain, so `go install` has no remote path to resolve. Both new artifacts are listed in `_SHA256SUMS`, so the existing signature over that file covers them.

### BUG FIXES

- Restored the `kion_funding_source_permission_mapping`, `kion_global_permission_mapping`, and `kion_project_note` data sources, which the old provider shipped but which the rebuild dropped. Their companion-path archetypes (`association`/`blended`) derive no data source, so each needs a registered companion template; only their `ou_permission_mapping`/`project_permission_mapping` siblings had one. The permission-mapping data sources also gained a `filter` block, matching those siblings.
- The `kion_azure_policy` data source regained its `filter` block and no longer requires `id`. Its read payload (`AzurePolicyAugmented`) carries no top-level `id` — the record, id included, is nested under `azure_policy` — so the generator read the payload as id-less and degraded the data source to id-only. Record-wrapper detection now also recognises a record nested under a model object attribute, so `id`, `description`, `name`, `parameters`, and `policy` are sourced from the wrapped record in both by-id and by-filter mode. `kion_azure_policy` also gained a real acceptance-test sweeper for the same reason.
- Upgrading a `kion_aws_cloudformation_template` resource from the SDKv2 provider no longer fails when it has `tags` set. The old schema took `tags` as `map(string)`; the new `kion_cft` schema declares it as a list of `{ tag_key, tag_value }` objects, but the generated v0 state upgrader passed the old map through verbatim, so `terraform plan` aborted with `AttributeName("tags"): invalid JSON, expected "[", got "{"` and the resource could not be migrated at all. The state upgrader now explodes the map into the object list, in sorted key order (which is the order Terraform recorded the map in), and leaves an already-migrated list untouched so re-running is a no-op. The map-to-object-list restructure is now expressed once, as a `kv_list` rule in `codegen/state_upgrades.yaml`, and read by both the state upgrader and `kmigrate`'s config rewriter — previously only the config half knew about it, which is why the state half was missed. New snapshot-driven guards assert that every old-to-new attribute whose JSON shape changed is covered by a rule, and that every block passed through into a nested-object attribute still matches it field for field.
- Data sources whose SDK list envelope puts the items directly in `Data` (for example `UserListResponse{Data []User}`) are no longer silently downgraded to id-only. List resolution looked the trimmed field type up in the struct index, which for that shape is the literal string `[]User` — a slice type that can never be a struct — so resolution aborted, the list model stayed empty, and the id-only template was rendered instead of the list one. The only signal was a line on stderr while `kgen` still exited 0, which is why a large number of data sources shipped without a `filter` block and requiring an `id` their old-provider configurations never had. That shape is now handled by the same helper the items-field path already used.
- `kgen crud` no longer silently skips `no_read`-archetype resources. The service-package template branches on a field that the `no_read` call site did not pass, and Go's `text/template` errors on a field absent from the data struct rather than treating it as empty — so template execution failed, the resource was skipped with a line on stderr, and `kgen` still exited 0. Three resources (`aws_resource_tag`, `ou_cloud_access_role_exemption`, `project_cloud_access_role_exemption`) were therefore unregenerable, and their committed output had drifted from the templates with nothing reporting it. A `no_read` resource can now also register a companion data source, lifting a limitation its own code comment described.
- `kmigrate` now rewrites ownership blocks written as `dynamic` blocks. Every ownership block in the coverage surface is written as `dynamic`, which is how a configuration generates a repeatable block from a variable, and the rewriter recognised only literal blocks — so all 22 `owner_users` / `owner_user_groups` / `owner_groups` findings survived migration untouched. An attribute has no dynamic form, so the construct collapses into a `for` expression over the same `for_each`, with the block's iterator resolved to the loop variable (the two-variable form only when the body reads `.key`); a resource mixing literal and dynamic blocks of the same name gets a `concat`. Every generated expression is round-tripped through the HCL parser, so a rewrite that would not parse is reported rather than written, and a malformed dynamic block comes back as a follow-up instead of a panic. The same support covers the object-bodied blocks (`project_funding`, `budget`, `move_ou_settings`).
- `kmigrate` now drops `kion_gcp_account.create_mode` and `kion_custom_account.skip_access_checking`, two old settable attributes with no home in the new schema. A migrated configuration that still carried them failed `terraform validate` with "Unsupported argument". They had been held out by a policy stating that the four `*_account` resources migrate via import and are therefore not subject to config rewriting — which was not true, since `kmigrate` already rewrites `name` to `account_name` on those same resources. The exclusion is now narrowed to what still holds.
- `kmigrate` now drops attributes the new schema made read-only. `kion_project_note.create_user_id` was a required input on the old resource; the new one derives it from the authenticated caller and reports it back as computed, so configuration that still set it failed with "Invalid Configuration for Read-Only Attribute". This could not be expressed as an ordinary drop, whose guard asserts the attribute is absent from the new schema rather than present-but-computed, so it uses a sibling table with the opposite guard, checked both ways round by a test — any future settable-to-computed change now fails the build until it is covered.

## [1.0.0] - 2026-08-20


### BREAKING CHANGES

- Provider SDK imports moved from `kion-sdk-go/generated` to per-version sub-packages. The provider builds against `generated/v3_16`; the root `kion` package no longer exports `NewClient`.
- `kion_user` now requires `email`, `first_name`, `idms_id`, `last_name`, and `username`. The previous provider's `kion_user` had no configurable attributes at all — only a computed `id`, with read and delete and no create — so it could only adopt an existing user for deletion. The new resource is full CRUD. Existing state migrates without action (the read repopulates the new attributes), but an existing `resource "kion_user"` block must gain the five arguments or `terraform plan` fails with "Missing required argument". `kmigrate` reports each affected block; it cannot fill the values in.

### FEATURES

- `kgen` code generator with Go templates for resources, data sources, tests, and examples; `kmigrate` rewrites configurations written for the previous provider.
- Runtime Kion-version compatibility: the provider detects the connected Kion version at configure time and gates resources whose defining API operation does not exist on that release, instead of shipping a separate provider release line per Kion version.
- AWS-style service-package architecture across 72 service packages, with generated CRUD, schemas, data sources, acceptance-test scaffolding, examples, registry docs, and a Terraform module per resource.

### ENHANCEMENTS

- `kmigrate` now reports blocks missing an attribute the new schema requires and the old provider had no equivalent for — today only `kion_user`. It cannot supply the values, so these are listed separately from the edits it made, under "block(s) need attention before `terraform plan` will pass". Rename targets are excluded, since the rewrite fills those in from the old name. A guard test derives the set from the schema snapshots, so a future spec that makes an existing resource require something new fails the build until it is recorded.
- Label data source: backwards-compatible filter mode.
- `OptStringToFrameworkLegacy` flex helper for legacy SDK compatibility.
- Restored the `PostAccountCacheCreateNewAWS` endpoint with govcloud and organizational-unit support.

### BUG FIXES

- Fixed the raw HTTP endpoints (`app_role`, `billing_source`, `billing_source_aws`, `billing_source_gcp`, `billing_source_oci`, `dashboard`, `funding_source_note`, `permission_scheme`, `project_note`) appending a hardcoded `/api` on top of the already-resolved API root — requesting `/api/api/…` under a default configuration and ignoring `apipath`. `apipath` now applies to the SDK and the raw endpoints alike.
- Setting an attribute the connected Kion is too old to accept now fails at plan, naming the attribute and both versions. Previously nothing checked it: gating worked at the level of whole operations, so a field added in 3.16 to a resource that exists in 3.13 had no gate at all. Portal decodes request bodies with `json.Unmarshal` and no `DisallowUnknownFields`, so the field was not rejected but dropped — the value never landed, and the practitioner saw "Provider produced inconsistent result after apply" or a diff that never converged, neither of which names the cause. `kgen versions` now walks the create body across every tracked SDK version and records the oldest Kion accepting each attribute, which finds `kion_billing_source.azure_connection`, `kion_cloud_rule.automation_policy_ids`, and `kion_project_enforcement.notification_emails` / `notification_frequency` — all 3.16-only fields on resources with no version gate of their own. Attributes no newer than the resource itself are omitted, since the resource gate already refuses those.
- Fixed SDK fixspec silently dropping 9 of 13 query-string-discriminated operations (`PostAwsAccount`, `PostAzureSubscription`, `PostGoogleCloudAccount`, and others).
- A Kion instance too old for a resource is now reported during `terraform plan` rather than part-way through `apply`. Gated resources gain a `ModifyPlan` (destroy plans stay allowed, so an unsupported instance cannot strand a resource in state), and the gate now runs in `Read`, `Update` and `Delete` as well as `Create` — as `RequireKionVersionInRange`'s own documentation already described. Gating `Read` matters most: on an instance without the endpoint the API answers 404, which the generated read treats as "resource gone" and silently removes from state. `codegen/version_support.yaml` was also stale, leaving `kion_billing_source` (3.14+), `kion_dashboard` (3.15+) and `kion_scope` (3.15+) with no gate at all; regenerating it brings them and several data sources under the same protection.
- `make version` reported a hardcoded `3.15.1`. It read `info.version` from `spec/swagger-*.json`, which the repo carried until that file was replaced by `openapi3.json`; the glob has matched nothing since, so the fallback — the Kion spec version it happened to be pinned to — became the only value it could produce. That number also named the directory `make install` wrote to, and it described Kion's API rather than the provider. The version now comes from the latest release tag (`0.0.0-dev` when untagged), matching how a release is actually cut. The unused `version-get`/`version-set` targets and their `version/VERSION` file are gone, as is the hand-rolled `make release` cross-compile, which produced bare binaries the Registry would reject; `make release-snapshot` builds the real artifact set locally instead.

### NOTES

- Supports Terraform Protocol version 6.
- Releases are built by goreleaser from a tag push, producing the artifact set the Terraform Registry requires: per-platform zips, a GPG-signed `_SHA256SUMS`, and the registry manifest. Changelog entries are changie fragments under `.changes/unreleased/`, batched into `CHANGELOG.md` when a release is cut.
- Built against kion-sdk-go 0.9.0 (`generated/v3_16`).
## [0.1.0] - 2025-10-21

### NOTES

- Initial development version
- CI/CD infrastructure setup complete
- Automated changelog generation configured
