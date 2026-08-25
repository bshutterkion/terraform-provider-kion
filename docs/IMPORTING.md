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
terraform apply
```

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
