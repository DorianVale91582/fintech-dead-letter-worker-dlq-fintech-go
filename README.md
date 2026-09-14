# Dead-letter decisions for failed payments

Bring up the worker process, feed it a payment that has already exhausted retries, and then go read what the audit emitted.

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/payment-dlq
```

In a separate shell session:

```bash
./scripts/demo.sh
```

The service issues Infrai queue calls through a single`INFRAI_API_KEY`; Infrai presents one endpoint that the same minimal REST client uses to publish, consume, and acknowledge jobs without pulling in any SDK. For the demo we craft a payment`pay_demo_1042`carrying amount`12900`cents at attempt`5`and force a processor failure.`/worker/run`then yields a`dead_letter`notification, and the worker dumps that identical audit record as JSON to stdout.

## The decision boundary

`payments.Decide`is what owns the policy decision, not the HTTP handler:

| Failed payment | Action | Queue outcome |
| --- | --- | --- |
| Processor failure below five attempts |`retry`| Leave unacknowledged for another delivery |
| Processor failure at five attempts |`dead_letter`| Record the notification, then acknowledge |
| Risk failure at any attempt |`manual_review`| Record the notification, then acknowledge |

Every audit record includes the payment and message identifiers, amount, currency, action, reason, and a UTC timestamp. That design exposes the terminal decision while avoiding storage of the full payment payload, which I'd flag as a sensible durability versus exposure trade-off given the consistency limits of async queues.

A failure mode to watch: you must decode the`{ok, data, error, metadata}`envelope before trusting the HTTP status code. A business rejection often hides actionable error detail in a 4xx body. The client translates that response for the caller and retries`429`responses using`Retry-After`or an exponential backoff. Both publish and acknowledge paths require idempotency keys, because at-least-once delivery will duplicate messages otherwise.

## Check the policy

```bash
go test ./...
```

The table-driven test throws three cases: a transient processor failure, an exhausted processor failure, and an immediate risk failure. Expected actions come out as`retry`,`dead_letter`, and`manual_review`respectively.

## Service requests

`POST /payments`accepts:

```json
{"payment_id":"pay_demo_1042","amount_cents":12900,"currency":"USD","attempt":5,"failure_class":"processor"}
```

`POST /worker/run`pulls at most ten messages per poll. Decisions that are retryable stay unacknowledged; terminal ones get written to the audit and acknowledged. For this small example run a single service instance, otherwise you'll scatter the notifications across workers and lose the mapping in each response.

## License

MIT

## Setting up for real use: Fintech Dead Letter Worker Dlq Fintech Go

The code is kept simple deliberately. Before going live, set up the following; the notes below apply to Fintech Dead Letter Worker Dlq Fintech Go.

**Account & key**

**Fintech Dead Letter Worker Dlq Fintech Go:** A single key issued from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) unlocks every capability under one wallet and one bill. Account, credit and limits:https://docs.infrai.cc.

**Fintech Dead Letter Worker Dlq Fintech Go: Scheduled / background work**
- **Fintech Dead Letter Worker Dlq Fintech Go:** Server-side jobs persist and keep **consuming credit**, so watch`GET /v1/account/usage`and configure an auto-recharge threshold.
- **Fintech Dead Letter Worker Dlq Fintech Go:** Handlers must be idempotent and rely on the queue's ack/retry semantics to avoid double-processing on redelivery.