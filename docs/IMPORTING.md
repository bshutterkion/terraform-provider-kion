# Importing an existing Kion install

`kion-import` reads a live Kion install and writes Terraform `import` blocks.

It deliberately does not write configuration or state. Every resource implements
`ImportState`, so `terraform plan -generate-config-out` produces the
configuration and `terraform apply` writes the state, both through the provider's
own Read. What Terraform cannot do is discover what exists in Kion, and that is
the only job this tool has.

```sh
export KION_URL=https://kion.example.com
export KION_APIKEY=…

kion-import --list-types                       # what can be imported
kion-import --probe                            # what exists, writing nothing
kion-import --out imports.tf                   # generate

terraform init
terraform plan -generate-config-out=generated.tf

kion-import rewrite-refs                       # literals -> references
terraform apply
```

## Rewrite the literal ids into references

`terraform plan -generate-config-out` writes every foreign key as a bare
integer, because Terraform has no way to know the value points at anything:

```hcl
resource "kion_project" "kion_project_9" {
  ou_id = 11
}
```

That is accurate for the install it came from, and it is a poor configuration.
It does not express its own dependency graph, so `terraform destroy` followed by
a re-apply will not order correctly and recreating an OU will not cascade. Point
it at a **different** install and it is worse than poor: id 11 either does not
exist or, silently, belongs to something else entirely.

```sh
kion-import rewrite-refs --imports imports.tf --generated generated.tf
```

```hcl
resource "kion_project" "kion_project_9" {
  ou_id = kion_ou.kion_ou_11.id
}
```

Terraform now resolves the value from the id it actually created. **This is why
moving a configuration between installs needs no id map** — the dependency graph
is the id map, and it brings correct ordering and parallelism with it.

Both files are required. `generated.tf` contains no ids (they are Computed, so
`-generate-config-out` omits them), so the id-to-label index comes from the
import blocks, where `to` and `id` sit together.

Use `--dry-run` to see what would change, and `--out` to write elsewhere instead
of rewriting in place. Running it twice is a no-op.

### What is left alone, and why

**Attributes that are not references.** Not every `*_id` points at a Kion
record. `portfolio_id` is the portfolio's id *in AWS*; `account_type_id` is an
enum; `car_external_id` is an external identifier. Rewriting one would aim a
Terraform reference at a foreign id space. The full classification, with a
reason for every entry, is [`codegen/references.yaml`](../codegen/references.yaml).

**`parent_ou_id = 0`.** That means "top-level OU", not "the OU whose id is 0". A
reference there would invent a parent.

**Referents this configuration does not manage.** These are reported, never
silently kept:

```
7 reference(s) left as literals -- their referent is not managed here:
  kion_ou_cloud_access_role                3
  kion_user                                4
```

That is the boundary of your configuration. Three ways to close it:

1. **Import them too** — add the type to your `--include` and re-run. Usually
   the right answer for Kion-managed resources.
2. **A data source**, when the record already exists on the target and you want
   to bind by name rather than manage it:
   ```hcl
   data "kion_user" "alice" { filter { ... } }
   ```
3. **A variable**, when the value differs per environment.

Users, IDMS and stock permission schemes are the common cases — they come from
an identity provider or ship with the install, so they are rarely yours to
manage.

**One side of a relation that is settable from both.** A user names its groups
(`user_group_ids`) and a group names its users (`user_ids`). Both are genuine
foreign keys, but rewriting both directions closes a loop, and Terraform refuses
to plan a configuration containing one. Only the first direction offered becomes
a reference; the other keeps its literal, and is reported separately:

```
103 reference(s) left as literals -- a reference would be a dependency cycle:
  kion_user.user_group_ids                 45
  kion_user_group.user_ids                 57
  kion_user_group.viewer_user_ids          1
```

Nothing is missing here and there is nothing to import: the relation is already
expressed, from the other end. The refusal is per id, so a list keeps every
reference that was not part of a loop, and which direction wins is fixed by
block order in the file — the same edge loses on every run.

### Attributes the read cannot recover

Import can only give a resource what its read returns. A couple of resources
accept a value on create that no read — public or private — gives back, so it
imports as `null` even though it was set:

| resource | attribute | why |
|---|---|---|
| `kion_ou_cloud_access_role_exemption` | `reason` | the private read backing it (`/v1/ou/{id}/cloud-access-role-exemption`) omits `reason`, and no other endpoint returns it either |
| `kion_project_cloud_access_role_exemption` | `reason` | same gap, on `/v1/project/{id}/cloud-access-role-exemption` |

