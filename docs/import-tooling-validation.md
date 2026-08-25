# kion-import: live validation

Record of running `kion-import` against a real Kion installation. The collection
paths a parent-scoped or association resource reads through are authored in
`codegen/config_overrides.yaml` rather than derived -- codegen records by-id
reads, which those shapes have none of -- so they are unverified until a run like
this exercises them. Re-run this after changing an archetype, a read path, or one
of those entries.

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

These totals are that run's, and they are now ten low: the padding fix in
*Pages padded to `total`* recovers ten `compliance_family` records that this run
never fetched. The per-resource sections below reflect the fixed behavior; the
headline counts are left as recorded rather than adjusted by arithmetic, and a
full re-run supersedes them.

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

### 5. Pages padded to `total`, hiding real records

Found after this run, by investigating the `record(s) skipped: no id` caveats
below rather than accepting them as a documented quirk.

Some collections pad every page out to `total` instead of to the page size.
`/v4/compliance/program/5/family` holds 110 families:

| request | items | real | zero-valued padding |
|---|---:|---:|---:|
| `count=100&page=1` | 110 | 100 (Access…Temporary) | 10 |
| `count=100&page=2` | 110 | 10 (Transaction…Wireless) | 100 |
| `count=500&page=1` | 110 | 110 | 0 |

`Client.List` unwrapped 110 records and `total: 110` from page 1, evaluated
`len(records) < total` as `110 < 110`, and stopped. The ten real records that
exist only on page 2 were never requested. `toRecords` then dropped the ten
fillers for having no id and reported `10 record(s) skipped: no id`.

The count was right and the wording was wrong in the way that matters: it
describes records being *ignored* when the event was records never being
*fetched*. It also happened to equal the number missing, which made it look
self-consistent. What gave it away was the alphabet -- every missing family
sorted between Transaction and Wireless, which is a lost final page, not
scattered bad data.

Fixed: zero-valued records are dropped before the page count is compared against
`total`. Such a record carries no id and could never have produced an `import`
block, so nothing is lost, and the count now reflects what was retrieved.
`compliance_family` returns the full 597, matching a direct count over the API.

The padding itself is a server-side defect and is worth reporting separately:
any client paging that endpoint hits it.

## Still failing on this install

Nothing errors. Three rows are refused by design, six are legitimately empty, and
five read with a caveat. **All are recorded, not silently omitted** -- they
appear in both the report and the generated file.

The same padding hides far more `compliance_control`. That collection is
parent-scoped under 28 programs, each capped at its first 100 controls:

```
GET /v4/compliance/program/12/control?count=100&page=1
  -> total 1189, items 1189, of which 100 carry an id
```

`compliance_family` lost 10 records to this; `compliance_control` lost **3,592**.
Re-probed on the fixed build: 2,414 -> 6,006 and 587 -> 597, a total of +3,602,
which is exactly the two skip counts.


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

This table used to carry three more rows -- `compliance_control` (3592),
`compliance_family` (10) and `scope_criteria` (9), each reported as
`record(s) skipped: no id`. None of them were what that wording implied; see
*Pages padded to `total`* below for the first two, and the `nested_collection`
read shape for `scope_criteria`. All three now read clean:

| resource | then | now |
|---|---|---|
| `compliance_control` | 6006, 3592 skipped | 6006, no caveat |
| `compliance_family` | 587, 10 skipped | **597**, no caveat |
| `scope_criteria` | 9 skipped, 0 read | 9, no caveat |

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
- **A skip count is a symptom, not a diagnosis.** The same mechanism that
  surfaced the wrapper bug later described missing `compliance_family` and
  `compliance_control` records as *skipped* ones, and the numbers were accurate
  enough that the caveats sat in this document as settled. A skip count says
  something was not emitted; it does not say the records were reachable and
  ignored. Reconcile against the API before recording one as understood.
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
