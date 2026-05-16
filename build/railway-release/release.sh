#!/usr/bin/env bash
set -euo pipefail

: "${GITHUB_TOKEN:?GITHUB_TOKEN is required}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required, e.g. owner/repo}"

if [[ -z "${RELEASE_TAG:-}" ]]; then
  RELEASE_TAG="$(git describe --tags --exact-match 2>/dev/null || true)"
fi

if [[ -z "$RELEASE_TAG" ]]; then
  echo "RELEASE_TAG is required when the deployment commit is not exactly tagged" >&2
  exit 1
fi

mkdir -p dist/windows

(
  cd client
  GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
    go build -trimpath -ldflags="-s -w -H=windowsgui" -o ../dist/windows/Turbo.exe .
)

(
  cd dist/windows
  zip -qry ../Turbo-windows-amd64.zip Turbo.exe
)

sha256sum dist/Turbo-windows-amd64.zip dist/windows/Turbo.exe > dist/checksums.txt

if gh release view "$RELEASE_TAG" --repo "$GITHUB_REPOSITORY" >/dev/null 2>&1; then
  gh release upload "$RELEASE_TAG" \
    dist/Turbo-windows-amd64.zip \
    dist/windows/Turbo.exe \
    dist/checksums.txt \
    --repo "$GITHUB_REPOSITORY" \
    --clobber
else
  gh release create "$RELEASE_TAG" \
    dist/Turbo-windows-amd64.zip \
    dist/windows/Turbo.exe \
    dist/checksums.txt \
    --repo "$GITHUB_REPOSITORY" \
    --title "$RELEASE_TAG" \
    --notes "Turbo desktop release $RELEASE_TAG" \
    --latest
fi

gh release view "$RELEASE_TAG" --repo "$GITHUB_REPOSITORY" --json tagName,url,assets
