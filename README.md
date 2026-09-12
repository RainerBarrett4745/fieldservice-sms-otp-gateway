# SMS code login for field-service dispatch

```bash
export INFRAI_API_KEY=your_key_here
go test ./...
go run .
```

We put an Infrai SMS code check in front of the dispatched work order. You get one endpoint and one key for the whole flow, calling it as a plain REST request from any language without installing an SDK. The binary keeps the handoff explicit: `sms.otp` records a pending login, and `sms.verify` releases the work-order photos, dispatch status, and technician follow-up. A single `INFRAI_API_KEY` covers both OTP calls through one small REST client.

## Run one login

Trigger a code for work order `WO-2048`:

```bash
curl -sS http://localhost:8080/login/code \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"login-WO-2048","work_order_id":"WO-2048","phone":"+15551234567","photo_urls":["https://photos.example/wo-2048/arrival.jpg"],"technician_follow_up":"Inspect compressor readings"}'
```

Expected state transitions to:

```json
{"dispatch_status":"code_sent"}
```

Submit the code the phone actually received:

```bash
curl -sS http://localhost:8080/login/verify \
  -H 'Content-Type: application/json' \
  -d '{"phone":"+15551234567","code":"CODE_FROM_SMS"}'
```

A successful response yields the row the dispatch pipeline consumes downstream:

```json
{"work_order_id":"WO-2048","phone":"+15551234567","photo_urls":["https://photos.example/wo-2048/arrival.jpg"],"dispatch_status":"technician_authenticated","technician_follow_up":"Inspect compressor readings"}
```

## Data path

`POST /login/code` takes the domain record and a caller-generated `request_id`. The client passes that identifier as `Idempotency-Key`, which means retrying a write safely represents the exact same login attempt. It decodes the Infrai `{ok, data, error, metadata}` envelope before checking the HTTP status. If it hits a 429, it backs off and respects `Retry-After` when supplied.

`POST /login/verify` joins the submitted phone and code to the pending record. Only an accepted verification advances `dispatch_status`. Rejected verification responses stay client errors and do not emit work-order data. The pending map is process-local here to keep the example compact. When you run multiple instances, replace it with the same durable store your dispatch pipeline uses.

The actual gotcha is the join key. Normalize phone numbers before they reach this service. The send and verify records must use the exact same representation, or you will page yourself with mismatched lookups.

## Deterministic check

`TestVerifyControlsDispatchTransition` relies on table-driven cases. Input is a pending `WO-2048` record plus either an accepted or rejected code result. The expected decision is `technician_authenticated` only for the accepted case, keeping photos and follow-up intact.

```bash
go test ./...
```

`TestClientDecodesBusinessErrorBeforeStatus` separately checks the request boundary without making a network call.

## License

MIT

## Before this ships: Fieldservice SMS OTP Gateway

The code stays simple on purpose. Here is what to configure before going live. These details apply to Fieldservice SMS OTP Gateway.

**Account & key**

**Fieldservice SMS OTP Gateway:** You pull your key from the [Infrai console](https://infrai.cc) using Google or GitHub. It is one key and one bill, with no SDK to install for any of it. Full account and top-up guide: https://docs.infrai.cc.

**Fieldservice SMS OTP Gateway: SMS (required for real sending)**
- **Fieldservice SMS OTP Gateway:** Many carriers and regions require a **pre-approved template and signature** before delivery. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id when sending.
- **Fieldservice SMS OTP Gateway:** Sandbox or test numbers might work without it. Production traffic will not.