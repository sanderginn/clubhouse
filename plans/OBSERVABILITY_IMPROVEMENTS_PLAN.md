# Observability Improvements Plan (Golden State)

## Objective

Bring Clubhouse to a zero-violation Fog of War (FOW) state and keep it there with automated guardrails in local development (`prek`) and CI.

Golden state definition:

1. `fow analyze --fail-on warning -f json` reports `0` violations.
2. `fow analyze --fail-on never -f json -s info` reports `0` violations.
3. All flows are `complete` at 100% effective coverage.
4. No unmanaged rule suppressions or broad exclusions without documented rationale in `.fow.yml`.

## Current Baseline (2026-02-26 rerun)

Command baseline:

```bash
fow analyze --fail-on never -f json
fow analyze --fail-on never -f json -s info
```

Counts:

| Rule | Severity | Count |
|---|---|---:|
| `state-change-audit` | warning | 115 |
| `service-trace-span` | error | 102 |
| `state-change-metric` | info | 74 |
| `error-rate-counter` | warning | 45 |
| `external-call-latency-histogram` | warning | 22 |
| `external-call-trace-span` | warning | 22 |
| `handler-logging` | error | 17 |
| `context-background-in-traced-code` | error | 6 |
| `flow-context-passthrough` | warning | 1 |
| `flow-trace-continuity` | warning | 1 |
| `panic-recovery-middleware` | error | 1 |
| `resource-saturation-gauge` | warning | 1 |
| `unstructured-logging` | warning | 1 |

Flow status:

- `114` total flows
- `113` complete
- `1` broken (`ANY /api/v1/ws -> wsHandler.HandleWS`)

## Delivery Strategy

Implement in phases so signal quality improves immediately while reducing rework.

## Phase 0: Instrumentation Contract And Tooling Prep

Goal: establish one consistent implementation pattern so remediation is mechanical and testable.

1. Define canonical patterns for:
   - Service span lifecycle (`Start`, attributes, `recordSpanError`, `End`).
   - State-change audit emission (action names and metadata shape).
   - Error-rate counting (`metric_error`) on all observable error exits.
   - Business activity metrics (`state-change-metric`) with stable names/labels.
2. Create helper wrappers where needed so teams do not hand-roll instrumentation in every method.
3. Write/update tests for helpers and one representative service per domain.

Exit criteria:

- Approved instrumentation template documented in code comments and reused.
- No new domain-specific ad hoc instrumentation style introduced.

## Phase 1: Eliminate Structural P0/P1 Gaps First

Goal: remove violations that represent cross-cutting safety or trace correctness problems.

1. `panic-recovery-middleware` (`1`)
   - Add panic recovery middleware to the global chain in `backend/cmd/server/main.go`.
   - Ensure panic path logs structured error and returns standard error response.
2. `context-background-in-traced-code` (`6`)
   - Replace `context.Background()` in traced execution paths with request/parent context propagation.
   - Current hits:
     - `backend/internal/handlers/pubsub.go`
     - `backend/internal/services/config.go` (2)
     - `backend/internal/services/links/metadata.go`
     - `backend/internal/services/post.go`
     - `backend/internal/services/push.go`
3. Broken flow (`HandleWS`)
   - Add spans for `WebSocketHandler.writeLoop` and `WebSocketHandler.pingLoop`.
   - Fix context propagation into async boundary (`pingLoop`).
   - Confirm both `flow-trace-continuity` and `flow-context-passthrough` reach zero.

Exit criteria:

- `panic-recovery-middleware=0`
- `context-background-in-traced-code=0`
- Broken flows: `1 -> 0`

## Phase 2: External I/O Instrumentation Completion

Goal: enforce full outbound dependency observability.

1. Resolve `external-call-trace-span` (`22`)
2. Resolve `external-call-latency-histogram` (`22`)
3. Apply one reusable outbound-call helper where possible (backend links clients + frontend API paths).
4. Ensure frontend remaining methods (`ApiClient.prefetchCsrfToken`, `ApiClient.uploadImage`) get explicit span and latency recording.

Target files include:

- `backend/internal/services/links/*.go`
- `frontend/src/services/api.ts`

Exit criteria:

- `external-call-trace-span=0`
- `external-call-latency-histogram=0`

## Phase 3: Handler Logging Completion

Goal: remove missing-log handler violations (`17`) with meaningful structured logs.

1. Add structured logs on fail/edge paths (not noisy success spam).
2. Prioritize websocket helper handlers and auth/watchlist/saved_recipe/upload endpoints currently flagged.
3. Verify no `fmt.Println`/`console.*` regressions.

