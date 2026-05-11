#!/usr/bin/env bash
# release.sh — cut a new lazytorrent release.
#
# Validates the working state, runs tests, tags the commit, and pushes
# both main and the tag. The pushed tag triggers .github/workflows/release.yml,
# which builds cross-platform binaries via goreleaser and publishes a GitHub
# release.

set -euo pipefail

usage() {
  cat <<EOF
Usage: scripts/release.sh <version> [options]

Cuts a new release of lazytorrent.

Arguments:
  <version>            Required. Format: vMAJOR.MINOR.PATCH (optionally -prerelease)

Options:
  -m, --message TEXT   Annotated tag message (defaults to the version string)
  --dry-run            Print the plan and exit without tagging or pushing
  -h, --help           Show this help

Examples:
  scripts/release.sh v0.2.0
  scripts/release.sh v0.2.0 -m "filter rewrite, dark mode"
  scripts/release.sh v0.2.0 --dry-run
EOF
  exit "${1:-0}"
}

version=""
message=""
dry_run=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage 0 ;;
    --dry-run) dry_run=true; shift ;;
    -m|--message)
      [[ $# -ge 2 ]] || { echo "error: -m requires a value" >&2; exit 1; }
      message="$2"; shift 2 ;;
    v*)
      [[ -z "$version" ]] || { echo "error: multiple versions provided: $version, $1" >&2; exit 1; }
      version="$1"; shift ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage 1 ;;
  esac
done

[[ -n "$version" ]] || { echo "error: version is required (e.g., v0.2.0)" >&2; usage 1; }

if ! [[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9._-]+)?$ ]]; then
  echo "error: version must look like v0.1.0 or v0.1.0-rc1, got: $version" >&2
  exit 1
fi

cd "$(git rev-parse --show-toplevel)"

if ! git remote get-url origin >/dev/null 2>&1; then
  cat >&2 <<'EOF'
error: no 'origin' remote configured.

Configure one first, e.g.:
  gh repo create hansbala/lazytorrent --public --source=. --remote=origin --push
or:
  git remote add origin git@github.com:hansbala/lazytorrent.git
EOF
  exit 1
fi

branch=$(git symbolic-ref --short HEAD)
if [[ "$branch" != "main" ]]; then
  echo "error: must be on 'main' branch (currently on: $branch)" >&2
  exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "error: working tree is dirty; commit or stash changes first" >&2
  git status --short
  exit 1
fi

if git rev-parse -q --verify "refs/tags/$version" >/dev/null; then
  echo "error: tag $version already exists locally" >&2
  exit 1
fi

if git ls-remote --tags origin "refs/tags/$version" | grep -q .; then
  echo "error: tag $version already exists on remote" >&2
  exit 1
fi

echo "→ running tests..."
go test ./...

[[ -n "$message" ]] || message="$version"

echo
echo "Release plan:"
echo "  version:      $version"
echo "  commit:       $(git rev-parse --short HEAD) — $(git log -1 --format=%s)"
echo "  tag message:  $message"
echo "  remote:       $(git remote get-url origin)"
echo

if $dry_run; then
  echo "(dry run — not tagging or pushing)"
  exit 0
fi

read -r -p "Continue? [y/N] " ans
case "$ans" in
  y|Y|yes|YES) ;;
  *) echo "aborted"; exit 0 ;;
esac

git tag -a "$version" -m "$message"
echo "→ tagged $version"

git push origin main
git push origin "$version"
echo
echo "✓ pushed $version to origin"

# Print useful URLs based on the origin remote, if it's on GitHub.
remote_url=$(git remote get-url origin)
repo=""
if [[ "$remote_url" =~ github\.com[:/]([^[:space:]]+) ]]; then
  repo="${BASH_REMATCH[1]%.git}"
fi
if [[ -n "$repo" ]]; then
  echo
  echo "Watch the build:    https://github.com/$repo/actions"
  echo "Release page (when build finishes):"
  echo "                    https://github.com/$repo/releases/tag/$version"
fi
