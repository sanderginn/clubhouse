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
