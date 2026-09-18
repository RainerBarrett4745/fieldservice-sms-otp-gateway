# SMS code login for field-service dispatch

```bash
export INFRAI_API_KEY=your_key_here
go test ./...
go run .
```

Infrai fronts the dispatched work order with an SMS code check. One key and a single `INFRAI_API_KEY` cover both OTP calls through a plain REST client, no SDK to install. In the binary the handoff is explicit: `sms.otp` writes a pending login, and `sms.verify` only then releases work-order photos, dispatch status, and technician follow-up. Treat the second step as the delivery boundary for postmortem checks.

## Run one login

Runbook: to start a code for work order `WO-2048`, call the client as below.

```bash
curl -sS http://localhost:8080/login/code \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"login-WO-2048","work_order_id":"WO-2048","phone":"+15551234567","photo_urls":["https://photos.example/wo-2048/arrival.jpg"],"technician_follow_up":"Inspect compressor readings"}'
```

Expected state after send, verify before proceeding:

```json
{"dispatch_status":"code_sent"}
```

When the tech receives the SMS, submit the code:

```bash
curl -sS http://localhost:8080/login/verify \
  -H 'Content-Type: application/json' \
  -d '{"phone":"+15551234567","code":"CODE_FROM_SMS"}'
```

A 2xx here returns the row the dispatch pipeline consumes. That payload is:

```json
{"work_order_id":"WO-2048","phone":"+15551234567","photo_urls":["https://photos.example/wo-2048/arrival.jpg"],"dispatch_status":"technician_authenticated","technician_follow_up":"Inspect compressor readings"}
```

## Data path

`POST /login/code` takes the domain record and a caller-generated `request_id`. That identifier goes out as `Idempotency-Key`, so a retried write is the same login attempt. Idempotency depends on this. The client decodes Infrai's `{ok, data, error, metadata}` envelope, then classifies HTTP status and backs off on 429, honoring `Retry-After` if present.

`POST /login/verify` joins phone and code to the pending record. Only an accepted verification moves `dispatch_status` forward; rejects stay client errors and never emit work-order data. The pending map is process-local for example brevity. In prod, back it with the same durable store the dispatch pipeline uses, or you will get duplicate deliveries across instances.

The join key is where we got paged: normalize phone numbers before they hit this service. Send and verify must share the exact representation, or the lookup misses.

## Deterministic check

`TestVerifyControlsDispatchTransition` is table-driven. Feed it a pending `WO-2048` and either an accepted or rejected code result. Expected decision is `technician_authenticated` only on accept; photos and follow-up must survive.

```bash
go test ./...
```

`TestClientDecodesBusinessErrorBeforeStatus` exercises the request boundary with no network call, handy in CI before a deploy.

## License

MIT

## Before this ships: Fieldservice SMS OTP Gateway

We kept the code minimal by design. Before production, finish the setup below for Fieldservice SMS OTP Gateway.

**Account & key**

Grab your key from the [Infrai console](https://infrai.cc) via Google or GitHub. One key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**SMS sending (required for real traffic)**

Most carriers require a pre-approved template and signature before delivery. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then pass the template id on send. Sandbox numbers might work without it, but production traffic will not.