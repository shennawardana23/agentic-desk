---
name: backend-engineering
description: Guides backend API design and implementation in Go and PostgreSQL. Use when designing REST or RPC endpoints, writing database migrations, handling errors, or structuring the testing pyramid for a Go service. Use when a partitioned PostgreSQL table needs query filtering, when choosing between synchronous and asynchronous handlers, or when reviewing backend code for idiomatic Go patterns.
category: Engineering
---

# Backend Engineering

## Overview

Backend engineering here means building HTTP/RPC services in Go against PostgreSQL that are correct under concurrency, safe to deploy, and cheap to operate. This skill covers API design, error handling, database migrations, partition-aware querying, and a testing pyramid appropriate for services that other teams depend on.

## When to Use

- Designing a new Go service or adding endpoints to an existing one
- Writing or reviewing PostgreSQL migrations
- Querying tables partitioned by `hotel_id` or similar tenant key
- Deciding how to propagate and handle errors across package boundaries
- Structuring unit, integration, and contract tests for a backend change
- Reviewing a pull request for idiomatic Go and safe SQL

## Workflow

### 1. Define the contract before writing handlers

Write the request/response types and the OpenAPI or protobuf contract first. Treat it as the spec.

```go
type CreateReservationRequest struct {
    HotelID    int64     `json:"hotel_id"`
    GuestID    int64     `json:"guest_id"`
    CheckIn    time.Time `json:"check_in"`
    CheckOut   time.Time `json:"check_out"`
    RoomTypeID int64     `json:"room_type_id"`
}

type Reservation struct {
    ID         int64     `json:"id"`
    HotelID    int64     `json:"hotel_id"`
    Status     string    `json:"status"`
    CreatedAt  time.Time `json:"created_at"`
}
```

Validate at the boundary (HTTP handler), never deep in the call stack. Once past the handler, trust the types.

### 2. Structure errors so callers can act on them

Use sentinel errors or typed errors wrapped with `fmt.Errorf("%w", ...)`, never bare strings compared with `==`.

```go
var ErrReservationNotFound = errors.New("reservation not found")
var ErrRoomUnavailable = errors.New("room unavailable for requested dates")

func (s *Service) GetReservation(ctx context.Context, hotelID, id int64) (*Reservation, error) {
    r, err := s.repo.FindByID(ctx, hotelID, id)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, fmt.Errorf("get reservation %d: %w", id, ErrReservationNotFound)
    }
    if err != nil {
        return nil, fmt.Errorf("get reservation %d: %w", id, err)
    }
    return r, nil
}
```

Map errors to HTTP status once, at the outermost handler layer, using `errors.Is`/`errors.As` — not string matching.

### 3. Always filter partitioned tables explicitly

Tables partitioned by `hotel_id` require the partition key in every query's `WHERE` clause, or PostgreSQL scans every partition. This is the single most common source of unnecessary full-table scans in this codebase.

```sql
-- Bad: scans every hotel partition
SELECT * FROM reservations WHERE guest_id = $1;

-- Good: PostgreSQL can prune to one partition
SELECT * FROM reservations WHERE hotel_id = $1 AND guest_id = $2;
```

Enforce this at the repository layer: every repository method that touches a partitioned table takes `hotelID` as its first parameter, not an optional filter.

### 4. Write migrations that are safe to roll forward and back

```sql
-- migrations/0042_add_reservation_source.up.sql
ALTER TABLE reservations ADD COLUMN source TEXT NOT NULL DEFAULT 'direct';

-- migrations/0042_add_reservation_source.down.sql
ALTER TABLE reservations DROP COLUMN source;
```

Rules:
- Never rewrite history on a migration that has already run in any shared environment — add a new one instead.
- Add columns as `NOT NULL DEFAULT ...` in one step only on small tables; on large partitioned tables, add nullable, backfill in batches, then add the constraint in a follow-up migration to avoid long locks.
- Every new index on a large table uses `CREATE INDEX CONCURRENTLY` in its own migration (cannot run inside a transaction — configure the migration tool accordingly).
- Test the `down` migration in CI, not just the `up`.

### 5. Choose the right concurrency shape

- Use `context.Context` as the first parameter of every function that does I/O, and respect cancellation.
- Use `errgroup.Group` for fan-out calls that must all succeed.
- Use buffered channels or worker pools for bounded concurrency against external services; never spawn unbounded goroutines per request.
- Guard shared mutable state with `sync.Mutex` or prefer channels; run `go test -race` in CI on every package that touches concurrency.

### 6. Build the testing pyramid

| Layer | Scope | Tooling | Target volume |
|---|---|---|---|
| Unit | Pure functions, business logic, no I/O | `testing` + table-driven tests | Majority of tests |
| Repository/integration | Real PostgreSQL via `testcontainers-go` or a test schema | `testing` + `sqlx`/`pgx` | Moderate — one per query path |
| Contract | HTTP handler in/out shape matches the documented API | `httptest` + golden files | One per endpoint |
| End-to-end | Full service against a seeded database | Separate suite, run pre-deploy | Small — critical user journeys only |

Table-driven unit test pattern:

```go
func TestCalculateNightlyRate(t *testing.T) {
    tests := []struct {
        name     string
        base     int64
        nights   int
        discount float64
        want     int64
    }{
        {"no discount", 10000, 3, 0, 30000},
        {"ten percent off", 10000, 3, 0.10, 27000},
        {"zero nights", 10000, 0, 0, 0},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := CalculateNightlyRate(tt.base, tt.nights, tt.discount)
            if got != tt.want {
                t.Errorf("got %d, want %d", got, tt.want)
            }
        })
    }
}
```

## Checklist

- [ ] Request/response types and error cases defined before handler code
- [ ] Every query against a partitioned table filters explicitly on `hotel_id` (or the partition key)
- [ ] Errors wrapped with context and checked via `errors.Is`/`errors.As`, not string comparison
- [ ] Migration has a tested, reversible `down` script
- [ ] Long-running index builds use `CREATE INDEX CONCURRENTLY`
- [ ] `go vet` and `go test -race` pass on packages touching concurrency or shared state
- [ ] Unit tests cover business logic; integration tests cover at least one path per query; contract tests cover the endpoint shape

## Common Rationalizations

| Rationalization | Reality |
|---|---|
| "It's a small query, partition pruning doesn't matter" | It scans every partition regardless of row count returned — cost is proportional to partition count. |
| "We'll add the down migration if we ever need it" | Untested down migrations fail exactly when you need them most, mid-incident. |
| "String error matching is fine, we control both sides" | It silently breaks the first time the message copy changes for a UI reason. |
| "Integration tests are slow, unit tests are enough" | Unit tests can't catch a wrong SQL join or a missing index — that's what breaks in production. |

## Red Flags

- Repository methods that accept an optional `hotelID *int64`
- `SELECT *` in application code (couples code to column order, breaks on migrations)
- Goroutines started without a bound, a context, or an `errgroup`
- Migrations that modify a file already merged to main instead of adding a new one
- Handlers that return raw `err.Error()` text to API clients

## Verification

- [ ] `go build ./...` and `go vet ./...` pass
- [ ] `EXPLAIN ANALYZE` confirms partition pruning on new queries against partitioned tables
- [ ] Migration applied and rolled back successfully in a scratch database
- [ ] New endpoint has unit, integration, and contract test coverage
- [ ] No secrets or connection strings committed alongside the change
