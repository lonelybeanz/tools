# go-elasticsearch v9 Upgrade and pkg/es Review Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade `pkg/es` to `github.com/elastic/go-elasticsearch/v9 v9.4.2`, preserve its public behavior, fix narrowly reproducible defects, and report remaining reliability and security risks.

**Architecture:** Keep the existing low-level `elasticsearch.Client` and `esapi.Response` integration instead of introducing the v9 typed client. Migrate module/import paths surgically, then use focused tests around the existing request callback and HTTP test servers to prove robustness fixes without requiring a live Elasticsearch cluster.

**Tech Stack:** Go 1.25, `github.com/elastic/go-elasticsearch/v9 v9.4.2`, standard `testing`/`httptest`, Go modules.

---

## File Map

- Modify `go.mod` and `go.sum`: replace the v8 client and transport dependency graph with v9.4.2 and its required transitives.
- Modify `pkg/es/esclient.go`: update imports and defensively handle a request callback returning no response.
- Modify `pkg/es/esclient_retry_test.go`: update imports and add the nil-response regression test.
- Modify `pkg/es/page.go`: update imports and make `_source` decoding safe for missing or null values.
- Create `pkg/es/page_test.go`: cover `QueryStream` response decoding through a local HTTP server.
- Modify `pkg/es/save.go`: update imports only unless compilation proves a v9 API adaptation is required.
- Modify `pkg/es/esclient_test.go`: update imports and remove hard-coded integration-test credentials in favor of optional environment variables.
- Review `pkg/es/log.go`, `pkg/es/save.go`, and `pkg/es/page.go`: record evidenced risks not suitable for a surgical compatibility patch.

### Task 1: Establish the Current Baseline

**Files:**
- Inspect: `go.mod`
- Test: `pkg/es/*_test.go`

- [ ] **Step 1: Confirm the worktree is clean except for the committed specification and plan**

Run: `rtk git status --short`

Expected: no uncommitted production or test changes.

- [ ] **Step 2: Run the existing package tests before changing the dependency**

Run: `rtk go test ./pkg/es`

Expected: PASS; `TestTime` skips when `ES_TEST_ADDR` is unset.

- [ ] **Step 3: Run the repository baseline**

Run: `rtk go test ./...`

Expected: PASS. If it fails, record the exact pre-existing failure and do not attribute it to v9.

### Task 2: Migrate the Elasticsearch Module and Imports

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `pkg/es/esclient.go`
- Modify: `pkg/es/esclient_retry_test.go`
- Modify: `pkg/es/esclient_test.go`
- Modify: `pkg/es/page.go`
- Modify: `pkg/es/save.go`

- [ ] **Step 1: Replace all import paths with v9**

Change both imports wherever present:

```go
"github.com/elastic/go-elasticsearch/v9"
"github.com/elastic/go-elasticsearch/v9/esapi"
```

- [ ] **Step 2: Select the exact v9 dependency**

Run: `rtk go get github.com/elastic/go-elasticsearch/v9@v9.4.2`

Expected: `go.mod` requires `github.com/elastic/go-elasticsearch/v9 v9.4.2`; v8 is removed after tidy.

- [ ] **Step 3: Compile the migrated package to expose API incompatibilities**

Run: `rtk go test ./pkg/es`

Expected initial signal: either PASS (the used low-level API is source-compatible) or compile errors naming the exact v9 adaptation required.

- [ ] **Step 4: Make only compiler-required v9 API adaptations**

Preserve these public/internal shapes unless compilation requires otherwise:

```go
var EsClient *elasticsearch.Client

func DoESRequest(
    ctx context.Context,
    req func(context.Context, *elasticsearch.Client) (*esapi.Response, error),
) ([]byte, error)
```

- [ ] **Step 5: Normalize the module graph**

Run: `rtk go mod tidy`

Expected: no v8 client or v8 transport remains; only expected v9 and transitive changes appear in `go.mod`/`go.sum`.

- [ ] **Step 6: Verify the migration**

Run: `rtk go test ./pkg/es`

Expected: PASS.

- [ ] **Step 7: Commit the dependency migration**

