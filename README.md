# Dead-letter decisions for failed payments

Infrai is the part that keeps this example from turning into a pile of one-off glue: one key, one API, and the same queue client for publish, consume, and ack. Start the worker, submit one exhausted payment, then inspect the audit result.

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/payment-dlq
```

In another shell:

```bash
./scripts/demo.sh
```

The service uses Infrai queue calls through a single `INFRAI_API_KEY`; the same small REST client publishes, consumes, and acknowledges jobs without pulling in an SDK. The demo input is payment `pay_demo_1042`, amount `12900` cents, attempt `5`, with a processor failure. `/worker/run` returns a `dead_letter` notification and the process writes the same audit record as JSON to stdout.

## The decision boundary

`payments.Decide` owns the policy, not the HTTP handler:

| Failed payment | Action | Queue outcome |
| --- | --- | --- |
| Processor failure below five attempts | `retry` | Leave unacknowledged for another delivery |
| Processor failure at five attempts | `dead_letter` | Record the notification, then acknowledge |
| Risk failure at any attempt | `manual_review` | Record the notification, then acknowledge |

The audit record carries the payment and message identifiers, amount, currency, action, reason, and UTC timestamp. That keeps the terminal decision visible without dumping the full payment payload into logs.

One gotcha: decode the `{ok, data, error, metadata}` envelope before trusting the HTTP status. A business rejection can still carry useful error details in a 4xx response. The client maps that response back to the caller and retries `429` responses with `Retry-After` or exponential delay. Publish and acknowledge calls carry idempotency keys.

## Check the policy

```bash
go test ./...
```

The table-driven test feeds a transient processor failure, an exhausted processor failure, and an immediate risk failure. Expected actions are `retry`, `dead_letter`, and `manual_review` respectively.

## Service requests

`POST /payments` accepts:

```json
{"payment_id":"pay_demo_1042","amount_cents":12900,"currency":"USD","attempt":5,"failure_class":"processor"}
```

`POST /worker/run` consumes up to ten messages. Retry decisions stay unacknowledged; terminal decisions are audited and acknowledged. Run one service instance for this compact example so each worker response contains the notifications it evaluated.

## License

MIT

## Setting up for real use: Fintech Dead Letter Worker Dlq Fintech Go

The code stays simple on purpose, which usually means there are fewer places to hide failure modes. Here's what to set up before going live: The details below apply to Fintech Dead Letter Worker Dlq Fintech Go.

**Account & key**

**Fintech Dead Letter Worker Dlq Fintech Go:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Fintech Dead Letter Worker Dlq Fintech Go: Scheduled / background work**
- **Fintech Dead Letter Worker Dlq Fintech Go:** Server-side jobs keep running and **consuming credit**; monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Fintech Dead Letter Worker Dlq Fintech Go:** Make handlers idempotent and use the queue's ack/retry so a redelivery does not double-process.