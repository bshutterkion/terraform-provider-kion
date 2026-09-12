#!/usr/bin/env bash
#
# Wipe every generated file under internal/service and rebuild it from codegen/,
# then report whether the result matches what was committed.
#
# The claim this checks is the provider's central one: nothing under
# internal/service is hand-maintained, so the whole tree can be thrown away and
# rebuilt from the codegen inputs. That claim was only ever verified by hand, and
# it is not self-evidently true -- two things break a naive attempt, and both are
# invisible during ordinary incremental work:
#
#   1. Bootstrap. `go run ./cmd/kgen` compiles the packages it is about to
#      write. 17 service packages have a GENERATED service_package.go, so wiping
#      deletes those directories outright, internal/provider stops compiling,
#      and the generator cannot run at all. kgen has to be built BEFORE the wipe.
#
#      The emptied directories must also be LEFT IN PLACE. Deleting them costs
#      12 packages: the generators enumerate what to write from the directories
#      present under internal/service, so a directory that is gone is a resource
#      that no longer exists as far as they are concerned.
#
#   2. Convergence. The generators feed each other: `kgen crud` needs the schema
#      models, and the schema tests plus some version gates read generated source
#      for a resource's constructor. One pass leaves 70 *_schema_gen_test.go
#      files and 2 version gates unwritten and 1 wrong -- silently, because the
#      tree still builds and the missing files are tests that no longer exist to
#      fail. Two passes reach a fixed point.
#
# Run from the repository root. Requires spec/openapi3.json (make refresh-spec),
# which is gitignored, so this cannot run in CI as-is.
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ ! -f spec/openapi3.json ]]; then
	echo "spec/openapi3.json is missing; run 'make refresh-spec' first" >&2
	exit 1
fi

if [[ -n "$(git status --porcelain internal/service)" ]]; then
	echo "internal/service has uncommitted changes; commit or stash them first" >&2
	echo "(this script wipes the tree and compares against HEAD)" >&2
	exit 1
fi

SDK_DIR="$(go list -m -f '{{.Dir}}' github.com/kionsoftware/kion-sdk-go)"
KGEN="$(mktemp -d)/kgen"

echo "==> building kgen before the wipe (it compiles what it generates)"
go build -o "$KGEN" ./cmd/kgen

echo "==> wiping generated files"
wiped=0
while IFS= read -r f; do
	if head -3 "$f" | grep -q 'Code generated'; then
		rm "$f"
		wiped=$((wiped + 1))
	fi
done < <(find internal/service -name '*.go')
echo "    removed $wiped generated files"

pass() {
	echo "==> pass $1"
	"$KGEN" versions --sdk "$SDK_DIR" --config codegen/generator_config.yaml \
		--overrides codegen/config_overrides.yaml >/dev/null
	"$KGEN" schemas --config codegen/generator_config.yaml --spec spec/openapi3.json \
		--renames codegen/renames.yaml --schema-overrides codegen/schema_overrides.yaml >/dev/null
	if [[ "$1" == 1 ]]; then
		"$KGEN" crud --config codegen/generator_config.yaml \
			--config-overrides codegen/config_overrides.yaml --sdk "$SDK_DIR" \
			--crud-overrides codegen/crud_archetypes.yaml --test-values codegen/test_values.yaml \
			--version-support codegen/version_support.yaml >/dev/null
	fi
}

pass 1
pass 2
"$KGEN" import-manifest >/dev/null

echo "==> comparing against HEAD"
if [[ -n "$(git status --porcelain internal/service)" ]]; then
	echo
	echo "REGENERATION DID NOT REPRODUCE THE COMMITTED TREE:"
	git status --porcelain internal/service
	echo
	echo "Either a file under internal/service is not actually generated, or a"
	echo "codegen input changed without the output being regenerated."
	exit 1
fi

echo "    internal/service reproduced exactly ($wiped files)"
echo "==> building"
go build ./...
echo "OK: the generated tree is fully reproducible from codegen/"
