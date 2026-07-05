#!/bin/bash
# Runs the crawler and the relay in a single container.
# If either process exits, the container exits non-zero so the
# orchestrator (e.g. Railway) restarts the pair together.
set -u -o pipefail

# Prefix each log line with the process name (line-buffered)
prefix() { while IFS= read -r line; do printf '[%s] %s\n' "$1" "$line"; done; }

/app/crawl 2>&1 | prefix crawl &
/app/relay 2>&1 | prefix relay &

# Wait for the first process to exit
wait -n
status=$?

echo "[start.sh] a process exited with status $status; shutting down" >&2

# Kill the whole process group (including this script), so the
# container exits and gets restarted.
kill 0
exit 1
