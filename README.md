# Pulse

A real-time discovery engine that surfaces emerging communities based on live momentum signals.

## What is this?

Pulse detects **what's gaining attention right now** — not what's already popular. It's built around a simple idea: the most interesting communities are often the ones *just starting to spike*, not the ones that are already large.

Think of it like a seismograph for community activity. When a community suddenly gets more engagement than usual, Pulse notices and ranks it higher.

## Why build this?

Most discovery systems favor established communities because they rely on cumulative metrics (total followers, all-time views). This creates a "rich get richer" problem where new communities struggle to surface.

Pulse solves this by focusing on **rate of change** instead of absolute size. A small community with a sudden burst of activity will rank higher than a large community with steady but flat engagement.

## How it works

```
User Activity → Events → Momentum Calculation → Ranked Feed
     ↓              ↓              ↓                 ↓
  (joins,       (weighted,     (sliding         (sorted by
   posts,        queued,        window,          momentum,
   views)        batched)       decay)           cached)
```

1. **Events come in** — Users interact with communities (join, post, comment, react, share)
2. **Events are weighted** — A "join" has more impact than a "view"
3. **Momentum is calculated** — Sum of weighted events in a sliding time window (default: 1 hour)
4. **Rankings are cached** — Redis sorted set for sub-millisecond reads
5. **Feed is served** — Communities ordered by current momentum

## Stack

| Layer | Tech | Why |
|-------|------|-----|
| API | Go + Echo | Fast, typed, simple |
| Database | PostgreSQL | Reliable, great for time-series queries |
| Cache | Redis | Sorted sets are perfect for leaderboards |
| Auth | Supabase JWT | Handles auth so we don't have to |

## Quick Start

```bash
# clone
git clone https://github.com/joacominatel/pulse.git
cd pulse

# copy env and fill in your values
cp .env.example .env.docker

# run with docker
docker compose up -d

# check health
curl http://localhost:8080/health
```

## API

### Ingest an event
```bash
curl -X POST http://localhost:8080/api/v1/events \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "community_id": "uuid-here",
    "event_type": "join"
  }'
```

Event types: `view`, `join`, `leave`, `post`, `comment`, `reaction`, `share`

### Get trending communities
```bash
curl http://localhost:8080/api/v1/communities?limit=20 \
  -H "Authorization: Bearer <token>"
```

Returns communities sorted by momentum (highest first).

### Trigger momentum recalculation
```bash
curl -X POST http://localhost:8080/api/v1/momentum/calculate \
  -H "Authorization: Bearer <token>"
```

## Architecture Decisions

**Why async event ingestion?**  
Events are queued in a buffered channel and batch-inserted. This handles traffic spikes without overwhelming the database.

**Why Redis for rankings?**  
Sorted sets give O(log N) inserts and O(1) rank lookups. The leaderboard stays fast regardless of community count.

**Why in-memory community cache?**  
Every event needs to verify the community exists. Caching this avoids a database round-trip on every request.

**Why sliding window, not all-time?**  
Momentum should reflect *current* activity. Events older than the window (default 1 hour) don't count.

## Project Structure

```
pulse/
├── cmd/pulse/          # application entrypoint
├── internal/
│   ├── domain/         # business logic, no dependencies
│   ├── application/    # use cases (ingest, calculate, etc)
│   └── infrastructure/ # database, cache, http, workers
├── scripts/            # utilities (load testing, noise generator)
└── docker-compose.yml  # local dev stack
```

## Environment Variables

```bash
# required
DB_HOST=localhost
DB_PORT=5432
DB_USER=pulse
DB_PASSWORD=secret
DB_NAME=pulse
SUPABASE_JWT_SECRET=your-jwt-secret

# optional
REDIS_URL=redis://localhost:6379/0  # enables caching
DB_SSL_MODE=disable                  # for local dev
DB_SCHEMA=pulse
```

## Performance

Tested with 500 concurrent users:

| Scenario | Requests | Error Rate | Throughput |
|----------|----------|------------|------------|
| Event ingestion | 5,225 | 0% | 348 req/s |
| Feed discovery | 5,636 | 0% | 376 req/s |

The async buffer handles bursts of 10,000 events before applying backpressure.

## What this is NOT

- A social network
- A content hosting platform  
- A recommendation engine based on user history
- A production-ready system (it's a learning project)

## License

MIT
