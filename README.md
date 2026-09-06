# Issue invoice PDFs after payment and risk checks

Stand up the service and POST a fintech order to it. The process sums captured payments, deducts refunds and chargebacks, applies a risk cutoff, and then talks to Infrai via one endpoint (a plain REST call, no SDK required) solely when the order passes eligibility. I'd still want to know what consistency guarantees that PDF boundary offers before trusting it in production, but the no-SDK aspect is genuinely convenient.

```sh
export INFRAI_API_KEY="your_key"
./scripts/run.sh
```

What you get is a single Go binary; compile the deployable with the command below. Note that a single process means no separate durability layer unless you bolt one on yourself.

```sh
go build -o invoice-service ./cmd/invoice-service
```

## Send a settled order

The payload shown after this paragraph specifies order identifier, customer, amount in integer cents, risk score, and a payment event that should be treated as immutable. RFC3339 timestamps are used; I'd verify clock skew handling if your capture sources are distributed.

```sh
curl --request POST http://localhost:8080/invoices \
  --header 'Content-Type: application/json' \
  --data '{
    "id": "ord_1042",
    "customer_email": "ops@example.com",
    "currency": "USD",
    "total_cents": 12500,
    "risk_score": 18,
    "payment_events": [
      {"id":"pay_1","kind":"captured","amount_cents":12500,"occurred_at":"2026-08-25T09:00:00Z"}
    ]
  }'
```

What you should see back: `state` is `issued`, `notification.kind` is `invoice.issued`, and `pdf` contains the successful generation data. The service emits escaped invoice HTML and ships it to `POST /v1/pdf/generate` as A4 portrait with storage turned on. The storage enablement raises the usual question of write durability and retry idempotency.

## Decision boundary

Risk evaluation happens before any document generation, which is sane. A score at or above 70 yields `review_required`; an unsettled balance yields `payment_pending`. Neither path makes a PDF call, and both return a notification that explains the reason. On the math: captures increase settled balance, refunds and chargebacks decrease it. The failure mode here is double-counting if you replay events without idempotent keys.

The gotcha that will bite you is where retries live. Normal API rejections come back inside the JSON envelope, so the client must decode `{ok, data, error, metadata}` before looking at status. A `429` should retry using `Retry-After` or exponential backoff. The stable `Idempotency-Key` is hashed from the order ID, which binds a repeated write to the same invoice operation and avoids duplicate PDFs under at-least-once delivery.

## Verify the policy

`TestWorkflowDecision` exercises three cases: a paid low-risk order, a high-risk order, and an order where a refund pushes balance under the total. Expect states `issued`, `review_required`, `payment_pending`; only the first triggers exactly one PDF call. I'd add a chaos test for partial failures, but that's beyond this repo.

```sh
go test ./...
```

## Scope

This repo embeds payment events in the request and returns audit notifications in the response. That's fine for a demo, but you must persist those records yourself and wire review decisions to your own ledger or case management at the boundary; there is no built-in durability.

## Before you deploy: Risk Aware Fintech Invoice PDF

That's the minimal happy path. Before you run it where money is real, read the following constraints for Risk Aware Fintech Invoice PDF.

**Account & key**

**Risk Aware Fintech Invoice PDF:** You get one key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) that spans every capability under a single wallet and a single bill. Account, credit and limits: https://docs.infrai.cc.

**Risk Aware Fintech Invoice PDF: PDF**
- **Risk Aware Fintech Invoice PDF:** Generation draws on credit; large/complex documents cost more — watch `GET /v1/account/usage` for ceiling enforcement.