Primary files:

- `backend/internal/handlers/websocket.go`
- `backend/internal/handlers/auth.go`
- `backend/internal/handlers/saved_recipe.go`
- `backend/internal/handlers/watchlist.go`
- `backend/internal/handlers/uploads.go`

Exit criteria:

- `handler-logging=0`
- `unstructured-logging=0`

## Phase 4: Service-Level Debt Burn Down (Largest Workstream)

Goal: close service observability debt at source: traces, audit, error metrics, business metrics.

1. `service-trace-span` (`102`)
2. `state-change-audit` (`115`)
3. `error-rate-counter` (`45`)
4. `state-change-metric` (`74`)

Execution approach:

1. Triage each violation as:
   - Legitimate missing instrumentation.
   - Rule scope mismatch (rare; only with explicit rationale).
2. Fix by domain batches to reduce context switching:
   - `user`, `notification`, `reaction`, `saved_recipe`, `watch_log`, `read_log`
   - then `bookshelf`, `cook_log`, `post`, `push`, `totp`, `comment`, `session`, others
3. For state-changing operations, ensure all four signals are present where applicable:
   - trace span
   - audit event
   - error-rate metric
   - business activity metric
4. Add/expand tests per domain:
   - audit writes for mutation paths
   - error-path metric calls
   - span creation on key service entrypoints

Important policy for helper methods:

1. Keep instrumentation on methods that independently perform I/O or state mutation.
2. Avoid meaningless duplication on pure private helpers by tightening rule scope only when justified and documented.
3. Any rule-scope adjustment must include before/after FOW evidence and explicit note in `.fow.yml`.

Exit criteria:

- `service-trace-span=0`
- `state-change-audit=0`
- `error-rate-counter=0`
- `state-change-metric=0`

## Phase 5: Resource Saturation Gauge Completion

Goal: close remaining saturation signal gap (`1`).

1. Add explicit Redis pool saturation metrics or recognized instrumentation path for `backend/internal/cache/redis.go`.
2. Validate FOW detects saturation gauge and no longer flags redis init path.

Exit criteria:

- `resource-saturation-gauge=0`

## Validation Workflow During Execution

Run after each domain batch:

```bash
fow analyze --fail-on never -f json > /tmp/fow.json
jq -r '.violations | group_by(.rule) | map({rule:.[0].rule,count:length}) | sort_by(-.count)[] | "\(.rule)\t\(.count)"' /tmp/fow.json
```

Run full completeness check:

```bash
fow analyze --fail-on warning -f json
fow analyze --fail-on never -f json -s info
```

## Guardrails Integration (Task + prek + Buildkite)

Add explicit FOW tasks:

1. `task obs:fow`
   - Runs full repo: `fow analyze --fail-on warning -f json`
2. `task obs:fow-info`
   - Runs full repo: `fow analyze --fail-on never -f json -s info`
3. `task obs:fow-changed`
   - Runs changed-only: `fow analyze --changed-only --fail-on warning -f json`
4. `task ci:fow`
   - CI wrapper for full strict check and summary output

Pre-commit (`.pre-commit-config.yaml`) integration:

1. Add local hook `fow-changed`:
   - Entry: `task obs:fow-changed`
   - Scope: backend + frontend source + `.fow.yml`
2. Keep existing lint/test hooks unchanged.
3. Ensure fast failure messaging points to rule/file/function and fix hint.

### TODO when `fow` is available from Buildkite context, skip for now
Buildkite (`.buildkite/pipeline.steps.yml`) integration:

1. Add step: `:mag: FOW observability`
2. Run `task ci:fow`
3. Make step required for merge to `main`.

## Ratcheting To Permanent Golden Enforcement

Because we are explicitly targeting zero now, use an aggressive ratchet:

1. Immediately enforce no-regression on this branch while fixing debt.
2. Once counts hit zero for warning/error, switch CI to strict:
   - `fow analyze --fail-on warning -f json`
3. Once info also hits zero, require:
   - `fow analyze --fail-on never -f json -s info`
   - plus a script assertion that total violations is zero.
4. Keep `obs:fow-changed` in `prek` permanently to prevent reintroduction.

## Definition Of Done

1. All FOW rules report zero violations at warning/error/info.
2. All flows are complete.
3. `task ci:fow` is wired in Buildkite and required.
4. `prek` runs `obs:fow-changed`.
5. README and AGENTS guidance include the new FOW guardrail commands.
