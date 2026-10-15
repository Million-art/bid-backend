# Integration Tests

End-to-end tests for the money-handling path: Chapa webhook → ledger upsert → ticket creation → wallet capture. They run against a real Postgres started via [testcontainers-go](https://github.com/testcontainers/testcontainers-go) so SQL-specific behavior (JSONB roundtrip, `ON CONFLICT`, decimal precision, transaction boundaries in `AtomicRecordPurchase`) is actually exercised  none of these would be caught by the in-memory mock suite.

## Running

```bash
# From admin-backend/
go test -tags=integration -race ./integration/...
```

**Requirements:** Docker daemon reachable. If Docker isn't available the suite calls `t.Skip` locally so unit-only workflows still work. In CI (`CI=true`) a missing Docker daemon fails the job.

**CI:** Runs before every deploy (`deploy.yml` → `migrate-and-test` on `feature/deployment`).

First run: ~90s (image pull + container boot). Subsequent runs: ~10s for the cached image.

## What's covered

| Test | What it pins down |
| :--- | :--- |
| `Chapa_HappyPath_CreatesTicketAndMarksPaymentProcessed` | The 95% case  Chapa says success, ticket lands, ledger marks processed, broadcast fires. |
| `Chapa_DuplicateDelivery_IsIdempotent` | Webhook retries don't double-mint tickets or oversell. |
| `Chapa_VerifyStatusNotSuccess_RecordsFailureNoTicket` | Declined card → ledger row with status=failed, NO ticket. |
| `Chapa_MetaMissing_RecordsFailureNoTicket` | Malformed Chapa payload still recorded for ops, NO ticket. |
| `Chapa_TicketsExhausted_DefersFulfillment` | Race: webhook arrives after sold-out → `ErrFulfillmentDeferred` so client-backend can return 200 to Chapa. |
| `ChapaRetry_PreviouslyDeferredPayment_GetsFulfilled` | `RetryUnprocessedPayments` recovers a deferred row when capacity reopens. |
| `WalletPurchase_HappyPath_DeductsBalanceAndCreatesTicket` | Internal-wallet purchase → balance → 0, `participant_purchase` ledger tx written, ticket created. |
| `WalletPurchase_InsufficientBalance_NoTicketNoDeduction` | The historical "balance went to 0 but no ticket" failure mode is impossible. |
| `RecordPurchase_RespectsMaxTicketsPerUser` | Per-user cap holds across two purchase calls. |
| `RecordPurchase_Suspendedbid_Rejects` | Suspended bids refuse sales (even through the service layer, not just the HTTP guard). |
| `LockbidRevenue_AfterSales_PopulatesAgentAndPlatformPending` | The 90/10 split lands in agent + platform pending wallets correctly. |

## Why these specific tests

These all sit on the **money invariant boundary**: a bug here either gives a participant a ticket they didn't pay for, charges a participant who didn't get a ticket, or moves the wrong amount into an agent's payable balance. Each scenario was chosen to either (a) protect a known historical failure mode, or (b) pin down a behavior that a refactor could silently break.

If you're adding a new test, the rule of thumb: would a regression cost the company money or a customer their seat? If yes, it belongs here. If it's about validation/UI/parsing, prefer a unit test in `core/services/*/service_test.go`.

## Architecture

* `testutil/pg.go`  testcontainers harness. One container per process; per-test isolation via `TRUNCATE ... RESTART IDENTITY CASCADE`.
* `testutil/wiring.go`  constructs the **real** `wallet.Service` + `bid.Service` + `chapapayment.Service` against real postgres repos. Only the outermost collaborators (email, websocket, telegram, redis) are mocked.
* `testutil/fixtures.go`  `SeedAgent`, `SeedParticipant`, `Seedbid` with sensible defaults so each test stays short and focused on the actual assertion.

A migration written today and shipped tomorrow gets run by `database.Migrate` here too  the same code path as production. If your migration breaks the integration suite, ship a fix before merging.
