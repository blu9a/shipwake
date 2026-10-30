#!/usr/bin/env bash
# Real repositories at pinned commits, for benchmarks that build a site as a project would.
# Deterministic: only the pinned commit's history and the tags on it are kept, no remote state.
#   tools/fixtures.sh [dir]   (default .cache/fixtures)
set -euo pipefail
dir=${1:-.cache/fixtures}
mkdir -p "$dir"

# charmbracelet/log: Conventional Commits, version tags with notes; 255 commits.
fixture() {
  local name=$1 url=$2 sha=$3 trunk=$4
  local d="$dir/$name"
  # done only when the last step wrote its mark: a half-made fixture is made again
  if [ "$(cat "$d/.git/shipwake-fixture" 2>/dev/null)" = "$sha" ]; then return; fi
  rm -rf "$d"
  git clone -q --no-checkout "$url" "$d"
  git -C "$d" checkout -q -B "$trunk" "$sha"
  git -C "$d" remote remove origin
  git -C "$d" for-each-ref --format='%(refname)' refs/heads | { grep -v "^refs/heads/$trunk$" || true; } | xargs -r -n1 git -C "$d" update-ref -d
  git -C "$d" tag --no-merged "$sha" | xargs -r git -C "$d" tag -d >/dev/null
  echo "$sha" > "$d/.git/shipwake-fixture"
  echo "$name at ${sha:0:7}: $(git -C "$d" rev-list --count HEAD) commits, $(git -C "$d" tag | wc -l) tags"
}

fixture charmbracelet-log https://github.com/charmbracelet/log.git 394fd9b15aba02cf7dd118b345248855174a90a4 main
