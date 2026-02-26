# FOW Notes

## 2026-02-26: Panic Recovery Detection Is Route-Registration-Coupled

Observed behavior:
- `panic-recovery-middleware` stayed active even when panic recovery was implemented as a global server wrapper (`server.Handler = recovery(mux)`), and even when recovery middleware was present in a separate middleware package.

What was counter-intuitive:
- FOW recovery detection only credits recovery wrappers that are discoverable from route registration calls (`Handle`, `HandleFunc`, `Use`, etc.) in configured route files.
- A top-level handler chain that wraps the mux at server construction time was not detected.

Practical implication:
- Recovery middleware must appear in route-registration-reachable call paths to satisfy this rule, even if functionally equivalent global wrapping exists elsewhere.

Workaround applied:
- Added a top-level `recoveryMiddleware` and registered it via `rootMux.Handle("/", recoveryMiddleware(mux))`, then applied the remaining middleware stack around `rootMux`.

## 2026-02-26: Service Trace Rule Over-Targets Private Helper Methods

Observed behavior:
- `service-trace-span` flagged large numbers of private helper methods that are not service entrypoints (for example key builders, internal lookup helpers, and write helpers).

What was counter-intuitive:
- The rule is configured at `service` role level and, without `exclude_unexported`, it treats every private helper as requiring first-class span lifecycle instrumentation.
- This creates heavy noise and pushes instrumentation into low-value internals rather than service boundaries.

Practical implication:
- Teams spend effort adding spans to private plumbing code just to satisfy FOW, while exported service methods remain the meaningful observability boundary.

Workaround applied:
- Added `exclude_unexported: true` for `service-trace-span` to keep enforcement focused on service API methods.

## 2026-02-26: Wrapper Methods Need Explicit Transitive Trace Credit

Observed behavior:
- Thin exported wrapper methods that delegate to an already traced helper (for example `WatchLogService.LogWatch` -> `WatchLogService.logWatch`) were still reported by `service-trace-span`.

What was counter-intuitive:
- Trace signal did not automatically propagate across service method delegation without a transitive-credit mapping.

Practical implication:
- Legitimate wrapper APIs get flagged even when delegated execution is fully traced.

Workaround applied:
- Added explicit transitive trace credit for `services.WatchLogService.logWatch`.

## 2026-02-26: State-Change Audit Rule Flags Private Mutation Helpers

Observed behavior:
- `state-change-audit` generated a large set of findings on unexported helper methods (`create*`, `restore*`, `update*`) that are internal implementation details under exported service APIs.

What was counter-intuitive:
- Internal helper methods were enforced as first-class API operations, even when the public service method is the meaningful audit boundary.

Practical implication:
- Audit remediation effort shifts toward helper plumbing rather than user-visible state transitions.

Workaround applied:
- Added `exclude_unexported: true` for `state-change-audit` to keep coverage focused on exported service operations.

## 2026-02-26: Audit Signal Detection Needed Explicit Method Patterns

Observed behavior:
- Many exported service methods that already call `LogAuditWithMetadata` were still reported by `state-change-audit`.

What was counter-intuitive:
- Existing transitive-credit entries for `AuditService.LogAudit*` were not sufficient to recognize audit coverage across real callsites.

Practical implication:
- FOW reported missing audits despite explicit audit method invocations in service code.

Workaround applied:
- Added explicit go-analyzer `signals.audit` method patterns for `LogAuditWithMetadata`, `LogAudit`, and `LogModerationAudit`.

## 2026-02-26: `recordSpanError` Needed Explicit Metric Error Credit

Observed behavior:
- Many mutation methods recorded span errors consistently but still failed `error-rate-counter` / `state-change-metric`.

What was counter-intuitive:
- Span error recording alone was not treated as metric error coverage, and generic error metric calls needed explicit transitive credit.

Practical implication:
- Large numbers of false-positive mutation error/metric violations remained even with consistent error handling.

Workaround applied:
- Added `observability.RecordServiceError` and invoked it from `recordSpanError`.
- Added transitive credit for `services.recordSpanError` and `observability.RecordServiceError` as `metric_error`.
