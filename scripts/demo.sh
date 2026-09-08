#!/bin/sh
set -eu

curl --fail-with-body -X POST http://localhost:8080/payments \
  -H 'Content-Type: application/json' \
  -d '{"payment_id":"pay_demo_1042","amount_cents":12900,"currency":"USD","attempt":5,"failure_class":"processor"}'

curl --fail-with-body -X POST http://localhost:8080/worker/run
