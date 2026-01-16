# Admin Endpoints Documentation

This document describes the admin approval endpoints for managing user registration.

## Overview

The admin user approval flow allows admins to approve or reject pending user registrations. All endpoints require authentication and admin privileges.

## Authentication

All admin endpoints require:
- Valid session cookie (`session_id`)
- User must have `is_admin = true`

If authentication fails, the endpoint returns:
```json
{
  "error": "Admin access required",
  "code": "ADMIN_REQUIRED"
}
```

---

## Endpoints

### 1. List Pending Users

**Endpoint:** `GET /api/v1/admin/users`

**Description:** Retrieve all users pending admin approval

**Authentication:** Required (Admin)

**Query Parameters:** None

**Response (200 OK):**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "username": "john_doe",
    "email": "john@example.com",
    "created_at": "2026-01-16T10:30:00Z"
  },
  {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "username": "jane_smith",
    "email": "jane@example.com",
    "created_at": "2026-01-16T11:15:00Z"
  }
]
```

**Error Responses:**

- `401 Unauthorized` (NO_SESSION): No session cookie provided
- `401 Unauthorized` (INVALID_SESSION): Session expired or invalid
- `403 Forbidden` (ADMIN_REQUIRED): User is not an admin
- `500 Internal Server Error` (FETCH_FAILED): Failed to fetch pending users

**Example cURL:**
```bash
curl -X GET http://localhost:8080/api/v1/admin/users \
  -H "Cookie: session_id=<SESSION_ID>"
```

---

### 2. Approve User

**Endpoint:** `PATCH /api/v1/admin/users/{id}/approve`

**Description:** Approve a pending user and allow them to log in

**Authentication:** Required (Admin)

**URL Parameters:**
- `id` (UUID): The ID of the user to approve

**Request Body:** Empty

**Response (200 OK):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "username": "john_doe",
  "email": "john@example.com",
  "message": "User approved successfully"
}
```

**Error Responses:**

- `400 Bad Request` (INVALID_USER_ID): Invalid UUID format
- `401 Unauthorized` (NO_SESSION): No session cookie provided
- `401 Unauthorized` (INVALID_SESSION): Session expired or invalid
- `403 Forbidden` (ADMIN_REQUIRED): User is not an admin
- `404 Not Found` (USER_NOT_FOUND): User does not exist
- `409 Conflict` (USER_ALREADY_APPROVED): User is already approved
- `410 Gone` (USER_DELETED): User has been deleted
- `500 Internal Server Error` (APPROVAL_FAILED): Failed to approve user

**Example cURL:**
```bash
curl -X PATCH http://localhost:8080/api/v1/admin/users/550e8400-e29b-41d4-a716-446655440000/approve \
  -H "Cookie: session_id=<SESSION_ID>"
```

**Effect:**
- Sets `approved_at` timestamp to current time
- Sets `updated_at` timestamp to current time
- User can now log in with email and password

---

### 3. Reject User

**Endpoint:** `DELETE /api/v1/admin/users/{id}`

**Description:** Reject and permanently delete a pending user

**Authentication:** Required (Admin)

**URL Parameters:**
- `id` (UUID): The ID of the user to reject

**Request Body:** Empty

**Response (200 OK):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "message": "User rejected and deleted successfully"
}
```

**Error Responses:**

- `400 Bad Request` (INVALID_USER_ID): Invalid UUID format
- `401 Unauthorized` (NO_SESSION): No session cookie provided
- `401 Unauthorized` (INVALID_SESSION): Session expired or invalid
- `403 Forbidden` (ADMIN_REQUIRED): User is not an admin
- `404 Not Found` (USER_NOT_FOUND): User does not exist
- `409 Conflict` (USER_ALREADY_APPROVED): Cannot reject an approved user
- `500 Internal Server Error` (REJECTION_FAILED): Failed to reject user

**Example cURL:**
```bash
curl -X DELETE http://localhost:8080/api/v1/admin/users/550e8400-e29b-41d4-a716-446655440000 \
  -H "Cookie: session_id=<SESSION_ID>"
```

**Effect:**
- Hard deletes the user from the database (cannot be restored)
- Can only reject users that have NOT been approved yet
- All related data (posts, comments, etc.) will cascade delete if foreign key constraints are set

---

## Workflow

### User Registration Flow

1. User registers via `POST /api/v1/auth/register`
   - User created with `approved_at = NULL`
   - Response message: "Registration successful. Please wait for admin approval."

2. User cannot log in yet
   - Login attempt returns: `403 Forbidden` with code `USER_NOT_APPROVED`

3. Admin lists pending users via `GET /api/v1/admin/users`

4. Admin approves user via `PATCH /api/v1/admin/users/{id}/approve`
   - User now has `approved_at = <timestamp>`

5. User can now log in via `POST /api/v1/auth/login`
   - System checks `approved_at IS NOT NULL`
   - Session is created and cookie is set

### User Rejection Flow

1. Admin lists pending users via `GET /api/v1/admin/users`

2. Admin rejects user via `DELETE /api/v1/admin/users/{id}`
   - User is permanently deleted
   - Cannot be undone

3. User can register again with same email if desired

---

## Testing

### Get Session Token (for testing)

First, create an admin user and log in:

```bash
# Register (as an existing admin, this must be done via direct DB or during setup)
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"AdminPassword123"}'
```

Extract the `session_id` cookie from response headers.

### Test Listing Pending Users

```bash
curl -X GET http://localhost:8080/api/v1/admin/users \
  -H "Cookie: session_id=<SESSION_ID>" \
  -H "Content-Type: application/json"
```

### Test Approving a User

```bash
curl -X PATCH http://localhost:8080/api/v1/admin/users/<USER_ID>/approve \
  -H "Cookie: session_id=<SESSION_ID>" \
  -H "Content-Type: application/json"
```

### Test Rejecting a User

```bash
curl -X DELETE http://localhost:8080/api/v1/admin/users/<USER_ID> \
  -H "Cookie: session_id=<SESSION_ID>" \
  -H "Content-Type: application/json"
```

---

## Notes

- All timestamps are in UTC ISO 8601 format
- Approved users can immediately log in
- Rejected users are permanently deleted and cannot be restored
- Approved users cannot be rejected (only deletion via audit would be possible, not implemented yet)
- The endpoint uses HTTP semantics: GET for retrieval, PATCH for state changes, DELETE for removal
