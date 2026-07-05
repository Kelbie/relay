#!/bin/bash
# Runs the crawler and the relay in a single container.
# If either process exits, the container exits non-zero so the
# orchestrator (e.g. Railway) restarts the pair together.
set -u -o pipefail

# One-shot data wipe: set/change WIPE_DATA_TOKEN to erase the SQLite store on
# the next boot (pair with a Redis FLUSHALL for a virgin bootstrap — the two
# stores must be wiped together, see SELFHOST.md landmine 4). The token is
# recorded in /data/.wipe-token so restarts with the same value never re-wipe.
if [ -n "${WIPE_DATA_TOKEN:-}" ] && [ "$(cat /data/.wipe-token 2>/dev/null || true)" != "$WIPE_DATA_TOKEN" ]; then
  echo "[start.sh] WIPE_DATA_TOKEN changed - wiping /data sqlite store"
  rm -f /data/events.sqlite /data/events.sqlite-* /data/*.sqlite /data/*.sqlite-*
  printf '%s' "$WIPE_DATA_TOKEN" > /data/.wipe-token
fi

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
