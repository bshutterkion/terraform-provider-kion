# Running the acceptance tests

The provider has acceptance tests across 47 of its 72 service packages. They are
the only thing that exercises `Create`, `Update`, `Delete` and `ImportState`:
everything in `ci.yml` is compile-, lint- or read-level, and `acctest-config`
only checks that the test HCL matches the schema without applying any of it.

Until these run, **408 CRUD entry points have never executed against a real
API**.

## They create and destroy real records

Use a disposable installation. A local one is ideal: the tests create
`test-acc-` prefixed resources, and `make sweep` reclaims anything a cancelled
run leaves behind.

Do not point them at anything shared.

## Local run

```sh
export KION_API_URL="http://localhost:8081"
export KION_API_KEY="app_1_…"
export KION_APIPATH=            # set-but-empty: the API is at the root, not /api

python3 scripts/discover-acc-env.py "$KION_API_URL" "$KION_API_KEY" ""   # prints the KION_ACC_* block
# ...paste what it prints, then:

make testacc
make sweep                       # always, including after a cancelled run
```

`KION_APIPATH` matters for a local install. A hosted Kion serves its API under
`/api`, which is the default; an app reached directly on localhost serves it at
the root. The tests write no provider block, so this environment variable is the
only way to tell them. **Set-but-empty means the root**; leaving it unset keeps
`/api`.

## The install-specific ids

Ten `KION_ACC_*` variables name records the tests cannot invent — a billing
source, an AWS account number, a SAML IDMS. `scripts/discover-acc-env.py` reads
them from a live install and prints an env block, naming anything that install
cannot supply.

**A test that skips for a missing id has not passed.** It reports `SKIP` with
the variable's name. A run that is mostly skips has verified almost nothing, so
read the counts rather than the exit code.

`KION_ACC_APP_CONFIG_MUTATION_OK` is opt-in rather than discovered: app config
is global state, so mutating it affects everyone on the install.

## In CI

`.github/workflows/acctest.yml` runs them, on `workflow_dispatch` only. It is
deliberately not on push or pull_request: that would mutate an installation on
every commit, and a fork PR would do it with someone else's configuration.

Credentials come from a GitHub Environment, chosen per run. Put reviewers on
that Environment and a run against a protected install needs approval before
anything is written. The workflow sweeps before and after, including on
cancellation.

## Tests that are red on purpose

A generated test file can carry a `Known issues this test is expected to
surface:` header naming an open defect. That comes from `KnownIssues` in
`internal/kgen/tests/resource_meta.go`, and it means the red is a recorded
finding rather than an unexplained failure — do not "fix" such a test by
loosening its assertions. As of this document:

| package | issue |
|---|---|
| `funding_source` | #68 `owner_user_ids` / `permission_scheme_id` dropped on read |
| `budget` | #69 `amount` dropped on read |
| `ou_enforcement`, `funding_source_enforcement` | #70 `cloud_rule_id` dropped on read |
| `funding_source_note` | #71 every note is created with id 0 |

## Writing a new one

Prefer generating: add an entry to the registry in
`internal/kgen/tests/resource_meta.go` and run
`go run -tags kgendocs ./cmd/kgen tests --force --resource kion_<name>`.
Beyond the SDK get/delete methods and per-field values, the registry carries:

- `Dependencies` — resources the config stands up first. A dependency's
  `TargetField` is emitted as a reference even when the field is Required.
- `RequiredEnv` — a `KION_ACC_*` variable the test cannot run without, with the
  reason. Emits a skip guard in every test function, including the data
  source's. **Guards only**: the value is not threaded into the HCL.
- `ImportIDParentField` — for the `parent_list` and `association` archetypes,
  whose `ImportState` parses `"<parent>/<id>"` rather than a bare id.
- `NoUpdate` — suppresses the `_update` test for a resource whose `Update`
  answers "cannot be updated in place".
- `KnownIssues` — the header described above.

Hand-write instead when the config must interpolate an install-specific id
(`internal/service/billing_rule` and `internal/service/idms_group_association`
are the precedents). `kgen tests` skips existing files unless given `--force`,
so a hand-written test survives regeneration.

JSON-valued attributes must be written through `jsonencode(...)`, not as a raw
string literal. Reads canonicalise JSON to the compact, key-sorted form
`jsonencode` emits (see `codegen/schema_overrides.yaml`), so a hand-ordered
literal comes back reordered and the apply fails with "Provider produced
inconsistent result after apply".

## Known gaps

- `make testacc` used to target `./internal/provider/...`, which contains no
  acceptance tests, so it exited 0 having run nothing. It now targets
  `./internal/service/...`.
- The five `billing_source*` packages still have no acceptance test. Every one
  of them needs real cloud credentials, and no install available so far has a
  billing source of any kind (`/v4/billing-source` and `/v1/payer` both return
  empty, and `discover-acc-env.py` reports `KION_ACC_BILLING_SOURCE_ID`,
  `KION_ACC_PAYER_ID` and `KION_ACC_AZURE_PAYER_ID` all unavailable). Tests that
  could only ever be observed skipping would repeat the pattern #63 exists to
  stop, so they were deliberately not written.
- `POST /v3/azure-role` returns 500 for every payload on an install with no
  Azure billing source — raw `curl` reproduces it, so it is the API rather than
  the provider. `kion_azure_role`'s test gates on `KION_ACC_AZURE_PAYER_ID`.
