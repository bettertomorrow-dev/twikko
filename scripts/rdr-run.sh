#!/bin/sh
# Run one example session, preserving its output outside runner retention.
set -eu
binary=$1
port=$2
mode=$3
case "$mode" in
  cards) example=tasks ;;
  table) example=orders ;;
  *) echo "Unsupported editor mode: $mode" >&2; exit 1 ;;
esac
umask 077
mkdir -p .rdr-output
session_dir=$(mktemp -d ".rdr-output/$mode.XXXXXX")
output="$PWD/$session_dir/result.yaml"
printf 'twikko: output=%s\n' "$output"
exec "$binary" --view "examples/$example.view.yaml" \
  --data "examples/$example.csv" --mode "$mode" \
  --output "$output" --port "$port"
