#!/usr/bin/env bash
# Fail if anything internal has crept into the tree.
#
# This repository is public and accepts outside contributions. Internal
# references — backend source paths, internal hostnames, internal repo/group
# names, AI planning notes — are not secrets, but they leak implementation
# detail nobody outside can act on, and once published they live in every fork.
# A reviewer cannot reliably catch them by eye, so this runs in CI.
#
# Adding a term here is cheap. Removing one should be deliberate.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# Patterns are extended regex, matched against TRACKED files only — untracked
# working files (local notes, scratch dirs) are the contributor's business.
patterns=(
  'kion/portal'                 # backend source tree
  '[[:space:]]portal[[:space:]]' # prose references to the backend by name
  'src/(app|domain)/'           # backend source paths
  'git\.kion\.io'               # internal GitLab
  'delivery-support'            # internal group path
  'kion-sdk-monorepo'           # internal monorepo name
  'docs/superpowers'            # AI planning notes (gitignored; catch re-adds)
                                # NOTE: .gitignore must name this path, so that
                                # one file is excluded for this pattern below.
  'PHASE2-SDK-EXPOSURE'         # internal roadmap doc
)

fail=0
for p in "${patterns[@]}"; do
  # `git grep -I` skips binaries. Exclude this script, which necessarily
  # contains every pattern it looks for.
  # .gitignore legitimately names docs/superpowers — that rule is what keeps the
  # notes out. Exclude it for that pattern only, not for every pattern.
  extra=()
  if [ "$p" = 'docs/superpowers' ]; then extra=(':!.gitignore'); fi
  if hits=$(git grep -nIE "$p" -- . ':!scripts/check-no-internal-refs.sh' "${extra[@]}" 2>/dev/null); then
    echo "✗ internal reference matched /$p/:" >&2
    printf '%s\n' "$hits" | head -20 | sed 's/^/    /' >&2
    fail=1
  fi
done

# The greps above match file CONTENTS. A planning doc re-added under
# docs/superpowers/ contains no such text, so it would sail through — check the
# tracked path list separately.
forbidden_paths='^docs/superpowers/|(^|/)PHASE2-SDK-EXPOSURE\.md$|^\.gitlab-ci\.yml$'
if paths=$(git ls-files | grep -E "$forbidden_paths"); then
  echo "✗ internal files are tracked:" >&2
  printf '%s\n' "$paths" | sed 's/^/    /' >&2
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  cat >&2 <<'EOF'

This repo is public. Remove the reference, or describe the behaviour without
citing internal sources — "the API returns X" rather than a backend file path.
If a term above is now legitimately public, drop it from `patterns` in
scripts/check-no-internal-refs.sh in the same change, so the reason is reviewable.
EOF
  exit 1
fi
echo "✓ no internal references in tracked files"
