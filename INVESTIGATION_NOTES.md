# Issue #140 Investigation Notes

## Bug Summary
The Admin Moderation panel shows "Loading pending users..." indefinitely and navigation is reportedly blocked.

## Investigation Findings

### Frontend Code Analysis (`frontend/src/components/admin/PendingUsers.svelte`)

**Current implementation:**
```javascript
let isLoading = false;  // Line 13 - starts as false
let pendingUsers: PendingUser[] = [];

onMount(() => {
  loadPendingUsers();  // Called on mount
});
```

**Issues identified:**

1. **`isLoading` starts as `false`** - This causes a brief "All caught up. No pending approvals right now." flash before showing "Loading pending users..." when the component mounts. This is confusing UX.

2. **No request timeout** - The `api.get('/admin/users')` call has no timeout. If the backend hangs, the loading state persists forever.

3. **No request cancellation on unmount** - If the component unmounts while the API call is in progress, there's no AbortController to cancel the request.

### Backend Code Analysis

**Endpoint:** `GET /api/v1/admin/users` → `ListPendingUsers` handler

**Route registration (`backend/cmd/server/main.go:217`):**
```go
mux.Handle("/api/v1/admin/users", middleware.RequireAdmin(redisConn)(http.HandlerFunc(adminHandler.ListPendingUsers)))
```

**Handler (`backend/internal/handlers/admin.go:36-51`):**
- Calls `h.userService.GetPendingUsers(r.Context())`
- Returns JSON array of pending users
- Has proper error handling

**Service (`backend/internal/services/user.go:263-292`):**
- Simple SQL query: `SELECT id, username, email, created_at FROM users WHERE approved_at IS NULL AND deleted_at IS NULL`
- Has proper error handling with context

**Middleware (`backend/internal/middleware/middleware.go:122-160`):**
- `RequireAdmin` checks session cookie, validates via Redis, checks `isAdmin` flag
- Returns proper error responses on failure
- Calls `next.ServeHTTP` on success

### Potential Root Causes

1. **Session/Redis timeout**: If Redis is slow or unavailable, the session validation in middleware could hang
2. **Database connection pool exhaustion**: If the DB pool is exhausted, queries would queue
3. **Network issue between frontend and backend**: Could cause fetch to hang
4. **CORS misconfiguration**: Though this typically causes errors, not hangs

### Recommended Fixes

#### 1. Fix initial loading state (HIGH PRIORITY)
Change `isLoading` to start as `true`:
```javascript
let isLoading = true;  // Show loading immediately
```

#### 2. Add request timeout (HIGH PRIORITY)
Add AbortController with timeout:
```javascript
const loadPendingUsers = async () => {
  isLoading = true;
  errorMessage = '';
  
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 10000); // 10s timeout
  
  try {
    const response = await fetch('/api/v1/admin/users', {
      signal: controller.signal,
      credentials: 'include',
    });
    // ... handle response
  } catch (error) {
    if (error.name === 'AbortError') {
      errorMessage = 'Request timed out. Please try again.';
    } else {
      errorMessage = error instanceof Error ? error.message : 'Failed to load pending users.';
    }
  } finally {
    clearTimeout(timeoutId);
    isLoading = false;
  }
};
```

#### 3. Add cleanup on unmount (MEDIUM PRIORITY)
```javascript
import { onMount, onDestroy } from 'svelte';

let abortController: AbortController | null = null;

onDestroy(() => {
  abortController?.abort();
});
```

#### 4. Backend: Add context timeout (MEDIUM PRIORITY)
In `ListPendingUsers`:
```go
ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
defer cancel()
pendingUsers, err := h.userService.GetPendingUsers(ctx)
```

### Files to Modify

1. `frontend/src/components/admin/PendingUsers.svelte` - Main fixes
2. `frontend/src/services/api.ts` - Optional: add global timeout support
3. `backend/internal/handlers/admin.go` - Optional: add context timeout

### Testing Notes

- Backend unit tests skip due to missing test database configuration
- Frontend tests in `frontend/src/components/admin/__tests__/PendingUsers.test.ts` mock the API
- Need to add tests for timeout and error scenarios

---
**Investigation by:** agent-1768983548-44286
**Date:** 2026-01-21
