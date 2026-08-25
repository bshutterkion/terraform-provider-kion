# kion-import: live validation

Record of running `kion-import` against a real Kion installation. The authored
endpoint tables in `internal/kgen/importmanifest/paths.go` cannot be derived from
codegen, so they are unverified until a run like this exercises them. Re-run this
after changing an archetype, a read path, or one of those tables.

## Environment

| | |
|---|---|
| Install | a production-scale Kion installation |
| Date | 2026-08-25 (superseding 2026-08-24) |
| Manifest | `codegen/import_manifest.json`, 68 resource types |
| Command | `kion-import --url https://kion.example.com --probe` |

Credentials come from `--api-key` or `KION_APIKEY`; none are recorded here.

## Result

```
Coverage: 68 resource types, 53513 records
  ok: 59, empty: 2, error: 3, unsupported: 4
```

49,556 `import` blocks generated. **Zero records skipped for a missing id.**

Largest reads: `kion_azure_policy` 19425, `kion_compliance_check` 16265,
`kion_compliance_control` 6006, `kion_project_permission_mapping` 1443,
`kion_gcp_iam_role` 1382, `kion_iam_policy` 1348,
`kion_funding_source_permission_mapping` 1035, `kion_azure_role` 828,
`kion_ou_permission_mapping` 726, `kion_compliance_family` 588.

The 2026-08-24 run reported `ok: 49, error: 15`. Ten of those fifteen now read,
including every resource previously listed under "flat list not served": `budget`
9, `compliance_control` 6006, `idms_group_association` 18, `idms_open_id` 1,
`ou_cloud_access_role` 13, `project_cloud_access_role` 62,
`saml_group_association` 18, `scope_criteria` 9. `ou_note` returns an empty
collection rather than 405.

47,324 `import` blocks generated. **Zero records skipped for a missing id.**

Largest reads: `kion_azure_policy` 19425, `kion_compliance_check` 16265,
`kion_gcp_iam_role` 1382, `kion_iam_policy` / `kion_aws_cloudformation_template`
1348, `kion_project_permission_mapping` 1443, `kion_funding_source_permission_mapping`
1035, `kion_azure_role` 828, `kion_ou_permission_mapping` 726, `kion_compliance_family`
597.

## Defects this run found

Three, none of which unit tests could have caught — each depends on a response
shape only a real install produces.

### 1. `/api` prefix, and an HTTP 200 that is not JSON

`https://kion.example.com` returns the web application's HTML **with status 200**;
only `https://kion.example.com/api` returns API JSON. A status-code check cannot
distinguish them, so the original client fed HTML to the JSON parser for all 68
resources and produced 68 identical unmarshal errors.

Fixed: the client appends `/api` by default, `--api-prefix` overrides it (its only
real use is an employee hitting an app directly on localhost, where the API is at
the root), and a non-JSON body now produces one error naming the likely cause
instead of 68 parse failures. Matches `kion-env-copy`'s `KION_API_PREFIX`
convention.

### 2. Doubly-nested list envelope

`/v4/billing-source`, `/v3/label`, `/beta/scope` and others return:

```json
{"status":200,"data":{"pagination":{…},"total":17,"items":[…]}}
```

`unwrap` handled `{items,total}` at the top level and `data` as an array, but when
`data` was an object it took the single-object branch and returned the inner
envelope itself as one record — producing exactly one id-less record per resource.

Fixed: an object `data` carrying `items` is unwrapped, and the inner `total` drives
paging. A genuine single-object `data` still yields one record.

### 3. Per-type record wrapper

A family of `/v3/*` endpoints wraps each record under a type key alongside sibling
arrays:

```
/v3/cft                -> {"cft":{…,"id":296}, "owner_users":[], "owner_user_groups":[], "tags":[]}
/v3/azure-role         -> {"azure_role":{…}, "car_restricted_users":[], "car_restricted_ugroups":[], …}
/v3/service-catalog    -> {"service_catalog_portfolio":{…}, …}
/v3/gcp-iam-role       -> {"gcp_role":{…}, …}
```

The real `id` is one level down, so every record was skipped — about 23,000 across
ten resources.

**The wrapper key is not the resource kind.** `service_catalog` wraps under
`service_catalog_portfolio` and `gcp_iam_role` under `gcp_role`. Detection must be
structural, never derived from the tf_type.

Fixed with a rule that fits every observed shape: if a record has no top-level `id`
and **exactly one** of its keys maps to an object that itself contains an `id`, that
inner object is the record. Sibling arrays and scalars are ignored; two candidates
means ambiguous and the record is left untouched. `Record.Raw` keeps the outer
object.

## Still failing on this install

Three error, four are structurally unreadable, two are legitimately empty. **All
are recorded, not silently omitted** -- they appear in both the report and the
generated file.

### Structurally unreadable (expected, by design)

`aws_resource_tag`, `ou_cloud_access_role_exemption`,
`project_cloud_access_role_exemption` are declared `kind: no_read` in
`crud_archetypes.yaml`: no by-id GET and no listable collection.
`custom_variable_override` needs two ids in its path, neither discoverable
without the other.

### Errors

| resource | |
|---|---|
| `dashboard` | `GET /beta/dashboard` 405 |
| `funding_source_note` | `GET /v2/funding-source-note` 405 |
| `idms_open_id_access_rule` | all 10 parents 404: `GET /v4/idms/open-id/1/access-rule` |

`idms_open_id` itself now reads one record, so the access-rule read is reaching
parents that exist but hold no access rules; the 404 is the API's answer for an
IDMS with none, not a wrong path.

### Empty

`account_linkage` and `ou_note` return empty collections. `ou_note` 405'd on the
previous run, so this is the endpoint behaving differently, not a code change.


### 4. Alias types were imported twice

`kion_aws_iam_policy` and `kion_iam_policy` are two names for one implementation
over one endpoint, as are `kion_aws_cloudformation_template` and `kion_cft`. The
manifest gave each its own row with the same `list_path`, so both were
enumerated: 1,348 identical ids under two type names, plus 296 more, for 1,644
duplicate `import` blocks. Applying them would have put two Terraform resources in
charge of each Kion record, each reverting the other's drift.

The schema snapshot the manifest is built from lists every tf_type the provider
serves, aliases included. `kindAliases` existed to map an alias back to its
generator_config key for lookup, but nothing stopped the alias getting its own
enumerable row.

Fixed: an alias row carries `alias_of` and `readable: false`, so it is still
accounted for in the report but never enumerated. `--list-types` shows the legacy
name against the current one, which is what an operator migrating a configuration
needs to see.

## What this run validated

- **The parent-scoped fallback works.** `compliance_family` and `compliance_level`
  405 on their flat lists and fall back automatically, returning 597 and 64 records.
- **Skip counting is load-bearing.** Before the wrapper fix, this run reported
  `kion_azure_policy: empty, 0 records` as a clean result. With skip counts it
  reported `19425 record(s) skipped: no id`. Roughly 23,000 records were being
  dropped silently; the count is what surfaced it.
- **The one-result-per-row contract holds** against the real manifest: 68 rows in,
  68 results out, every failure attributed.

## Reproducing

```sh
make build-tools
export KION_APIKEY=…
./bin/kion-import --url https://kion.example.com --probe          # read outcomes only
./bin/kion-import --url https://kion.example.com --out imports.tf # generate
```

Narrow a run with `--include` / `--exclude` / `--selection`; see
[IMPORTING.md](IMPORTING.md). Importing all 53,513 records is rarely what an
operator wants, since Kion's shipped policy and compliance catalogs are 35,690 of
them.

Add `--api-prefix ""` for a localhost app serving the API at the root.
