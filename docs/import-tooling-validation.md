# kion-import: live validation

Record of running `kion-import` against a real Kion installation. The authored
endpoint tables in `internal/kgen/importmanifest/paths.go` cannot be derived from
codegen, so they are unverified until a run like this exercises them. Re-run this
after changing an archetype, a read path, or one of those tables.

## Environment

| | |
|---|---|
| Install | a demo Kion installation (3.16.3) |
| Date | 2026-08-25, on `fix/records-skipped-no-id` (branched from `main` @ 89a9195) |
| Manifest | `codegen/import_manifest.json`, 68 resource types |
| Command | `kion-import --url https://kion.example.com --out imports.tf` |

Credentials come from `--api-key` or `KION_APIKEY`; none are recorded here.

## Result

```
Coverage: 68 resource types, 51966 records
  empty: 5, ok: 60, unsupported: 3
```

51,880 `import` blocks generated, **zero errors**, and — as of the page-padding
fix below — zero records skipped for a missing id. Every one of the 68 manifest
rows produced a result.

The three `unsupported` rows are refusals by design, not gaps: two alias types
(`kion_aws_cloudformation_template`, `kion_aws_iam_policy`) that would put two
Terraform resources in charge of one record, and `kion_custom_variable_override`,
whose identity is compound (`account_id` + `custom_variable_id`) and so is not
enumerable from a flat list even though its read exists.

### How this run compares

| run | install | records | ok | empty | error | unsupported |
|---|---|---|---|---|---|---|
| 2026-08-24 | production-scale | 53,513 | 49 | — | 15 | 4 |
| 2026-08-25 (earlier) | production-scale | 53,513 | 59 | 2 | 3 | 4 |
| 2026-08-25 @ 69c566c | demo (3.16.3) | 48,355 | 59 | 6 | 0 | 3 |
| 2026-08-25 @ `main` 89a9195 | demo (3.16.3) | 48,364 | 60 | 5 | 0 | 3 |
| **2026-08-25 @ `fix/records-skipped-no-id`** | **demo (3.16.3)** | **51,966** | **60** | **5** | **0** | **3** |

**The rows are not the same install** — the first two ran against a
production-scale install, the last three against a demo one. Compare the *status
columns*, which are properties of the tooling; the record counts are properties
of the install and are not comparable across rows.

The last two rows *are* the same install, an hour apart, so their record counts
are comparable: the 3,602-record jump is the page-padding fix below recovering
3,592 compliance controls and 10 compliance families. `scope_criteria` moving
from `empty` to `ok` between 69c566c and `main` is the earlier
`nested_collection` fix, not this one.

The last three errors and the fourth `unsupported` were cleared by the two fixes
recorded below (*Private reads that render SQL null wrappers* and *`no_read` did
not mean unreadable*). `kion_dashboard` and `kion_funding_source_note` now read;
`kion_aws_resource_tag` and both `*_cloud_access_role_exemption` types moved out
of "structurally unreadable" entirely.

The production-scale figures are the ones quoted in
[IMPORTING.md](IMPORTING.md): 53,513 records, of which 19,425 are Azure policy
definitions and 16,265 are compliance checks.

## Defects this run found

Five, none of which unit tests could have caught — each depends on a response
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

### 5. Pages padded to `total`, which stopped paging after page one

The compliance parent-scoped collections answer **every** page with an `items`
array whose length is the collection's `total`, populating only the requested
page's window and zero-filling the rest:

```
GET /v4/compliance/program/12/control?page=1&count=100
  -> total 1189, items 1189, of which 100 carry an id (records 1-100)
GET /v4/compliance/program/12/control?page=2&count=100
  -> total 1189, items 1189, of which 100 carry an id (records 101-200)
```

Each filler slot is a fully zero-valued struct:

```json
{"id":0,"compliance_family_id":0,"control_number":0,"name":"","description":"",
 "severity":"","title":"","compliance_levels":null,"cloud_provider_policy_ids":null}
```

