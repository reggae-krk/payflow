# PayFlow System Architecture

## 1. High-Level Overview
PayFlow is a financial ledger and payment flow system written in Go. It provides user management, wallet accounts (the schema is multi-currency-ready, but only PLN is supported), double-entry transaction ledgering, fund transfers, and transactional financial reporting. The core stack consists of Go 1.26, Gin for REST APIs, gRPC with Protocol Buffers for a single read-only reporting service, PostgreSQL with `pgx/v5` (`pgxpool`), JWT authentication, and Docker/Testcontainers for deployment and integration testing.

## 2. Directory Layout
* `cmd/`: Application entry points.
  * `cmd/api/`: REST API HTTP server (`main.go`, `routes.go`).
* `proto/`: gRPC protobuf definitions and server entry points.
  * `proto/reporting/`: gRPC reporting service entry point (`main.go`) and generated protobuf files (`pb/`, `v1/`).
* `internal/`: Private application code.
  * `app/`: Cross-domain orchestration workflows (e.g., user registration with default wallet initialization).
  * `auth/`: Authentication middleware (`RequireAuth`) and JWT token creation/validation.
  * `config/`: Environment configuration, validated at startup. `Load()` (used by `cmd/api`) returns DB settings, `JWT_SECRET` and the request timeouts; `LoadDBConfig()` (used by `proto/reporting`) returns only DB settings, so the reporting service doesn't depend on a secret it never uses. Both return an `error` that lists every invalid or missing variable (`errors.Join`).
  * `db/`: Database abstractions: `Querier` interface unifying `pgx.Tx` and `pgxpool.Pool`, `NewPool` (pool sized from config), `FinishTx` (deferred rollback helper) and Postgres error code helpers (`IsUniqueViolation`, `IsForeignKeyViolation`, `IsCheckViolation`).
  * `reporting/`: gRPC reporting service implementation for account summaries and aggregations.
  * `users/`: User domain logic, entity models, password hashing (bcrypt), input validation, repository, and HTTP handlers.
  * `wallet/`: Core financial domain logic (accounts, transfers, double-entry ledger entries, currencies), transactional services, repositories, and HTTP handlers.
  * `testhelpers/`: Test infrastructure and Postgres Testcontainers setup.
* `migrations/`: SQL migration files (`golang-migrate`) defining database tables, foreign keys, and non-negative balance constraints.
* `docs/`: System documentation, DBML definitions, and entity-relationship diagrams (ERD).

## 3. Data Flow
* **HTTP REST Request Flow (e.g., Fund Transfers / Deposits / Withdrawals)**:
  1. **Client Request**: Client sends an HTTP request with a JSON payload and Bearer JWT token to `cmd/api`.
  2. **Router & Middleware**: `timeoutMiddleware` replaces the request context with one that has the route group's deadline (`HEALTH_TIMEOUT`, `LOGREG_TIMEOUT` or `REQUEST_TIMEOUT`). Then `auth.RequireAuth` verifies the JWT token, extracts `userID`, and stores it in the Gin context.
  3. **Handler Layer**: `wallet.Handler` parses request parameters/body (`ShouldBindJSON`), extracts `userID`, and calls `wallet.AccountService` with `c.Request.Context()`, so the deadline and a client disconnect reach pgx.
  4. **Service Layer**: `wallet.service` validates request logic, opens a Postgres transaction (`pool.Begin(ctx)`), verifies account ownership, and adjusts balances through a single atomic `UPDATE accounts SET balance_minor = balance_minor + $1 WHERE id = $2 AND balance_minor + $1 >= 0` statement per account. For transfers, both account rows are first locked with `SELECT ... FOR UPDATE` (`GetByIDForUpdate`) in ascending account ID order before the two `UPDATE` statements run. See "Transfer Lock Ordering" below.
  5. **Repository Layer**: Repositories (`AccountRepository`, `TransferRepository`, `LedgerRepository`) execute raw parameterized SQL using the `db.Querier` interface (bound to `pgx.Tx`).
  6. **Database Execution**: PostgreSQL executes raw SQL queries and enforces schema constraints (e.g., non-negative balances via `CHECK`).
  7. **Response Flow**: Repositories map rows to domain structs; the service commits the transaction; the handler serializes domain results into HTTP JSON responses.
  8. **Errors and cancellation**: every transactional method has a named `err` return and `defer db.FinishTx(ctx, tx, &err)`. `FinishTx` rolls back on a context detached from the request (`context.WithoutCancel` + `db.RollbackTimeout` = 5 s), so a cancelled request can still roll back. If the deadline fires *during* a query, pgx closes the connection instead; the rollback then fails with `conn closed` (joined into the returned error) and PostgreSQL aborts the transaction when it sees the disconnect.
* **gRPC Request Flow (e.g., Account Summary Reporting)**:
  1. **Client Request**: gRPC client invokes `GetAccountSummary` on port 50051.
  2. **gRPC Server**: `reporting.server` receives the request.
  3. **Database Aggregation**: Executes parameterized aggregate SQL queries directly via the `db.Querier` interface (in production wired to a `*pgxpool.Pool`; in unit tests wired to a fake implementation) to compute deposits, withdrawals, and transfers in/out.
  4. **Response Flow**: Query results are scanned into protobuf response structs (`pb.GetAccountSummaryResponse`) and returned to the client.

