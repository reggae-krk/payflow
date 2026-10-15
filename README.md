# PayFlow

PayFlow demonstrates REST API design, atomic SQL transactions, financial data integrity
(idempotency, lock ordering, no negative balances), unit and integration testing, and a small
gRPC service alongside the main HTTP API — without over-engineering with technologies that
aren't justified at this stage.

## Architecture

PayFlow is a **modular monolith**, not microservices. All core domain logic (auth, users,
wallet) lives in a single HTTP API process. The one exception is the **Reporting Service**, a
small, independent gRPC service that reads from the same PostgreSQL database (read-only) but
runs as its own process — added deliberately as a single, well-tested extension point rather
than splitting the whole system into microservices.

```
Client (curl/Postman)
    |  HTTP/JSON
    v
+-------------------------------------------------+
|               PayFlow API (Gin) :8080            |
|                                                   |
|  auth module     users module      wallet module |
|  - JWT           - registration    - account      |
|  - middleware     - login          - deposit       |
|                                     - withdraw      |
|                                     - transfer      |
|                                     - history       |
|                                                   |
|  handler -> service -> repository                |
+---------------------+-----------------------------+
                       |  pgx + SQL
                       v
                  PostgreSQL
                       ^
                       |  pgx + SQL (read-only)
+---------------------+-----------------------------+
|         Reporting Service (gRPC) :50051           |
|  GetAccountSummary -> aggregated account stats    |
+-----------------------------------------------------+
```

## Stack

| Layer | Technology |
|---|---|
| Language | Go 1.26+ |
| Web framework | Gin |
| Database | PostgreSQL |
| DB driver | pgx (raw SQL, no ORM) |
| Migrations | golang-migrate |
| Auth | JWT (golang-jwt) + bcrypt |
| Inter-service RPC | gRPC + Protocol Buffers |
| Containerization | Docker + Docker Compose |
| Testing | standard `testing` + testcontainers-go |

## Data model (simplified ledger)

- **users**: `id`, `email`, `password_hash`, `created_at`
- **accounts**: `id`, `user_id` (FK), `currency`, `balance_minor` (BIGINT — always minor units, never float), `created_at`
- **transfers**: `id`, `idempotency_key` (unique), `source_account_id`, `destination_account_id`, `amount_minor`, `status`, `created_at`
- **ledger_entries**: `id`, `transfer_id` (nullable; `NULL` for deposits and withdrawals), `account_id`, `operation_type` (deposit/withdrawal/transfer), `entry_type` (credit/debit), `amount_minor`, `created_at`

### Key domain rules

1. Monetary amounts are always stored as `BIGINT` in minor units (e.g. grosze/cents) — never as floating point.
2. An account balance can never drop below zero.
3. A transfer is a single atomic SQL transaction: debit sender, credit recipient, one `transfers` row, two `ledger_entries` rows.
4. An `X-Idempotency-Key` header prevents a retried request from executing the same transfer twice.
5. Transfer locks are acquired in a fixed order (ascending account ID) to prevent deadlocks on concurrent transfers.

## Running the project

### Prerequisites

- Go 1.26+
- Docker and Docker Compose
- `golang-migrate` CLI
- `grpcurl` (optional, for testing the gRPC service manually)

### Quickstart with Docker Compose

The entire stack (PostgreSQL, PayFlow HTTP API, and Reporting gRPC Service) can be started with a single command:

```bash
docker compose up --build
```

This command automatically:
1. Starts PostgreSQL and initializes the database schema (`/docker-entrypoint-initdb.d`).
2. Builds and launches the HTTP API (`:8080`).
3. Builds and launches the Reporting Service (`:50051`).

To verify API health:
```bash
curl http://localhost:8080/health
# Response: {"status":"ok"}
```

### Manual setup (Local development)

### 1. Start the database

```bash
docker compose up -d postgres
```

This starts only the PostgreSQL 16 container (healthcheck + persistent volume). Plain
`docker compose up -d` would also start the API and the Reporting Service in Docker, and their
ports would clash with the `go run` processes below.

### 2. Migrations

On the first start of an empty volume, Postgres applies every `migrations/*.up.sql` file itself
(they are mounted into `/docker-entrypoint-initdb.d` in `docker-compose.yml`), so no extra step
is needed. Don't run `migrate up` against this database afterwards: it doesn't know the tables
were already created and fails with `relation "users" already exists`.