```bash
rtk git add go.mod go.sum pkg/es/esclient.go pkg/es/esclient_retry_test.go pkg/es/esclient_test.go pkg/es/page.go pkg/es/save.go
rtk git commit -m "build: upgrade go-elasticsearch to v9"
```

### Task 3: Guard DoESRequest Against a Nil Successful Response

**Files:**
- Modify: `pkg/es/esclient_retry_test.go`
- Modify: `pkg/es/esclient.go`

- [ ] **Step 1: Write the failing regression test**

Add a test that installs an empty client, invokes the callback once, and returns `(nil, nil)`:

```go
func TestDoESRequestRejectsNilResponse(t *testing.T) {
    esMutex.Lock()
    oldClient := EsClient
    EsClient = &elasticsearch.Client{}
    esMutex.Unlock()
    defer func() {
        esMutex.Lock()
        EsClient = oldClient
        esMutex.Unlock()
    }()

    _, err := DoESRequest(context.Background(), func(context.Context, *elasticsearch.Client) (*esapi.Response, error) {
        return nil, nil
    })
    if err == nil || !strings.Contains(err.Error(), "nil response") {
        t.Fatalf("expected nil response error, got %v", err)
    }
}
```

- [ ] **Step 2: Run the focused test and confirm RED**

Run: `rtk go test ./pkg/es -run '^TestDoESRequestRejectsNilResponse$'`

Expected: FAIL with a nil-pointer panic because the callback contract is not currently checked.

- [ ] **Step 3: Add the minimal guard**

Immediately after the callback returns without an error:

```go
if res == nil {
    return nil, errors.New("es request returned nil response")
}
```

- [ ] **Step 4: Run focused and package tests**

Run: `rtk go test ./pkg/es -run '^TestDoESRequestRejectsNilResponse$'`

Expected: PASS.

Run: `rtk go test ./pkg/es`

Expected: PASS.

- [ ] **Step 5: Commit the fix**

```bash
rtk git add pkg/es/esclient.go pkg/es/esclient_retry_test.go
rtk git commit -m "fix: reject empty Elasticsearch responses"
```

### Task 4: Prevent QueryStream Panics on Missing `_source`

**Files:**
- Create: `pkg/es/page_test.go`
- Modify: `pkg/es/page.go`

- [ ] **Step 1: Write an HTTP-backed failing regression test**

Create a local `httptest.Server` returning a valid search response whose hit has `"_source": null`, temporarily install a v9 client configured with the server URL, and call `QueryStream`:

```go
func TestQueryStreamHandlesNullSource(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        _, _ = io.WriteString(w, `{"hits":{"total":{"value":1},"hits":[{"_id":"1","_source":null,"sort":[1]}]}}`)
    }))
    defer server.Close()

    client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}})
    if err != nil {
        t.Fatal(err)
    }
    esMutex.Lock()
    oldClient := EsClient
    EsClient = client
    esMutex.Unlock()
    defer func() {
        esMutex.Lock()
        EsClient = oldClient
        esMutex.Unlock()
    }()

    _, err = QueryStream(context.Background(), StreamRequest{
        Index: "idx", PageSize: 20,
        Filters: map[string]interface{}{"match_all": map[string]interface{}{}},
        Sort: []map[string]interface{}{{"_id": "asc"}},
    })
    if err == nil || !strings.Contains(err.Error(), "missing _source") {
        t.Fatalf("expected missing _source error, got %v", err)
    }
}
```

- [ ] **Step 2: Run the focused test and confirm RED**

Run: `rtk go test ./pkg/es -run '^TestQueryStreamHandlesNullSource$'`

Expected: FAIL with a panic from `h.Source.(map[string]interface{})`.

- [ ] **Step 3: Decode `_source` into its real required type and validate it**

Change the anonymous hit type and loop to:

```go
Source map[string]interface{} `json:"_source"`
```

```go
for _, h := range hits {
    if h.Source == nil {
        return nil, fmt.Errorf("hit %q is missing _source", h.Id)
    }
    h.Source["_id"] = h.Id
    resp.List = append(resp.List, h.Source)
}
```

- [ ] **Step 4: Run focused and package tests**

Run: `rtk go test ./pkg/es -run '^TestQueryStreamHandlesNullSource$'`

Expected: PASS.

Run: `rtk go test ./pkg/es`

