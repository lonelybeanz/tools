# pkg/es reliability hardening design

## Scope

Harden `pkg/es` without changing the existing TLS certificate behavior. Bulk action metadata may contain an `_index` different from the `indexName` argument; this is intentional multi-index behavior and must remain supported.

## Retry policy

The Bulk path owns one bounded, context-aware retry loop. Each operation may be sent at most three times; no nested path may produce 3x3 sends. `DoESRequest` itself becomes single-attempt because its callback API cannot prove that an arbitrary request is replay-safe or reconstruct its body.

Retry only failures that are explicitly known to be temporary:

- Bulk item responses with status 429, 502, 503, or 504. This status list is complete; an Elasticsearch error type cannot broaden it.
- Dial or connection-acquisition failures represented by a `net.OpError` whose operation is `dial`, before request bytes can be written.

Do not retry top-level HTTP errors, timeout, EOF, response-read failure, connection reset after write, or any other ambiguous request outcome. Do not retry any other item status, including 408, 409, and 500. Every attempt constructs a fresh reader.

Parse Bulk NDJSON into positional operation units: action plus source for `index`, `create`, and `update`; action only for `delete`. Preserve raw lines and all action metadata, including a differing `_index`. Reject malformed NDJSON and response/item-count mismatches before retrying. Retry only failed allowlisted units and accumulate permanent failures while the retriable subset completes. Return a combined `BulkResponseError`; preserve `errors.Is(err, ErrVersionConflict)` when every final failure is a version conflict and preserve `errors.Is(err, ErrBulkRetriable)` while retriable failures remain exhausted.

Keep `SaveAndRetry` as a `context.Background()` compatibility wrapper and add exported `SaveAndRetryContext(context.Context, string, bytes.Buffer)`. Bulk is the only automatic retry owner. Generic `DoESRequest` callers receive the first request or response error without replay; callers that need retries must implement an operation-specific, explicitly replay-safe policy.

## Resource and lifecycle limits

- Replace default full Bulk payload logging with metadata only: byte count, line count, and operation count.
- Bound individual asynchronous log messages to 64 KiB. Oversized messages are dropped, increment the existing dropped-message counter, and never enter the queue.
- Limit every Elasticsearch success or error response body to 32 MiB. Read at most limit+1 bytes; accept exactly the limit, but close and return sentinel `ErrESResponseTooLarge` without retry at limit+1.
- Make the health-check loop single-instance and cancellable. `StopEsClientHealthCheck()` is idempotent. `EsClientStart` cancels and waits for the prior loop before starting a replacement, so checks do not overlap; a stopped monitor can be restarted.
- Add `StreamRequest.ExactTotal bool` with JSON tag `exact_total` and `StreamResponse.TotalRelation string` with tag `total_relation,omitempty`. Exact mode sends `track_total_hits: true`; default mode sends `track_total_hits: 10000`. Populate `Total` and relation (`eq` or `gte`) from Elasticsearch; a missing total produces zero and an empty relation.

## Compatibility

Existing exported entry points remain available. TLS continues using the current `InsecureSkipVerify` setting. Bulk requests without a path index and requests whose action `_index` differs from the path index remain valid.

## Verification

Add focused tests first for retry classification, partial Bulk retry selection, mixed failures, payload logging, message-size limits, response-size limits, health-loop replacement/stop, and total-hit behavior. Then run formatting, `go test ./pkg/es`, race tests, repository tests, and `go vet ./pkg/es`.
