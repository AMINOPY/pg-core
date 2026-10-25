#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEST="$ROOT/.agents/skills/android-upstream"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if ! command -v git >/dev/null 2>&1; then
  echo "error: git is required to install Android skills" >&2
  exit 1
fi

git clone --depth 1 https://github.com/android/skills.git "$TMP/upstream"
UPSTREAM="$TMP/upstream"

mapfile -d '' SKILL_FILES < <(find "$UPSTREAM" -type f -name SKILL.md -not -path "$UPSTREAM/.git/*" -print0 | sort -z)
if (("${#SKILL_FILES[@]}" == 0)); then
  echo "error: upstream checkout contains no SKILL.md files" >&2
  exit 1
fi

# This path is intentionally fixed and dedicated to managed upstream content.
# Never delete or replace any other agent skill directory.
rm -rf "$DEST"
mkdir -p "$DEST"

for file in "${SKILL_FILES[@]}"; do
  relative="${file#"$UPSTREAM"/}"
  skill_dir="${relative%/SKILL.md}"
  mkdir -p "$DEST/$(dirname "$skill_dir")"
  cp -R "$UPSTREAM/$skill_dir" "$DEST/$skill_dir"
done

if [[ -f "$UPSTREAM/LICENSE.txt" ]]; then
  cp "$UPSTREAM/LICENSE.txt" "$DEST/LICENSE.txt"
fi

printf 'Installed %s Android skills under %s\n' "${#SKILL_FILES[@]}" "${DEST#"$ROOT"/}"
printf 'Upstream commit: '
git -C "$UPSTREAM" rev-parse --short HEAD
