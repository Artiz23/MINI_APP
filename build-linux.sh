#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
cd "$ROOT"
OUT="$HERE/dist/linux"
mkdir -p "$OUT"
export CGO_ENABLED=0 GOOS=linux GOARCH=amd64
go build -o "$OUT/miniapp" ./MINI_APP/cmd/miniapp
rm -rf "$OUT/web"
cp -R "$HERE/web" "$OUT/web"
echo "OK  $OUT/miniapp"
echo "Перед запуском задайте MINIAPP_BOT_TOKEN в окружении или systemd EnvironmentFile."