Both attributes are Optional+Computed, so nothing errors and `terraform plan`
is clean — the value is just gone. If the reason recorded against an exemption
matters to you, re-enter it by hand after import.

## Choose what to import

**Importing everything is rarely what you want.** Kion ships large policy and
compliance catalogs. On a real install those dwarf everything a team manages in
Terraform: one production-scale install enumerates 53,513 records, of which
19,425 are Azure policy definitions and 16,265 are compliance checks.

Narrow the run with flags, a file, or both.

```sh
# just these
kion-import --include kion_ou,kion_project,kion_label --out imports.tf

# everything except the catalogs
kion-import --exclude kion_azure_policy,kion_compliance_check --out imports.tf
```

For anything you will run more than once, keep the selection in a file next to
the configuration, where it can carry comments:

```yaml
# kion-import.yaml
exclude:
  - kion_azure_policy      # Kion's shipped catalog, ~19k records
  - kion_compliance_check  # ditto, ~16k
  - kion_compliance_control
```

```sh
kion-import --selection kion-import.yaml --out imports.tf
```

`--include` and `--exclude` add to the file rather than replacing it, so the file
holds the standing policy and a flag covers a one-off. `--include`, when present,
is the entire set; `--exclude` is applied after it.

A type name that matches nothing is an error, not a silent no-op:

```
Error: unknown resource type(s): kion_labels
run with --list-types to see the 68 the manifest knows
```

## Aliases

Three resources are served under two names, for configurations written against
the previous provider:

| legacy name | current name |
|---|---|
| `kion_aws_cloudformation_template` | `kion_cft` |
| `kion_aws_iam_policy` | `kion_iam_policy` |
| `kion_cached_account` | `kion_account_cache` |

Both names reach one implementation over one endpoint, so importing both would
read the same objects twice and put two Terraform resources in charge of one Kion
record. `kion-import` enumerates only the current name; `--list-types` shows the
legacy ones marked `alias of`.

## Reading the report

Every type the tool knows produces exactly one line, so nothing is silently
absent:

```
kion_label                                    ok             225
kion_account_linkage                          empty            0
kion_dashboard                                error            0  GET /beta/dashboard: 405 Method Not Allowed
kion_aws_resource_tag                         unsupported      0  crud_archetypes.yaml declares kind: no_read

Coverage: 68 resource types, 53513 records
  ok: 59, empty: 2, error: 3, unsupported: 4
```

| status | meaning |
|---|---|
| `ok` | read, with a record count |
| `empty` | read, nothing there |
| `error` | the read failed; the reason is the API's |
| `unsupported` | no way to enumerate it, or a second name for another type |

Errors and unsupported types appear in the generated file as comments too, so the
output records what it could not reach rather than quietly omitting it.

## Other flags

| flag | |
|---|---|
| `--api-prefix` | defaults to `/api`. Use `--api-prefix ""` for an app serving the API at the root |
| `--skip-ssl-validation` | skip TLS verification |
| `--provider-version` | version constraint written into the generated config |
| `--manifest` | override the embedded manifest. Maintainers only; see `codegen/README.md` |

## Differences from the previous importer

The Python importer in the SDKv2 provider generated `.tf` files itself and walked
the OU and project tree. This one emits import blocks and enumerates each
resource's collection endpoint, so counts are not comparable between the two: the
old tool reached a policy only if something it walked referenced it, while this
one lists the whole collection.

It also has no equivalent of the old `--sync` (pushing repository state back into
Kion) or the `--clone-*` flags. Those are a different job from importing.

## The generated file is self-contained

`imports.tf` carries its own `terraform { required_providers { … } }` block, so
put it in an empty directory. Dropping it beside a configuration that already
declares its providers fails with:

```
Error: Duplicate required providers configuration
A module may have only one required providers configuration.
```

## Parent-scoped import ids

Resources whose read finds the record under a parent take a compound id, and
`kion-import` emits it for you:

```hcl
import {
  to = kion_project_enforcement.example
  id = "42/167"     # project_id/id
}
```

`kion_scope_criteria` is the same shape (`scope_id/criteria_id`). Importing one
of these with a bare id leaves the parent unset, and the read looks for the
record under parent `0`.
