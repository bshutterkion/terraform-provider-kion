# Code generation

Everything the provider ships is generated from `spec/openapi3.json`. This file
is the guide to the inputs in this directory, the invariants that hold, and the
traps that have cost real time.

## Two paths, one spec

```
spec/openapi3.json
  |
  |-- tfplugingen ------------> internal/service/<n>/<n>_schema_gen.go   resource schema
  |
  `-- ogen --> kion-sdk-go --> kgen crud --> <n>.go                      resource CRUD
                                             <n>_data_source.go          the whole data source
```

The SDK is ogen-generated from the same spec, so both paths are spec-derived.
They are not two sources of truth.

**Resources** get their schema from tfplugingen and their CRUD from `kgen crud`.
**Data sources** are generated whole by `kgen crud`, schema included.

tfplugingen also *can* emit a data-source schema, but it maps the HTTP response
literally, so the result is the list envelope (`count`, `page`, `query`,
`sort_method`, `sort_order`, nested `data`) rather than the record a practitioner
selects. `kgen crud` unwraps the envelope and adds the `filter` blocks. Those
tfplugingen data-source schemas are therefore written only for packages that read
one, which is `gcp_regions` alone; see `dataSourceSchemaConsumers` in
`internal/kgen/schemas/schemas.go`. 48 of them used to be emitted, referenced by
nothing, and one drifted for months before anyone noticed.

## The inputs

| File | Generated? | Purpose |
|---|---|---|
| `generator_config.yaml` | **yes**, by `kconfig gen --write` | Per-resource CRUD paths driving schema generation |
| `config_overrides.yaml` | no | Paths and ignores `kconfig` cannot derive. Merged over the derivation |
| `schema_overrides.yaml` | no | What the spec cannot express: descriptions, plan modifiers, retypes, `sensitive`, attribute removal |
| `spec_additions.yaml` | no | Endpoints and property types the public spec omits. Merged into a working copy of the spec |
| `renames.yaml` | no | Attribute renames, applied before schema overrides |
| `crud_archetypes.yaml` | no | Which CRUD shape each resource uses |
| `private_endpoints.yaml` | no | Endpoints absent from the public spec, served over raw HTTP |
| `memberships.yaml`, `state_upgrades.yaml`, `test_values.yaml`, `version_support.yaml` | mixed | See the header comment in each |

**Never hand-edit `generator_config.yaml`.** It is regenerated from the service
packages plus `config_overrides.yaml`. Anything you add by hand is lost on the
next `kconfig gen --write`, silently. If derivation cannot produce what you need,
put it in `config_overrides.yaml`, which is where the five entries below live.

## Invariants

`make codegen-check` enforces both. Run it after touching anything here.

- `kconfig check` reports drift between `generator_config.yaml` and the code. It
  must say `config in sync with code`.
- `kconfig gen --write` is idempotent, and produces no diff against the committed
  file. If regenerating changes it, someone hand-edited it.

Neither runs in `make ci`, because both need `spec/openapi3.json` and it is
gitignored. CI and a plain clone cannot check this, so it is on you.

If either breaks, derivation and the committed file have diverged and the file is
no longer generated in any meaningful sense. Both were broken before this was
written: five entries existed only by hand, and regenerating dropped `count` and
`page` from `billing_source`'s ignores, which fails schema validation because
`count` is a reserved Terraform root attribute name.

## Derivation guesses, and sometimes guesses wrong

`codegen/import_manifest.json` takes its list paths from the `data_sources`
section, so a wrong entry there is not cosmetic: it sends `kion-import` at a path
that cannot be listed. Two were caught this way when the config was first made
reproducible.

- `saml_group_association` derived `/v3/idms/group-association/{id}`, whose
  collection is POST only. The records hang off the parent, so the read is pinned
  to `/v3/idms/{id}/group-association`.
- `custom_variable_override` is polymorphic across account, OU and project, so
  derivation resolved whichever GET the data source reached first, `/v3/ou`, and
  called it the collection. There is no single list, so the entry is suppressed
  with `skip: true`. It is still importable: `kion-import` reads all three
  `/v3/{entity}/{id}/custom-variable` collections from `multiParentOverrides` in
  `internal/kgen/importmanifest/generate.go`, not from this file.
- `ou_permission_mapping` and `project_permission_mapping` create and update
  through the same PATCH upsert, and derivation attributes that one PATCH to
  create alone. Without a pinned `update` they become create-and-replace only.

Check a derived path before trusting it: it must exist in the spec with a GET,
and it must be the collection rather than the by-id route.

## What derivation cannot see

`kconfig` derives a resource's ops from the SDK calls its CRUD methods make. Five
entries have no such calls to find and are supplied by `config_overrides.yaml`:

- `funding_source_note` is served entirely over raw HTTP through `internal/conns`,
  so it calls no SDK op at all.
- `funding_source_enforcement` and `ou_enforcement` read a list under their parent
  (`GET .../enforcement`), which no naming convention finds. Without the override
  both render as `INCOMPLETE, missing read` and are dropped, because tfplugingen
  requires create and read.
- The `ou` and `user_group` data sources reach their read through a helper, so the
  SDK call is not attributed to the package.

## Traps

**An untyped spec property drops the whole type.** tfplugingen refuses a property
with no `type` and no `allOf`/`oneOf`/`anyOf`, and it discards the entire resource
or data source rather than that one field. It logs the skip and exits 0, so the
run looks successful while the previously generated file sits stale on disk.
`scope`, `scope_criteria` and the `custom_variable` data source were all skipped
this way, and stayed stale long enough to miss a schema-wide change. `kgen schemas`
now compares the produced code spec against the config and fails, naming each
missing type. Fix it by giving the property a type under `property_types` in
`spec_additions.yaml`, or by ignoring it in `config_overrides.yaml`.

**An `ignores` entry must match the path tfplugingen reports.** It is not fuzzy.
The `custom_variable` data source ignored `data.default_value` where the real path
is `data.items.default_value`, because the list response nests records under
`items`. The ignore was never applied and the data source was dropped anyway. The
path appears in the warning text.

**Blocks cannot be generated.** The provider code spec has no representation for
them, so tfplugingen emits nested attributes only. This is why `account` and
`aws_account` still declare their schema by hand: they expose
`aws_organizational_unit` and `move_project_settings` as blocks, and delegating
would change the HCL practitioners write from `foo { ... }` to `foo = { ... }`.
It is also why `kgen crud` owns data sources, 38 of which have a `filter` block.
Do not expect a configuration change to fix this; it needs block support in the
upstream toolchain.

**Overrides are add-if-missing.** Naming an attribute that does not exist creates
it rather than failing. Name one at the wrong nesting depth and you get a second,
unpopulated attribute alongside the real one. Nested overrides merge into the
existing children, so overriding one field of a nested object leaves its siblings
alone.

**`config_overrides.yaml` accepts `ignores` at the entry top level or under
`schema:`.** Both work. Unknown keys fail the generate rather than being dropped,
which they were until four billing-source resources were found leaking list-query
parameters into their schemas.

## Adding a resource

```bash
go run ./cmd/kgen service --name CloudRule --snakename cloud_rule
# add entries to crud_archetypes.yaml, and to config_overrides.yaml if the paths
# do not derive, then:
make generate          # version gates, schemas, CRUD
make modules examples docs
```

Adding or removing a resource changes four generated trees: `internal/service/`,
`examples/`, `docs/` and `modules/`. `make ci` gates the last three, so a missed
regeneration fails the build rather than shipping stale documentation.

## Regenerating

```bash
make refresh-spec        # fetch the spec locally (maintainers; needs the SDK monorepo)
make generate-schemas    # tfplugingen: resource schemas
make crud-force          # kgen crud: CRUD and data sources, overwriting
make modules docs        # Terraform modules and registry docs
go run ./cmd/kconfig gen --write   # generator_config.yaml
```

`spec/openapi3.json` is gitignored. The generated code is committed, so a plain
clone builds, tests, and regenerates modules, docs and examples with no spec.
Only the two schema generators need it.
