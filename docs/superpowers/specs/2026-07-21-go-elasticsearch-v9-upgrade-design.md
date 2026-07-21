# go-elasticsearch v9 Upgrade and pkg/es Review Design

## Goal

Upgrade `github.com/elastic/go-elasticsearch/v8` from `v8.19.0` to the latest confirmed v9 release, `github.com/elastic/go-elasticsearch/v9 v9.4.2`, while preserving the observable behavior and public API of `pkg/es`. Review `pkg/es` for concrete correctness, reliability, and security weaknesses.

## Scope

- Update the Go module dependency and all v8 import paths under `pkg/es`.
- Adapt code only where the v9 API requires it.
- Review client initialization, concurrent access, request retries, timeouts, response-body lifecycle, Bulk error handling, pagination behavior, and logging of sensitive or oversized data.
- Add regression tests before fixing any clear, low-risk defect found during the review.
- Report larger architectural risks or behavior-changing improvements instead of folding them into this migration.

The review excludes `pkg/log`, `pkg/pcm`, and unrelated repository code except where needed to compile or test `pkg/es`.

## Compatibility Requirements

- Existing exported names and function signatures in `pkg/es` remain unchanged unless v9 makes that impossible.
- Existing callers continue to use the same request and response structures.
- Retry counts, timeout values, and pagination semantics remain unchanged unless a test demonstrates a correctness or safety defect and the fix is narrowly scoped.
- Elasticsearch integration tests remain opt-in through `ES_TEST_ADDR`; default verification must not require a live cluster.

## Implementation Approach

1. Establish a clean baseline with the current dependency and the existing test suite.
2. Add a dependency/import compatibility check by migrating to v9 and compiling the package tests. Treat compiler failures as the migration's initial failing signal.
3. Make the smallest v9 API adaptations needed to restore compilation and existing tests.
4. Review each `pkg/es` execution path. For every actionable defect, first add a focused test that fails for the expected reason, then apply the minimal fix and rerun the focused test.
5. Run formatting, package tests, repository tests, static analysis, and dependency consistency checks.

## Review Classification

Findings will be classified as:

- Critical or high: exploitable exposure, data corruption, deadlock, panic on ordinary inputs, or systemic request failure.
- Medium: incorrect retry/error behavior, resource leaks, unsafe concurrency, or incorrect pagination under realistic conditions.
- Low: observability gaps, brittle edge cases, confusing errors, or missing defensive validation.

Only findings supported by a concrete code path and reproducible reasoning will be reported. Clear low-risk findings may be fixed in this change; broad refactors and compatibility-breaking changes will be listed as remaining work.

## Verification

- `go test ./pkg/es`
- `go test ./...`
- `go vet ./...`
- `go mod tidy` followed by a clean dependency diff check
- Search confirms no remaining `github.com/elastic/go-elasticsearch/v8` imports.

Completion requires all non-integration tests to pass and the final report to distinguish fixed findings from remaining risks.