`golang-migrate` is for a database that was **not** created by Docker Compose:

```bash
migrate -path migrations -database "postgres://payflow:payflow_dev_password@localhost:5432/payflow?sslmode=disable" up
```

### 3. Set required environment variables

The API refuses to start without a JWT secret and the three request timeouts (see
[Configuration](#configuration)); the Reporting Service only needs the database settings, which
all have local defaults.

```bash
export JWT_SECRET=your-local-dev-secret
export HEALTH_TIMEOUT=2s LOGREG_TIMEOUT=3s REQUEST_TIMEOUT=5s
```

### 4. Run the main API

```bash
go run ./cmd/api
```

The API listens on `:8080`.

### 5. Run the Reporting Service (optional, separate process)

```bash
go run ./proto/reporting
```

The gRPC server listens on `:50051`. See [Reporting Service](#reporting-service-grpc) below for details.

## Configuration

All settings come from environment variables and are validated at startup. A missing,
malformed or inconsistent value makes the process exit with an error that lists **every**
problem at once; it never starts half-configured.

| Variable | Used by | Required | Default | Notes |
|---|---|---|---|---|
| `JWT_SECRET` | API | yes | — | HMAC key for signing JWTs |
| `HEALTH_TIMEOUT` | API | yes | — | Deadline for `GET /health`, e.g. `2s` |
| `LOGREG_TIMEOUT` | API | yes | — | Deadline for `/register` and `/login` |
| `REQUEST_TIMEOUT` | API | yes | — | Deadline for all authenticated `/accounts/...` endpoints |
| `DB_HOST` | both | no | `localhost` | |
| `DB_PORT` | both | no | `5432` | |
| `DB_USER` | both | no | `payflow` | |
| `DB_PASSWORD` | both | no | `payflow_dev_password` | |
| `DB_NAME` | both | no | `payflow` | |
| `DB_MAX_CONNS` | both | no | `max(4, CPUs)` | pgxpool default; must be > 0 |
| `DB_MIN_CONNS` | both | no | `0` | pgxpool default; must be <= `DB_MAX_CONNS` |

Timeouts are Go durations (`500ms`, `3s`, `1m`) and must be > 0. `docker-compose.yml` sets
`HEALTH_TIMEOUT=2s`, `LOGREG_TIMEOUT=3s`, `REQUEST_TIMEOUT=5s` for the API container.

## Timeouts and graceful shutdown

- **Request deadlines**: a middleware wraps each request's context with the timeout of its route
  group. Handlers pass `c.Request.Context()` down to pgx, so when the deadline passes or the
  client disconnects, the running query is cancelled and the transaction is rolled back. An
  integration test (`TestServiceTransferRollsBackOnDeadline`) checks that a transfer interrupted
  after both balance updates leaves balances, transfers and ledger entries unchanged.
- **Startup check**: every request timeout + the 5 s rollback timeout (`db.RollbackTimeout`) must
  be shorter than the HTTP `WriteTimeout` (15 s) and the shutdown timeout (25 s).
- **HTTP server limits**: `ReadHeaderTimeout` 5 s, `ReadTimeout` 5 s, `WriteTimeout` 15 s,
  `IdleTimeout` 30 s (idle keep-alive connections).
- **Fail fast**: both services ping the database at startup (5 s) and exit if it is unreachable.
- **Graceful shutdown**: on `SIGINT`/`SIGTERM` the API stops accepting connections and waits up to
  25 s for in-flight requests (`http.Server.Shutdown`). The Reporting Service uses
  `grpc.Server.GracefulStop` with the same 25 s limit, then forces `Stop`. A second `Ctrl+C` kills
  the process immediately. `stop_grace_period: 30s` in `docker-compose.yml` gives Docker enough
  time before it sends `SIGKILL`.

## API examples

### Register

```bash
curl -X POST http://localhost:8080/register \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "Supersecret1!"}'
```

Creates the user and a default PLN account in one transaction and returns `201` with `user_id`
and `account_id`. Passwords must be 8–32 characters with at least one digit, one uppercase
letter and one special character.

### Login

```bash
curl -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "Supersecret1!"}'
```

Returns a JWT (valid for 15 minutes) to use as `Authorization: Bearer <token>` on the endpoints below.

### Check balance

```bash
curl http://localhost:8080/accounts/1/balance \
  -H "Authorization: Bearer <token>"
```

### Deposit

```bash
curl -X POST http://localhost:8080/accounts/1/deposit \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"amount_minor": 10000}'
```

### Withdraw

```bash
curl -X POST http://localhost:8080/accounts/1/withdraw \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"amount_minor": 5000}'
```

### Transfer (idempotent)

```bash
curl -X POST http://localhost:8080/accounts/1/transfer \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: a-unique-key-per-attempt" \
  -d '{"destination_account_id": 2, "amount_minor": 2500}'
```

### Transaction history (paginated)

```bash
curl "http://localhost:8080/accounts/1/history?limit=20&offset=0" \
  -H "Authorization: Bearer <token>"
```

`limit` defaults to 20 (max 100), `offset` to 0. Ready-made requests for the endpoints live in
`test-api-requests/*.http`.

## Reporting Service (gRPC)

A small, independent gRPC service that returns aggregated account statistics (total deposits,
withdrawals, transfers in/out, current balance) by reading directly from the same PostgreSQL
database as the main API.

### Why gRPC instead of another REST endpoint

This service is meant for internal, service-to-service communication rather than
browser/mobile clients. gRPC was chosen deliberately as the first and, for now, only use of
gRPC in this project — to learn the technology in a low-risk, isolated context (a small
read-only reporting service) without touching the existing Auth/Wallet logic, and without
splitting the system into microservices.

### Regenerating code from the `.proto` contract

The contract lives at `proto/reporting/v1/reporting.proto`; the generated Go code the services
import lives in `proto/reporting/pb/`.

> **Known issue:** the command below writes the files next to the `.proto` file
> (`proto/reporting/v1/`), not into `pb/`, and `option go_package` still points to the old path
> `internal/reporting/pb`. Until that is fixed, move the generated files into `pb/` by hand.

The command used so far:

```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       proto/reporting/v1/reporting.proto
```

### Running the server

```bash
go run ./proto/reporting
```

### Example request with grpcurl

gRPC reflection is enabled on this server (development convenience), so `grpcurl` can call it
without needing the `.proto` file:

```bash
grpcurl -plaintext -d '{"account_id": 1}' localhost:50051 reporting.v1.ReportingService/GetAccountSummary
```

### Running tests

```bash
go test ./internal/reporting/... -v
```

(For the whole project: `go test ./... -race`, which is what CI runs. Integration tests start
PostgreSQL with testcontainers-go, so Docker must be running.)

Includes both unit tests (fake `db.Querier`, no database required) and integration tests
(testcontainers-go, real PostgreSQL, gRPC client connected via `bufconn`).

### Known limitations

- **Read-only**: the service only queries existing data; it never writes to the database.
- **No authorization**: at this MVP stage, `GetAccountSummary` does not verify that the caller
  is allowed to view the requested account. This must be addressed before any production use.
- **Single endpoint**: only `GetAccountSummary` is implemented. No streaming endpoints yet.

## Known limitations (project-wide)

- Only PLN currency is supported.
- No refresh tokens; JWTs are short-lived with no rotation mechanism.
- Single database instance; no read replicas. Pool size is configurable (`DB_MAX_CONNS`,
  `DB_MIN_CONNS`) but has not been load-tested.
- Idempotency is not full replay yet: retrying a transfer with an already used
  `X-Idempotency-Key` returns `409 Conflict` instead of the original transfer (TODO PF-14.1).
- Emails are not normalised (no lowercase/trim), so `User@x.com` and `user@x.com` can register
  as two different users.
- No rate limiting on any endpoint.
- CI (GitHub Actions, `.github/workflows/ci.yml`) runs build, vet, `golangci-lint`, `go test -race`
  and `docker compose build` on pushes and PRs to `main`. The lint step is currently non-blocking
  (`continue-on-error: true`).
- Kafka, Kubernetes, Prometheus/Grafana, and advanced AWS usage are intentionally out of scope
  for this project — the focus is depth on a smaller, well-tested stack (Go, Gin, PostgreSQL,
  gRPC) rather than breadth across many technologies.