Expected: PASS.

- [ ] **Step 5: Commit the fix**

```bash
rtk git add pkg/es/page.go pkg/es/page_test.go
rtk git commit -m "fix: validate Elasticsearch search sources"
```

### Task 5: Remove Hard-Coded Integration Credentials

**Files:**
- Modify: `pkg/es/esclient_test.go`

- [ ] **Step 1: Replace literals with opt-in environment configuration**

Use empty defaults so unauthenticated test clusters still work:

```go
cfg := elasticsearch.Config{
    Addresses: []string{addr},
    Username:  os.Getenv("ES_TEST_USERNAME"),
    Password:  os.Getenv("ES_TEST_PASSWORD"),
}
```

- [ ] **Step 2: Confirm the old credential literals are absent**

Run: `rtk rg -n 'Username:\s*"admin"|Password:\s*"1@in"' pkg/es`

Expected: no matches.

- [ ] **Step 3: Run the package tests**

Run: `rtk go test ./pkg/es`

Expected: PASS with the live-cluster test skipped unless explicitly enabled.

- [ ] **Step 4: Commit the test hardening**

```bash
rtk git add pkg/es/esclient_test.go
rtk git commit -m "test: read Elasticsearch credentials from environment"
```

### Task 6: Complete the pkg/es Logic Review

**Files:**
- Review: `pkg/es/esclient.go`
- Review: `pkg/es/page.go`
- Review: `pkg/es/save.go`
- Review: `pkg/es/log.go`
- Review: `pkg/es/*_test.go`

- [ ] **Step 1: Trace each external-input and lifecycle path**

Check and record exact file/line evidence for:

- TLS verification and credential transport.
- Client initialization/reinitialization and background health goroutines.
- Retry eligibility, retry backoff, full-payload Bulk retries, and idempotency.
- Response-body reads, drains, and closes on every exit path.
- Pagination input validation, cursor direction, sorting, and response decoding.
- Full Bulk payload logging, queue capacity, dropped-message behavior, and sensitive-data exposure.
- Global test/client/logger state and race-test behavior.

- [ ] **Step 2: Separate code changes from report-only findings**

Fix only issues that have a focused failing test, preserve public behavior, and require a small local change. Record behavior-changing items such as enabling TLS verification, redesigning Bulk partial retries, changing compensation logging, or making client startup cancellable as remaining risks with severity and remediation.

- [ ] **Step 3: Run the race detector for pkg/es**

Run: `rtk go test -race ./pkg/es`

Expected: PASS. If it exposes a race, capture the full access paths; add a focused test/fix only if it remains within the approved surgical scope.

### Task 7: Final Verification and Review Report

**Files:**
- Verify: `go.mod`
- Verify: `go.sum`
- Verify: `pkg/es/*.go`

- [ ] **Step 1: Format changed Go files**

Run: `rtk gofmt -w pkg/es/esclient.go pkg/es/esclient_retry_test.go pkg/es/esclient_test.go pkg/es/page.go pkg/es/page_test.go pkg/es/save.go`

Expected: exit 0.

- [ ] **Step 2: Verify no v8 imports or dependency entries remain**

Run: `rtk rg -n 'github\.com/elastic/go-elasticsearch/v8|elastic-transport-go/v8' go.mod go.sum pkg/es`

Expected: no matches.

- [ ] **Step 3: Run all package and repository checks freshly**

Run: `rtk go test ./pkg/es`

Expected: PASS.

Run: `rtk go test -race ./pkg/es`

Expected: PASS.

Run: `rtk go test ./...`

Expected: PASS.

Run: `rtk go vet ./...`

Expected: PASS.

- [ ] **Step 4: Check module and patch cleanliness**

Run: `rtk go mod tidy`

Run: `rtk git diff --check`

Run: `rtk git status --short`

Expected: tidy makes no additional changes; diff check exits 0; status contains only intentional implementation changes, or is clean if all task commits were made.

- [ ] **Step 5: Produce the final report**

Report:

- exact v9 version and migration files;
- tests, race detector, vet, and repository verification results;
- fixed findings with severity and code evidence;
- remaining findings ordered by severity, with impact and minimal remediation;
- whether the optional live Elasticsearch integration test was run or skipped.
