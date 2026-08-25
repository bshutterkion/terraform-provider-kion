# kion-import: live validation

Record of running `kion-import` against a real Kion installation. The authored
endpoint tables in `internal/kgen/importmanifest/paths.go` cannot be derived from
codegen, so they are unverified until a run like this exercises them. Re-run this
after changing an archetype, a read path, or one of those tables.

## Environment

| | |
|---|---|
| Install | `demo1.kion.io` |
| Date | 2026-08-24 |
| Manifest | `codegen/import_manifest.json` — 68 resources, 65 readable |
| Command | `kion-import --url https://demo1.kion.io --probe` |

Credentials come from `--api-key` or `KION_APIKEY`; none are recorded here.

## Result

```
Coverage: 68 resource types, 47324 records
  ok: 49, empty: 1, error: 15, unsupported: 3
```

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

`https://demo1.kion.io` returns the web application's HTML **with status 200**;
only `https://demo1.kion.io/api` returns API JSON. A status-code check cannot
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

Fifteen resources error and three are structurally unreadable. **These are recorded,
not silently omitted** — they appear in both the report and the generated file.

### Structurally unreadable (expected, by design)

`aws_resource_tag`, `ou_cloud_access_role_exemption`,
`project_cloud_access_role_exemption` — all declared `kind: no_read` in
`crud_archetypes.yaml`: no by-id GET and no listable collection.

### Flat list not served (405) — needs a parent-scoped or per-id read

`budget`, `compliance_control`, `dashboard`, `idms_group_association`,
`idms_open_id`, `ou_cloud_access_role`, `ou_note`, `project_cloud_access_role`,
`saml_group_association`, `scope_criteria`

Probed alternates that also failed: `/v3/ou/1/cloud-access-role` → 404,
`/v3/ou/1/note` → 404, `/v4/compliance/control` → 405. `/beta/scope/{id}/criteria`
405s for all 9 scopes, so `ParentPaths`' entry for `scope_criteria` is wrong.

Note `project_note` **does** read (5 records) from `/v3/project-note` while
`ou_note` 405s on `/v3/ou-note` — the two are not symmetric.

### Authored path wrong (404)

`custom_variable_override`, `funding_source_note`, `global_permission_mapping` —
the `SpecialPaths` / `GlobalAssociationPaths` entries do not exist on this install.

### Needs a parent id in the path (400)

`idms_open_id_access_rule`, `idms_open_id_group_association` — both return
`"Failed to parse to a number"`, meaning the endpoint expects an id segment. They
should become parent-scoped reads under `idms_open_id`, which itself 405s and must
be resolved first.

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
./bin/kion-import --url https://demo1.kion.io --probe          # read outcomes only
./bin/kion-import --url https://demo1.kion.io --out imports.tf # generate
```

Add `--api-prefix ""` for a localhost app serving the API at the root.