`List` pages while `len(records) < total`. Page one already returned `total`
records, so it never asked for page two — and the 1,089 padding slots flowed
into `toRecords`, which correctly refused to import a record whose id is `0`
(that guard exists because emitting `id = "0"` produced `Cannot import
non-existent remote object`). The result read as a partial success with a
reported skip count rather than an error: `kion_compliance_control ok 2414,
3592 record(s) skipped: no id`, and `kion_compliance_family ok 587, 10
record(s) skipped: no id`. The skip count is not a coincidence — it is exactly
the records past page one, `total` minus 100 per program.

This is the same class as defects 2 and 3 — a response shape the client
mis-modelled — but the failure mode is the inverse: the earlier two lost the id
*within* a record, this one loses whole records to a **`total` the client
trusted as a length**.

Fixed: a paginated response whose page overshoots the requested page size has
its all-zero records dropped, and paging continues until `total` real records
are collected. The filter cannot cost anything real — an all-zero record has no
id and is unimportable either way — and it does not touch a well-behaved
endpoint, whose pages never exceed the page size. An endpoint that ignores
`count` and returns every (real) record on page one still yields all of them in
one request, since nothing gets dropped and the loop is already satisfied.

After the fix, on the same install: `kion_compliance_control ok 6006`,
`kion_compliance_family ok 597`, no skips.

**`total` is a count, not a length.** Nothing in a list response guarantees the
array is only as long as the page.

## Still failing on this install

Nothing errors. Three rows are refused by design, five are legitimately empty,
and five read with a caveat. **All are recorded, not silently omitted** -- they
appear in both the report and the generated file. No row skips a record for a
missing id.

### Refused by design (3)

`custom_variable_override` needs two ids in its path, neither discoverable
without the other. `aws_cloudformation_template` and `aws_iam_policy` are alias
tf_types; see *Alias types were imported twice* below.

This list used to include `aws_resource_tag`, `ou_cloud_access_role_exemption`
and `project_cloud_access_role_exemption` as "structurally unreadable". That was
wrong -- see *`no_read` did not mean unreadable*.

### Read with caveats (5)

| resource | caveat |
|---|---|
| `idms_group_association` | 2 parents failed; `GET /v3/idms/1/group-association` 502 |
| `saml_group_association` | same 502, same parent |
| `idms_open_id_access_rule` | 10 parents had none |
| `ou_cloud_access_role_exemption` | 237 records of another kind sharing the collection |
| `project_cloud_access_role_exemption` | 12 records of another kind, so none remain |

The 502 is server-side and specific to IDMS 1 on that install -- IDMS 2/3/4
answer 200/404 normally, and three consecutive retries all returned 502. It is
reported rather than swallowed, and one bad parent does not sink the resource.

The two exemption caveats are the kind-mixing filter working; see below.

### Empty (5)

Five collections are genuinely empty on this install, including
`kion_aws_resource_tag` -- which is why its new read has **no live coverage**.
Its field names come from the spec's `AWSResourceTag` component and are exercised
only by unit tests. Verify it against an install that has records.


## What this run validated

- **The parent-scoped fallback works.** `compliance_family` and `compliance_level`
  405 on their flat lists and fall back automatically, returning 597 and 64 records.
- **Skip counting is load-bearing.** Before the wrapper fix, this run reported
  `kion_azure_policy: empty, 0 records` as a clean result. With skip counts it
  reported `19425 record(s) skipped: no id`. Roughly 23,000 records were being
  dropped silently; the count is what surfaced it. It earned its keep a second
  time on the page-padding defect: a resource returning 2,414 records looks
  healthy, and only `3592 record(s) skipped` said otherwise.
- **The one-result-per-row contract holds** against the real manifest: 68 rows in,
  68 results out, every failure attributed.

## Private reads that render SQL null wrappers

Enumerating a record is not the same as reading it. `--probe` only exercises the
list endpoint, so `kion_project_note` and `kion_dashboard` both reported `ok`
while every one of their records failed at `terraform plan
-generate-config-out`, where the provider's own `Read` runs:

```
decoding response: json: cannot unmarshal object into Go struct field
project_noteWire.data.updated_at of type string
```

Their private reads render several columns through Go's `sql.Null*` types, so
those arrive as objects and the whole response fails to decode. Observed
directly against a live install:

