# Self-hosting the Vertex relay + crawler

This fork ships a single Docker image (see [Dockerfile](Dockerfile)) that runs
both processes side by side via [start.sh](start.sh):

- `crawl` — the [crawler_v2](https://github.com/vertex-lab/crawler_v2) (pinned to
  the version in `go.mod`), which builds the follow graph in Redis and stores
  events in SQLite.
- `relay` — this repo's relay (`cmd/main.go`), which serves the Vertex DVMs and
  the Open Ranking HTTP API on top of that data.

If either process dies, the container exits so the platform (e.g. Railway)
restarts the pair together.

## Requirements

- **Redis 7.4+** reachable from the container. The graph database uses Redis
  functions and modern hash features, so older Redis versions won't work.
  Start with an **empty Redis DB** dedicated to this deployment: the crawler
  initializes the graph on first run, and pre-existing keys from anything else
  will conflict.
- **A persistent volume mounted at `/data`** for the shared SQLite database.
  Without it, events are lost on every deploy.

## Environment variables

Both processes read the same environment (godotenv/env-based config).

### Shared (used by both crawler and relay)

| Variable | Example | Notes |
|---|---|---|
| `REDIS_ADDRESS` | `redis.railway.internal:6379` | Redis 7.4+, empty DB. |
| `SQLITE_PATH` | `/data/events.sqlite` | **Set explicitly.** The crawler defaults to `events.sqlite` and the relay to `relay.sqlite`; both must point at the same file on the volume. |

### Crawler

| Variable | Example | Notes |
|---|---|---|
| `INIT_PUBKEYS` | `hex1,hex2,...` | Comma-separated **hex** pubkeys to seed the graph with. Pick a few well-connected accounts. |
| `FIREHOSE_KINDS` | `0,3,10000,10002` | Kinds ingested from the live firehose. Default is all kinds; trimming to profile/graph kinds keeps SQLite small when you only need ranking/search. |
| `POOL_AUTH_KEY` | 64-char hex secret key | Optional; used for NIP-42 auth to upstream relays (less rate limiting). |

### Relay

| Variable | Example | Notes |
|---|---|---|
| `RELAY_ADDRESS` | `0.0.0.0:3334` | Nostr relay + legacy API listen address. Default is `localhost:3334`, which is unreachable in a container — set it. |
| `RELAY_DOMAIN` | `vertex.example.com` | **Required.** Domain used for the NIP-42 auth URL; the relay panics at startup if empty. |
| `RELAY_SECRET_KEY` | 64-char hex | **Required.** Identity of the relay/DVM (signs responses). |
| `API_SECRET_KEY` | 64-char hex | **Required.** Identity used by the `/api/v1/dvms` HTTP endpoint. |
| `CREDITS_DISABLED` | `true` | **This fork's addition.** Skips credit deduction in `Service.Allow`, so all DVM/relay requests are allowed. Without it the upstream credit system applies. |
| `ADDRESS` | `0.0.0.0:8080` | Open Ranking HTTP server. Leave internal unless you want to expose it. |
| `RATE_UNKNOWN_INITIAL_TOKENS` / `RATE_UNKNOWN_MAX_TOKENS` / `RATE_UNKNOWN_TOKENS_PER_INTERVAL` / `RATE_UNKNOWN_INTERVAL` | `3000` / `3000` / `1000` / `1m` | Per-IP rate limiting for unknown clients (values shown are the defaults). `RATE_TRUSTED_*` / `RATE_UNTRUSTED_*` plus `RATE_TRUSTED_LIST` / `RATE_UNTRUSTED_LIST` (comma-separated IPs) work the same way. |

## Railway notes

- Expose port **3334** as the service's public port (the Nostr relay websocket
  and `/api/v1/dvms`).
- Mount a volume at **`/data`** and set `SQLITE_PATH=/data/events.sqlite`.
- Add a Redis 7.4+ service and point `REDIS_ADDRESS` at its private address.

## Bootstrap expectations

The graph starts empty. After seeding with `INIT_PUBKEYS`, the crawler walks
follow lists outward and the pagerank iterates as the graph grows. Expect
**1–2 days** of crawling before global/personalized pagerank and profile
search return useful results; follower counts and search quality keep
improving after that.

## Cold-start landmines (learned the hard way)

1. **`POOL_RELAYS` is REQUIRED.** It has no code default — the values in
   `.env.example` are suggestions, not fallbacks. Unset, the crawler's
   firehose subscribes to zero relays: everything logs "ready" and the only
   symptom is `Arbiter: total walks are non-positive total=0` forever.
2. **If the first boot ever ran without relays, flush Redis.** Init pubkeys
   are seeded only when the graph is empty (`nodes == 0`), and their fetch is
   enqueued exactly once. A broken first boot leaves a poisoned graph that no
   restart repairs — FLUSHALL and redeploy.
3. **Disable demotion while bootstrapping: `ARBITER_DEMOTION=0.99`.** With a
   handful of isolated seed nodes every rank sits at the baseline, so the
   default demotion threshold (1.05 × base) demotes all seeds on the first
   scan — which deletes their walks and freezes the graph at zero. Values
   <= 1 disable demotion (the arbiter warns accordingly); raise it back
   toward 1.05 once the graph has expanded past a few thousand nodes.
