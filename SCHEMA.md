# Database Schema Reference

## Tables

### `users`

| Column Name | Data Type | Constraints |
| --- | --- | --- |
| `id` | `BIGINT` | `PK`, `GENERATED ALWAYS AS IDENTITY` |
| `email` | `TEXT` | `NOT NULL`, `UNIQUE` |
| `password_hash` | `TEXT` | `NOT NULL` |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL`, `DEFAULT now()` |

---

### `accounts`

| Column Name | Data Type | Constraints |
| --- | --- | --- |
| `id` | `BIGINT` | `PK`, `GENERATED ALWAYS AS IDENTITY` |
| `user_id` | `BIGINT` | `NOT NULL`, `FK (users.id)` |
| `currency` | `CHAR(3)` | `NOT NULL`, `DEFAULT 'PLN'`, `CHECK (currency IN ('PLN'))` |
| `balance_minor` | `BIGINT` | `NOT NULL`, `DEFAULT 0`, `CHECK (balance_minor >= 0)` |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL`, `DEFAULT now()` |

**Additional Table Constraints:**
- `accounts_balance_non_negative`: `CHECK (balance_minor >= 0)`
- `accounts_currency_supported`: `CHECK (currency IN ('PLN'))`
- `accounts_user_currency_unique`: `UNIQUE (user_id, currency)`
- `balance_non_negative`: `CHECK (balance_minor >= 0)`

---

### `transfers`

| Column Name | Data Type | Constraints |
| --- | --- | --- |
| `id` | `BIGSERIAL` | `PK` |
| `idempotency_key` | `TEXT` | `UNIQUE` |
| `source_account_id` | `BIGINT` | `NOT NULL`, `FK (accounts.id)` |
| `destination_account_id` | `BIGINT` | `NOT NULL`, `FK (accounts.id)` |
| `amount_minor` | `BIGINT` | `NOT NULL`, `CHECK (amount_minor > 0)` |
| `status` | `TEXT` | `NOT NULL`, `DEFAULT 'pending'`, `CHECK (status IN ('pending', 'completed', 'failed'))` |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL`, `DEFAULT NOW()` |

---

### `ledger_entries`

| Column Name | Data Type | Constraints |
| --- | --- | --- |
| `id` | `BIGSERIAL` | `PK` |
| `transfer_id` | `BIGINT` | `FK (transfers.id)` |
| `account_id` | `BIGINT` | `NOT NULL`, `FK (accounts.id)` |
| `operation_type` | `TEXT` | None |
| `entry_type` | `TEXT` | `NOT NULL`, `CHECK (entry_type IN ('credit', 'debit'))` |
| `amount_minor` | `BIGINT` | `NOT NULL`, `CHECK (amount_minor > 0)` |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL`, `DEFAULT NOW()` |

---

## Foreign Key Relationships

| Table Name | Foreign Key Column | Referenced Table | Referenced Column |
| --- | --- | --- | --- |
| `accounts` | `user_id` | `users` | `id` |
| `transfers` | `source_account_id` | `accounts` | `id` |
| `transfers` | `destination_account_id` | `accounts` | `id` |
| `ledger_entries` | `transfer_id` | `transfers` | `id` |
| `ledger_entries` | `account_id` | `accounts` | `id` |

---

## Indices

| Index Name | Table Name | Columns | Definition |
| --- | --- | --- | --- |
| `idx_accounts_user_id` | `accounts` | `user_id` | `CREATE INDEX idx_accounts_user_id ON accounts(user_id);` |
| `idx_ledger_entries_account_created_at` | `ledger_entries` | `account_id, created_at` | `CREATE INDEX idx_ledger_entries_account_created_at ON ledger_entries (account_id, created_at);` |
| `idx_ledger_entries_transfer_id` | `ledger_entries` | `transfer_id` | `CREATE INDEX idx_ledger_entries_transfer_id ON ledger_entries (transfer_id);` |

---

## Custom Types, Enums & Triggers

### Enums & Custom Types
No custom database types or native enum types (`CREATE TYPE ... AS ENUM`) are defined in the schema. Allowed values for enumerated status and type fields are constrained via `CHECK` constraints:
- `accounts.currency`: `CHECK (currency IN ('PLN'))`
- `transfers.status`: `CHECK (status IN ('pending', 'completed', 'failed'))`
- `ledger_entries.entry_type`: `CHECK (entry_type IN ('credit', 'debit'))`

### Triggers
No database triggers are defined in the schema.

