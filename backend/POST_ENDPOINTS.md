# Post Endpoints Documentation

This document describes the post management endpoints for creating, retrieving, and deleting posts.

## Overview

The post endpoints allow users to create posts in sections, retrieve post details, and delete their own posts. Admins can also delete any post for moderation purposes.

## Authentication

All endpoints except listing require:
- Valid session cookie (`session_id`)
- User must be approved (status: approved)

## Endpoints

### 1. Create Post

**Endpoint:** `POST /api/v1/posts`

**Description:** Create a new post in a section

**Authentication:** Required

**Request Body:**
```json
{
  "section_id": "550e8400-e29b-41d4-a716-446655440000",
  "content": "Check out this amazing song",
  "links": [
    {
      "url": "https://open.spotify.com/track/..."
    }
  ]
}
```

**Response (201 Created):**
```json
{
  "post": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "user_id": "550e8400-e29b-41d4-a716-446655440002",
    "section_id": "550e8400-e29b-41d4-a716-446655440000",
    "content": "Check out this amazing song",
    "links": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440003",
        "url": "https://open.spotify.com/track/...",
        "metadata": {
          "title": "Song Name",
          "artist": "Artist Name"
        },
        "created_at": "2026-01-16T10:30:00Z"
      }
    ],
    "comment_count": 0,
    "created_at": "2026-01-16T10:30:00Z"
  }
}
```

**Error Responses:**
- `400 Bad Request` (SECTION_ID_REQUIRED): Section ID is missing
- `400 Bad Request` (INVALID_SECTION_ID): Section ID format is invalid
- `400 Bad Request` (CONTENT_REQUIRED): Post content is missing
- `400 Bad Request` (CONTENT_TOO_LONG): Content exceeds 5000 characters
- `401 Unauthorized` (UNAUTHORIZED): Not authenticated
- `404 Not Found` (SECTION_NOT_FOUND): Section does not exist
- `500 Internal Server Error` (POST_CREATION_FAILED): Failed to create post

---

### 2. Get Post

**Endpoint:** `GET /api/v1/posts/{id}`

**Description:** Retrieve a single post with all related data

**Authentication:** Optional

**URL Parameters:**
- `id` (UUID): The ID of the post to retrieve

**Response (200 OK):**
```json
{
  "post": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "user_id": "550e8400-e29b-41d4-a716-446655440002",
    "section_id": "550e8400-e29b-41d4-a716-446655440000",
    "content": "Check out this amazing song",
    "links": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440003",
        "url": "https://open.spotify.com/track/...",
        "metadata": {
          "title": "Song Name",
          "artist": "Artist Name"
        },
        "created_at": "2026-01-16T10:30:00Z"
      }
    ],
    "comment_count": 3,
    "user": {
      "id": "550e8400-e29b-41d4-a716-446655440002",
      "username": "john_doe",
      "email": "john@example.com",
      "profile_picture_url": "https://...",
      "bio": "Music enthusiast",
      "is_admin": false,
      "created_at": "2026-01-10T08:00:00Z"
    },
    "created_at": "2026-01-16T10:30:00Z"
  }
}
```

**Error Responses:**
- `400 Bad Request` (INVALID_POST_ID): Invalid UUID format
- `404 Not Found` (POST_NOT_FOUND): Post does not exist or has been deleted
- `500 Internal Server Error` (GET_POST_FAILED): Failed to retrieve post

**Example cURL:**
```bash
curl -X GET http://localhost:8080/api/v1/posts/550e8400-e29b-41d4-a716-446655440001
```

---

### 3. Delete Post (Soft Delete)

**Endpoint:** `DELETE /api/v1/posts/{id}`

**Description:** Soft delete a post (only post owner or admin can delete)

**Authentication:** Required

**URL Parameters:**
- `id` (UUID): The ID of the post to delete

**Request Body:** Empty

**Response (200 OK):**
```json
{
  "post": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "user_id": "550e8400-e29b-41d4-a716-446655440002",
    "section_id": "550e8400-e29b-41d4-a716-446655440000",
    "content": "Check out this amazing song",
    "links": [...],
    "comment_count": 3,
    "user": {...},
    "created_at": "2026-01-16T10:30:00Z",
    "deleted_at": "2026-01-16T11:45:00Z",
    "deleted_by_user_id": "550e8400-e29b-41d4-a716-446655440002"
  },
  "message": "Post deleted successfully"
}
```

**Error Responses:**
- `400 Bad Request` (INVALID_POST_ID): Invalid UUID format
- `401 Unauthorized` (UNAUTHORIZED): Not authenticated
- `403 Forbidden` (UNAUTHORIZED): User is not the post owner and not an admin
- `404 Not Found` (POST_NOT_FOUND): Post does not exist
- `500 Internal Server Error` (POST_DELETION_FAILED): Failed to delete post

**Authorization Rules:**
- Post owner can always delete their own posts
- Admin can delete any post for moderation purposes
- Other users cannot delete posts they don't own

**Example cURL (Owner deletes own post):**
```bash
curl -X DELETE http://localhost:8080/api/v1/posts/550e8400-e29b-41d4-a716-446655440001 \
  -H "Cookie: session_id=<SESSION_ID>" \
  -H "Content-Type: application/json"
```

**Example cURL (Admin deletes any post):**
```bash
curl -X DELETE http://localhost:8080/api/v1/posts/550e8400-e29b-41d4-a716-446655440001 \
  -H "Cookie: session_id=<ADMIN_SESSION_ID>" \
  -H "Content-Type: application/json"
```

---

## Soft Delete Behavior

When a post is deleted:
1. `deleted_at` timestamp is set to current time
2. `deleted_by_user_id` is set to the ID of the user who deleted it
3. The post is no longer visible in feeds or search results
4. The post remains in the database for 7 days before hard purge
5. Admins can view deleted posts and restore them if needed (future feature)

Posts are excluded from all queries by default using `WHERE deleted_at IS NULL`.

---

## Notes

- All timestamps are in UTC ISO 8601 format
- Post content can be up to 5000 characters
- Link URLs can be up to 2048 characters
- Soft deletes preserve post history and relationships (comments, reactions)
- Hard deletion of old posts is handled by a scheduled cleanup job (future implementation)
