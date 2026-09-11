# Running the acceptance tests

The provider has 90 acceptance tests across 61 service packages. They are the
only thing that exercises `Create`, `Update`, `Delete` and `ImportState`:
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

## Known gaps

- `make testacc` used to target `./internal/provider/...`, which contains no
  acceptance tests, so it exited 0 having run nothing. It now targets
  `./internal/service/...`.
- No environment in the current estate has been confirmed safe to write to, so
  as of this document **the tests have still never run**. The wiring is
  complete; the target is not chosen.
