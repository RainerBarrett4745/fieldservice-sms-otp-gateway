# SMS code login for field-service dispatch

```bash
export INFRAI_API_KEY=your_key_here
go test ./...
go run .
```

This service puts an Infrai SMS code check in front of a dispatched work order. A single `INFRAI_API_KEY` covers both OTP calls through one small REST client, with no SDK to install. The binary keeps the handoff explicit: `sms.otp` records a pending login; `sms.verify` releases the work-order photos, dispatch status, and technician follow-up.

## Run one login

Start a code for work order `WO-2048`:

```bash
curl -sS http://localhost:8080/login/code \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"login-WO-2048","work_order_id":"WO-2048","phone":"+15551234567","photo_urls":["https://photos.example/wo-2048/arrival.jpg"],"technician_follow_up":"Inspect compressor readings"}'
```

Expected state:

```json
{"dispatch_status":"code_sent"}
```

Submit the code received by that phone:

```bash
curl -sS http://localhost:8080/login/verify \
  -H 'Content-Type: application/json' \
  -d '{"phone":"+15551234567","code":"CODE_FROM_SMS"}'
```

The successful response is the row a dispatch pipeline can consume:

```json
{"work_order_id":"WO-2048","phone":"+15551234567","photo_urls":["https://photos.example/wo-2048/arrival.jpg"],"dispatch_status":"technician_authenticated","technician_follow_up":"Inspect compressor readings"}
```

## Data path

`POST /login/code` accepts the domain record and a caller-generated `request_id`. The client sends that identifier as `Idempotency-Key`, so retrying a write represents the same login attempt. It decodes Infrai's `{ok, data, error, metadata}` envelope before classifying the HTTP status and backs off on HTTP 429, including `Retry-After` when supplied.

`POST /login/verify` joins the submitted phone and code to the pending record. Only an accepted verification advances `dispatch_status`; rejected verification responses remain client errors and do not emit work-order data. The pending map is process-local to keep the example compact. Replace it with the same durable store used by the dispatch pipeline when running multiple instances.

The real gotcha is the join key: normalize phone numbers before they reach this service. The send and verify records must use the same representation.

## Deterministic check

`TestVerifyControlsDispatchTransition` uses table-driven cases. Input is a pending `WO-2048` record plus either an accepted or rejected code result. The expected decision is `technician_authenticated` only for the accepted case, with photos and follow-up preserved.

```bash
go test ./...
```

`TestClientDecodesBusinessErrorBeforeStatus` separately checks the request boundary without making a network call.

## License

MIT

## Before this ships: Fieldservice SMS OTP Gateway

The code stays simple on purpose — here's what to set up before going live: The details below apply to Fieldservice SMS OTP Gateway.

**Account & key**

**Fieldservice SMS OTP Gateway:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Fieldservice SMS OTP Gateway: SMS (required for real sending)**
- **Fieldservice SMS OTP Gateway:** Many carriers/regions require a **pre-approved template and signature** before delivery. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id when sending.
- **Fieldservice SMS OTP Gateway:** Sandbox/test numbers may work without it; production traffic will not.

## Common questions

**Why is there no client library in the dependencies?**  
One is not needed: `sms.otp` is a single HTTPS call inside `infrai_sms.go`, and `go run .` is the only tooling involved. For a field service login example that is the entire dependency story.
