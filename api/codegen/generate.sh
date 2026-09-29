#!/usr/bin/env sh
# Regenerates all committed API code from api/openapi.yaml.
set -eu

root="$(cd "$(dirname "$0")/../.." && pwd)"

cd "$root/backend"
go tool oapi-codegen -config "$root/api/codegen/oapi-codegen.yaml" "$root/api/openapi.yaml"