## 4. Core Design Patterns
* **Dependency Injection & Interface Segregation**: Components depend on explicit Go interfaces (`UserHandler`, `AccountService`, `AccountRepository`, `Querier`) passed via constructor functions (`NewHandler`, `NewService`, `NewAccountRepository`), ensuring decoupled design and testability.
* **Querier Pattern (Unified DB/Tx Operations)**: The `db.Querier` interface abstracts both `*pgxpool.Pool` and `pgx.Tx` behind the same method set (`Exec`, `Query`, `QueryRow`), allowing repositories to run either inside a transaction or directly against the pool without code duplication. The Reporting Service reuses this same interface to swap a real pool for a fake in unit tests, without needing a running database.
* **Double-Entry Ledger Pattern**: Financial operations append immutable `LedgerEntry` records (debit/credit) alongside account balance updates for auditability.
* **Atomic Compare-and-Update Balance Guard**: Instead of a separate read-then-check-then-write sequence, balance changes go through a single `UPDATE ... WHERE balance_minor + $1 >= 0` statement. PostgreSQL evaluates the `WHERE` clause and applies the update atomically for that row, so a concurrent transaction cannot observe a stale balance between check and write for the *same* account.
* **Domain Error Wrapping & Translation**: Low-level database errors (`pgconn.PgError`) are inspected via helper functions (`IsUniqueViolation`, `IsForeignKeyViolation`, `IsCheckViolation`) and translated into domain errors (`ErrUserNotFound`, `ErrDuplicateTransfer`, `ErrInsufficientFunds`, `ErrForbidden`). Note: `ErrUserNotFound` is defined in the `wallet` package (raised when creating an account for a non-existent user violates a foreign key), not in `users`.

## 5. Transfer Lock Ordering
`Transfer()` prevents circular-wait deadlocks between concurrent transfers by locking both accounts in a fixed order:
1. Source and destination IDs are sorted so the lower account ID is locked first.
2. Each account row is locked with `SELECT ... FOR UPDATE` (`AccountRepository.GetByIDForUpdate`) inside the transfer transaction. A missing account maps to `ErrAccountNotFound`.
3. Ownership of the source account is checked against the requesting user (`ErrForbidden`).
4. Balances are adjusted with the atomic `UPDATE ... WHERE balance_minor + $1 >= 0` guard (source debit, then destination credit). Since both rows are already locked, the order of these updates cannot cause a deadlock.
5. The `transfers` row and both `ledger_entries` rows are inserted, then the transaction commits.

Two opposite-direction transfers (account 1 -> 2 and account 2 -> 1) therefore both try to lock account 1 first; one waits for the other instead of each holding one lock and blocking on the other. `TestServiceTransferNoDeadlockOnConcurrentOppositeTransfers` in `internal/wallet/service_integration_test.go` covers this case against a real PostgreSQL instance.

Deposits and withdrawals touch a single account and rely only on the atomic `UPDATE` guard; they do not take an explicit row lock.

## 6. Configuration & Environment
* **Configuration Loading**: Environment variables are parsed in `internal/config` using `os.LookupEnv`. DB settings have local-development defaults; `JWT_SECRET`, `HEALTH_TIMEOUT`, `LOGREG_TIMEOUT` and `REQUEST_TIMEOUT` are required. Any missing, malformed or inconsistent value is returned as an `error` and `run()` exits before the server starts (no panics). `cmd/api` additionally checks that each request timeout + `db.RollbackTimeout` is below the HTTP `WriteTimeout` (15 s) and the shutdown timeout (25 s). See the Configuration table in `README.md`.
* **Database Connection Pooling**: Database connections are managed by `pgxpool.Pool`. Both services create it with `db.NewPool(ctx, cfg)`, which parses `cfg.ConnString()` and applies `DB_MAX_CONNS` / `DB_MIN_CONNS` (defaults match pgxpool: `max(4, NumCPU)` and `0`). Both services `Ping` the database at startup (5 s timeout) and exit if it is unreachable.
* **JWT secret**: `internal/auth` reads `JWT_SECRET` from the environment on every sign/verify call (`config.LoadJWTSecret()`), in addition to the startup check in `Load()`. Injecting the secret once at startup is a planned refactor.

## 7. Server Lifecycle
* **HTTP API** (`cmd/api`): `http.Server` with `ReadHeaderTimeout` 5 s, `ReadTimeout` 5 s, `WriteTimeout` 15 s, `IdleTimeout` 30 s. On `SIGINT`/`SIGTERM` it calls `Shutdown` and waits up to 25 s for in-flight requests.
* **Reporting** (`proto/reporting`): on `SIGINT`/`SIGTERM` it calls `GracefulStop`; if RPCs are still running after 25 s it falls back to `Stop`.
* A second signal restores the default behaviour, so a second `Ctrl+C` kills the process at once. `docker-compose.yml` sets `stop_grace_period: 30s` (> 25 s) for both services.