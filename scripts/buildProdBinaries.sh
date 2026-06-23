#!/usr/bin/env bash
set -euo pipefail

# Run everything relative to the repo root regardless of where the script is invoked from.
cd "$(dirname "${BASH_SOURCE[0]}")/.."

# Production builds read their secret values from a git-ignored env file. Copy
# cli-telemetry.env.example to cli-telemetry.env and fill in the real values. This keeps the
# telemetry connection string out of the repo and out of shell history, while guaranteeing a release
# is never built without it.
ENV_FILE="cli-telemetry.env"
if [ ! -f "$ENV_FILE" ]; then
  echo "ERROR: $ENV_FILE not found." >&2
  echo "Copy cli-telemetry.env.example to $ENV_FILE and fill in the values." >&2
  exit 1
fi

# Load the file's KEY="value" lines into the environment. Values must be quoted in the file because
# the connection string contains ';'.
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

: "${REVDEP_TELEMETRY_CONNECTION_STRING:?must be set in $ENV_FILE}"

CONN="$REVDEP_TELEMETRY_CONNECTION_STRING"
# -s -w strips debug info; -X injects the connection string at link time. NOTE: a typo in the -X
# import path is silently ignored by the linker, so the baked-in value is verified after the build.
LDFLAGS="-s -w -X rev-dep-go/internal/telemetry.connectionString=${CONN}"

GOOS=darwin GOARCH=arm64 go build -o ./npm/@rev-dep/darwin-arm64/bin/rev-dep -ldflags="$LDFLAGS" ./cmd/cli
GOOS=linux GOARCH=amd64 go build -o ./npm/@rev-dep/linux-x64/bin/rev-dep -ldflags="$LDFLAGS" ./cmd/cli
GOOS=windows GOARCH=amd64 go build -o ./npm/@rev-dep/win32-x64/bin/rev-dep.exe -ldflags="$LDFLAGS" ./cmd/cli

# Post-build verification: prove the connection string actually landed in the binary. The same
# LDFLAGS apply to all three builds, so verifying the binary that runs natively on this host is
# enough.
HOST_BIN="./npm/@rev-dep/darwin-arm64/bin/rev-dep"
if [ "$(uname -s)" = "Linux" ]; then
  HOST_BIN="./npm/@rev-dep/linux-x64/bin/rev-dep"
fi

if ! "$HOST_BIN" __telemetry --check; then
  echo "ERROR: telemetry connection string was not baked into the binary - check the -X import path in LDFLAGS." >&2
  exit 1
fi

echo "Production binaries built; telemetry connection string verified."
