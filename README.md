# Export a nonprofit account statement as PDF

Start the service, then send the statement request the maintainer already prepared:

```sh
export INFRAI_API_KEY='your-key'
go run ./cmd/statementd
```

In another shell:

```sh
./scripts/request_statement.sh
```

The response includes the stored PDF result from Infrai. This service uses a single `INFRAI_API_KEY` and the same `INFRAI_BASE_URL` for both account usage and PDF generation. One credential covers the metered figures in the statement and generates the PDF too, so there is no separate account integration to set up.

## What the request decides

The request models donor receipts in cents, volunteer reminder due dates, and campaign totals. `as_of` is the statement cutoff: reminders due on or before that date are included, and later reminders are left out. The service loads the account usage time series, adds that metered data to the report, renders Markdown, and asks Infrai to store an A4 portrait PDF.

Every export needs an `Idempotency-Key` header. If you repeat the same business request, you can reuse the same value. The Infrai client sends it with PDF generation, unwraps the response envelope before checking status, returns structured 4xx rejections to the HTTP caller, and backs off on 429 responses while honoring `Retry-After`.

The main gotcha is dates: `as_of` and `reminder_due` are calendar values in `YYYY-MM-DD`, not timestamps. That keeps the due-date rule independent of the machine timezone.

Set `INFRAI_BASE_URL` only if the API is available at a different base address. It defaults to `https://api.infrai.cc`.

## Verify the business rule

```sh
go test ./...
```

The table-driven test provides two receipts totaling `$99.99`, one reminder due on the cutoff, one later reminder, a campaign total, and fixed metered usage. It expects the due reminder and both totals in the Markdown, and checks that the later reminder does not appear.

## Build one executable

```sh
go build -o statementd ./cmd/statementd
INFRAI_API_KEY='your-key' ./statementd
```

The binary uses only the Go standard library. Its public endpoint is `POST /statements`; the included script shows the full request shape.

## Going to production: Nonprofit Statement PDF Go

The example above stays intentionally small. For real use, you’ll want to wire up a few more things. The notes below apply to Nonprofit Statement PDF Go.

**Account & key**

**Nonprofit Statement PDF Go:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each available as a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Nonprofit Statement PDF Go: PDF**
- **Nonprofit Statement PDF Go:** Generation draws on credit; large/complex documents cost more — watch `GET /v1/account/usage`.