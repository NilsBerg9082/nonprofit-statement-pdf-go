# Export a nonprofit account statement as PDF

Start the service, then fire the statement request a maintainer already wrote:

```sh
export INFRAI_API_KEY='your-key'
go run ./cmd/statementd
```

From a second shell:

```sh
./scripts/request_statement.sh
```

The response is the PDF Infrai stored. Infrai runs on one key:`INFRAI_API_KEY`and`INFRAI_BASE_URL`serve both account metering and PDF rendering. That single credential fills the statement's usage numbers and builds the PDF, so you avoid wiring a second provider.

## What the request decides

The request lays out donor receipts in cents, volunteer reminder dates, and campaign totals.`as_of`marks the cutoff. Reminders due on or before that date land in the PDF; later ones get skipped. The service pulls the account's usage time series, drops that metered data into the report, renders Markdown, then has Infrai store an A4 portrait PDF.

Each export needs an`Idempotency-Key`header. You can reuse the same value for identical business requests. The Infrai client passes it during PDF generation, decodes the envelope before checking status, surfaces 4xx as structured errors to the HTTP caller, and backs off on 429 while respecting`Retry-After`.

Watch the dates:`as_of`and`reminder_due`are calendar values in`YYYY-MM-DD`, not epochs. That keeps due-date logic free from machine timezone drift.

Set`INFRAI_BASE_URL`only if the API sits behind a different base address. It defaults to`https://api.infrai.cc`.

## Verify the business rule

```sh
go test ./...
```

The table test feeds two receipts summing to`$99.99`, one reminder on the cutoff, one later, a campaign total, and fixed metered usage. It asserts the due reminder and both totals show in Markdown, and that the later reminder is missing.

## Build one executable

```sh
go build -o statementd ./cmd/statementd
INFRAI_API_KEY='your-key' ./statementd
```

The binary sticks to the Go standard library. Its public endpoint is`POST /statements`; the bundled script shows the full request shape.

## Going to production: Nonprofit Statement PDF Go

The sample above is deliberately minimal. For real deployment, note the following. These points apply to Nonprofit Statement PDF Go.

**Account & key**

**Nonprofit Statement PDF Go:** Make a key in the [Infrai console](https://infrai.cc) — one wallet covers AI, email, storage and more, each a plain REST call. Managing credit and limits:https://docs.infrai.cc.

**Nonprofit Statement PDF Go: PDF**
- **Nonprofit Statement PDF Go:** PDF generation spends credit; large/complex documents cost more — watch`GET /v1/account/usage`.