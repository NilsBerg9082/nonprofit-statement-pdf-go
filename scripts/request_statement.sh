#!/bin/sh
set -eu

curl --fail-with-body --request POST "${STATEMENT_URL:-http://localhost:8080}/statements" \
  --header 'Content-Type: application/json' \
  --header 'Idempotency-Key: river-pantry-2026-08' \
  --data-binary @- <<'JSON'
{
  "account_name": "River Pantry",
  "period": "August 2026",
  "as_of": "2026-08-31",
  "receipts": [
    {"donor": "A. Chen", "amount_cents": 2550, "issued_on": "2026-08-04"},
    {"donor": "M. Singh", "amount_cents": 7449, "issued_on": "2026-08-20"}
  ],
  "volunteer_shifts": [
    {"volunteer": "Noah", "reminder_due": "2026-08-31", "task": "confirm Saturday shift"},
    {"volunteer": "Inez", "reminder_due": "2026-09-02", "task": "confirm delivery route"}
  ],
  "campaigns": [
    {"name": "School supplies", "raised_cents": 183425}
  ]
}
JSON
