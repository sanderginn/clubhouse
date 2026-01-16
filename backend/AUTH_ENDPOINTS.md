# Authentication Endpoints

## User Registration

### Endpoint
```
POST /api/v1/auth/register
```

### Description
Registers a new user in the system. The user will be created in an unapproved state and requires admin approval before they can log in.

### Request Body
```json
{
  "username": "string",
  "email": "string",
  "password": "string"
}
```

### Request Validation Rules
- **Username:**
  - Required
  - 3-50 characters
  - Alphanumeric characters and underscores only
  - Must be unique
- **Email:**
  - Required
  - Valid email format (includes @ and domain with TLD)
  - Must be unique
- **Password:**
  - Required
  - Minimum 8 characters
  - Must contain uppercase letters, lowercase letters, and numbers

### Success Response (201 Created)
```json
{
  "id": "uuid",
  "username": "string",
  "email": "string",
  "message": "Registration successful. Please wait for admin approval."
}
```

### Error Responses

#### 400 Bad Request
```json
{
  "error": "Username is required",
  "code": "USERNAME_REQUIRED"
}
```

Possible error codes:
- `USERNAME_REQUIRED` - Username not provided
- `INVALID_USERNAME_LENGTH` - Username length not between 3-50 characters
- `INVALID_USERNAME_FORMAT` - Username contains invalid characters
- `EMAIL_REQUIRED` - Email not provided
- `INVALID_EMAIL` - Email format is invalid
- `INVALID_PASSWORD_LENGTH` - Password less than 8 characters
- `INVALID_PASSWORD_STRENGTH` - Password lacks required character types
- `INVALID_REQUEST` - Request body is malformed JSON

#### 409 Conflict
```json
{
  "error": "Username already exists",
  "code": "USERNAME_EXISTS"
}
```

Possible error codes:
- `USERNAME_EXISTS` - Username is already taken
- `EMAIL_EXISTS` - Email is already registered

#### 500 Internal Server Error
```json
{
  "error": "Failed to register user",
  "code": "REGISTRATION_FAILED"
}
```

### Example Usage

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "john_doe",
    "email": "john@example.com",
    "password": "SecurePass123"
  }'
```

### Notes
- Passwords are hashed using bcrypt (cost: 12) before storage
- Users are created with `approved_at = NULL` and cannot log in until approved by an admin
- Usernames and emails are case-sensitive
- All fields are required

---

## User Login

### Endpoint
```
POST /api/v1/auth/login
```

### Description
Authenticates a user with email and password. On successful authentication, creates a Redis session and sets an httpOnly secure cookie.

### Request Body
```json
{
  "email": "string",
  "password": "string"
}
```

### Request Validation Rules
- **Email:**
  - Required
  - Valid email format (includes @ and domain with TLD)
- **Password:**
  - Required
  - Non-empty string

### Success Response (200 OK)
```json
{
  "id": "uuid",
  "username": "string",
  "email": "string",
  "is_admin": boolean,
  "message": "Login successful"
}
```

**Cookies Set:**
- `session_id`: Session identifier stored in Redis (httpOnly, Secure, SameSite=Lax)
  - Valid for 30 days
  - Automatically removed on logout

### Error Responses

#### 400 Bad Request
```json
{
  "error": "email is required",
  "code": "EMAIL_REQUIRED"
}
```

Possible error codes:
- `EMAIL_REQUIRED` - Email not provided
- `INVALID_EMAIL` - Email format is invalid
- `PASSWORD_REQUIRED` - Password not provided
- `INVALID_REQUEST` - Request body is malformed JSON

#### 401 Unauthorized
```json
{
  "error": "invalid email or password",
  "code": "INVALID_CREDENTIALS"
}
```

#### 403 Forbidden
```json
{
  "error": "user not approved",
  "code": "USER_NOT_APPROVED"
}
```

The user exists but has not been approved by an admin yet.

#### 500 Internal Server Error
```json
{
  "error": "Failed to login",
  "code": "LOGIN_FAILED"
}
```

### Example Usage

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john@example.com",
    "password": "SecurePass123"
  }'
```

### Notes
- Passwords are verified using bcrypt comparison (never expose hash)
- Sessions are stored in Redis with a 30-day TTL
- The session_id cookie is httpOnly and Secure to prevent XSS attacks
- A user must be approved (`approved_at != NULL`) to successfully log in
- Invalid credentials (wrong password or user not found) return the same error message for security

---

## User Logout

### Endpoint
```
POST /api/v1/auth/logout
```

### Description
Logs out a user by invalidating their session and clearing the session cookie. The user must have an active session to successfully logout.

### Request Body
No request body is required. The session is identified via the `session_id` cookie.

### Success Response (200 OK)
```json
{
  "message": "Logout successful"
}
```

**Cookies Cleared:**
- `session_id`: Session cookie is cleared (MaxAge set to -1)

### Error Responses

#### 401 Unauthorized
```json
{
  "error": "No active session found",
  "code": "NO_SESSION"
}
```

The user does not have an active session (no session_id cookie present).

#### 405 Method Not Allowed
```json
{
  "error": "Only POST requests are allowed",
  "code": "METHOD_NOT_ALLOWED"
}
```

Request was made with an HTTP method other than POST.

#### 500 Internal Server Error
```json
{
  "error": "Failed to logout",
  "code": "LOGOUT_FAILED"
}
```

An unexpected error occurred while deleting the session.

### Example Usage

```bash
curl -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Cookie: session_id=<session_id_value>"
```

### Notes
- The logout endpoint requires an active session cookie to succeed
- The session is deleted from Redis immediately upon logout
- The session_id cookie is cleared on the client side (MaxAge: -1)
- After logout, the user will be unauthenticated and must log in again
- No request body is needed; authentication is cookie-based
