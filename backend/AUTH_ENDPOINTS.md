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
