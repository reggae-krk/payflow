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
| Language | Go 1.22+ |
| Web framework | Gin |
| Database | PostgreSQL |
| DB driver | pgx (raw SQL, no ORM) |
| Migrations | golang-migrate |
| Auth | JWT (golang-jwt) + bcrypt |
| Inter-service RPC | gRPC + Protocol Buffers |
| Containerization | Docker + Docker Compose (database only) |
| Testing | standard `testing` + testcontainers-go |

## Data model (simplified ledger)

- **users**: `id`, `email`, `password_hash`, `created_at`
- **accounts**: `id`, `user_id` (FK), `currency`, `balance_minor` (BIGINT — always minor units, never float), `created_at`
- **transfers**: `id`, `idempotency_key` (unique), `source_account_id`, `destination_account_id`, `amount_minor`, `status`, `created_at`
- **ledger_entries**: `id`, `transfer_id` (nullable), `account_id`, `entry_type` (credit/debit), `amount_minor`, `created_at`

### Key domain rules

1. Monetary amounts are always stored as `BIGINT` in minor units (e.g. grosze/cents) — never as floating point.
2. An account balance can never drop below zero.
3. A transfer is a single atomic SQL transaction: debit sender, credit recipient, one `transfers` row, two `ledger_entries` rows.
4. An `X-Idempotency-Key` header prevents a retried request from executing the same transfer twice.
5. Transfer locks are acquired in a fixed order (ascending account ID) to prevent deadlocks on concurrent transfers.

## Running the project

### Prerequisites

- Go 1.22+
- Docker and Docker Compose
- `golang-migrate` CLI
- `grpcurl` (optional, for testing the gRPC service manually)

### 1. Start the database

```bash
docker compose up -d
```

This starts a PostgreSQL 16 container with a healthcheck and a persistent volume.

### 2. Run migrations

```bash
migrate -path migrations -database "postgres://payflow:payflow_dev_password@localhost:5432/payflow?sslmode=disable" up
```

### 3. Set required environment variables

The API requires a JWT signing secret; the Reporting Service only needs database connection settings.

```bash
export JWT_SECRET=your-local-dev-secret
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

## API examples

### Register

```bash
curl -X POST http://localhost:8080/register \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "supersecret"}'
```

### Login

```bash
curl -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "supersecret"}'
```

Returns a JWT to use as `Authorization: Bearer <token>` on the endpoints below.

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
curl "http://localhost:8080/accounts/1/history?page=1&page_size=20" \
  -H "Authorization: Bearer <token>"
```

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

The contract lives at `proto/reporting/v1/reporting.proto`. After changing it, regenerate the
Go bindings with:

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

Includes both unit tests (fake `db.Querier`, no database required) and integration tests
(testcontainers-go, real PostgreSQL, gRPC client connected via `bufconn`).

### Known limitations

- **Read-only**: the service only queries existing data; it never writes to the database.
- **No authorization**: at this MVP stage, `GetAccountSummary` does not verify that the caller
  is allowed to view the requested account. This must be addressed before any production use.
- **Single endpoint**: only `GetAccountSummary` is implemented. No streaming endpoints yet.
- **Not containerized**: unlike the database, neither the main API nor the Reporting Service
  currently run in Docker; both are started locally via `go run`. Adding a `Dockerfile` for
  both services is a natural next step once justified by need.

## Known limitations (project-wide)

- Only PLN currency is supported.
- No refresh tokens; JWTs are short-lived with no rotation mechanism.
- Single database instance; no read replicas or connection pooling tuning for high load.
- No rate limiting on any endpoint.
- CI (GitHub Actions) is planned but not yet set up.
- Kafka, Kubernetes, Prometheus/Grafana, and advanced AWS usage are intentionally out of scope
  for this project — the focus is depth on a smaller, well-tested stack (Go, Gin, PostgreSQL,
  gRPC) rather than breadth across many technologies.