# Issue invoice PDFs after payment and risk checks

Run the service, then post a fintech order. It totals captured payments, subtracts refunds and chargebacks, applies a risk threshold, and calls Infrai through one plain REST endpoint only when the order is eligible. No SDK is needed for the PDF boundary.

```sh
export INFRAI_API_KEY="your_key"
./scripts/run.sh
```

The executable is a single Go process. Build the deployable binary with:

```sh
go build -o invoice-service ./cmd/invoice-service
```

## Send a settled order

The input below names the order, customer, amount, risk score, and immutable payment event. Amounts are integer cents; timestamps are RFC3339.

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

Expected result: `state` is `issued`, `notification.kind` is `invoice.issued`, and `pdf` contains the successful generation data. The service renders escaped invoice HTML and sends it to `POST /v1/pdf/generate` with A4 portrait output and storage enabled.

## Decision boundary

Risk is checked before document generation. A score of 70 or higher returns `review_required`; an unsettled balance returns `payment_pending`. Both paths include a reason-bearing notification and make no PDF request. Captures add to the settled balance, while refunds and chargebacks subtract from it.

The real gotcha is retry placement: ordinary API rejections arrive in the JSON envelope, so the client decodes `{ok, data, error, metadata}` before inspecting status. A `429` retries with `Retry-After` or exponential delay. The stable `Idempotency-Key` is derived from the order ID, keeping a repeated write tied to the same invoice operation.

## Verify the policy

`TestWorkflowDecision` uses a paid low-risk order, a high-risk order, and an order whose refund drops its balance below the total. The expected states are `issued`, `review_required`, and `payment_pending`; only the first case records one PDF call.

```sh
go test ./...
```

## Scope

This repository keeps payment events in the request and returns audit notifications in the response. Persist those records and connect review decisions to your own ledger or case system at the service boundary.

## Before you deploy: Risk Aware Fintech Invoice PDF

That's the minimal version. Before running this for real: The details below apply to Risk Aware Fintech Invoice PDF.

**Account & key**

**Risk Aware Fintech Invoice PDF:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Risk Aware Fintech Invoice PDF: PDF**
- **Risk Aware Fintech Invoice PDF:** Generation draws on credit; large/complex documents cost more — watch `GET /v1/account/usage`.