| endpoint | attribute | on the wire |
|---|---|---|
| `/v2/project-note/{id}` | `last_update_user_id` | `{"Int":0,"Valid":false}` |
| `/v2/project-note/{id}` | `updated_at` | `{"Time":"0001-01-01T00:00:00Z","Valid":false}` |
| `/v1/dashboard/{id}` | `description` | `{"String":"…","Valid":true}` |
| `/v1/dashboard/{id}` | `updated_at` | `{"Time":"…","Valid":true}` |
| `/v1/dashboard/{id}` | `config` | the JSON **object**, not a string of JSON |

`created_at` is a plain string on both, so the wrapping is per-column and cannot
be inferred from the attribute's name or type. `spec_additions.yaml` declares the
private Dashboard by hand and had all three of these wrong, which is why the
generated wire struct disagreed with reality.

Declare the real shape per attribute in `private_endpoints.yaml` under
`read_kinds` (`null_int`, `null_string`, `null_time`, `json_string`). Unlike
`read_shape` this keeps the single wire struct, so a resource whose write is also
raw keeps working: the `flex.Null*` types encode back to the bare scalar.

**When a resource fails only under `plan`, fetch its private read directly and
compare it field by field against the generated wire struct.** The probe cannot
see this class of failure.

## `no_read` did not mean unreadable

Three resources — `kion_ou_cloud_access_role_exemption`,
`kion_project_cloud_access_role_exemption` and `kion_aws_resource_tag` — used the
`no_read` archetype, whose `Read` is a no-op that echoes state. `ImportState` sets
only the id, so `-generate-config-out` wrote **empty bodies**:

```hcl
# __generated__ by Terraform from "14"
resource "kion_ou_cloud_access_role_exemption" "kion_ou_cloud_access_role_exemption_14" {
}
```

The plan reported `0 to change` because every attribute is Optional+Computed and
null, so this looked green while describing nothing.

`no_read` means the public spec has no **by-id** GET, not that the record cannot
be read. Both exemptions are readable from the private
`/v1/{ou,project}/{id}/cloud-access-role-exemption` collection, and
`aws_resource_tag` from the public flat `/v3/aws-resource-tag`. All three now
declare a `parent_read` in `private_endpoints.yaml`.

Two properties of those collections had to be handled, both measured live:

- **Inherited, not owned.** `/v1/ou/{id}/cloud-access-role-exemption` returns
  every exemption visible to that OU's subtree, so one record comes back under
  many OUs — 329 rows for 22 distinct records, 22 OUs returning 5 distinct sets.
  The id in the path is therefore *not* the record's owner; its own `OUID` is.
  `Parent.ParentIDJSON` tells the enumerator to use it, which is what makes the
  `"<parent>/<id>"` import id resolve.
- **Kind-mixing.** The same collection returns cloud **rule** exemptions
  alongside cloud **access role** exemptions. Only 6 of the 22 OU records were
  the latter, and **0 of 12** on the project side. `Resource.RequireValidField`
  names the discriminator (`ou_cloud_access_role_id` being `Valid`); the drops
  are reported in the run's Reason, never silent.

After the fix, the same install enumerates 6 OU exemptions with their true owners
(`1/5`, `1/7`, `1/24`, `1/25`, `11/4`, `42/26`) and generates real configuration
for each. `reason` is in the schema and in the create body but absent from the
read payload, so it stays unreadable.

**A green `terraform plan` is not proof of coverage.** Check that the generated
configuration has attributes in it.

## Reproducing

```sh
make build-tools
export KION_APIKEY=…
./bin/kion-import --url https://kion.example.com --probe          # read outcomes only
./bin/kion-import --url https://kion.example.com --out imports.tf # generate
```

Narrow a run with `--include` / `--exclude` / `--selection`; see
[IMPORTING.md](IMPORTING.md). Importing everything is rarely what an operator
wants: Kion's shipped policy and compliance catalogs dominate the count. On the
install above, excluding `kion_azure_policy`, `kion_compliance_check`,
`kion_compliance_control` and `kion_compliance_standard` takes the run from
51,966 records to 10,150.

Add `--api-prefix ""` for a localhost app serving the API at the root.
