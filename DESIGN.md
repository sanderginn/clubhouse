# Clubhouse - Design Document

**Version:** 1.1
**Date:** January 22, 2026
**Status:** Implementation In Progress

---

## Table of Contents
1. [Product Overview](#product-overview)
2. [Features & MVP Scope](#features--mvp-scope)
3. [System Architecture](#system-architecture)
4. [Database Design](#database-design)
5. [API Specification](#api-specification)
6. [Authentication & Authorization](#authentication--authorization)
7. [Real-Time Communication](#real-time-communication)
8. [Link Metadata & Embeds](#link-metadata--embeds)
9. [Observability](#observability)
10. [Deployment & Operations](#deployment--operations)

---

## Product Overview

### Purpose
Clubhouse is a self-hosted, lightweight social platform for sharing links (music, photos, events, recipes, books, movies) within small-to-medium private communities of 5-500 people.

### Key Principles
- **Self-hosted & lightweight** — Minimal CPU/memory footprint, single Docker Compose deployment
- **Private by design** — No public internet access, all content visible only to registered members
- **Simple architecture** — Monolith backend, avoid microservices complexity
- **Observable from day one** — OpenTelemetry integration for logging, metrics, traces
- **Content-type aware** — Each section type can have custom metadata extraction and rich embeds

---

## Features & MVP Scope

### Content Types (Sections)
1. **Music** — Spotify, SoundCloud, YouTube links
2. **Photos** — Image uploads or links with metadata
3. **Events** — Event links (ra.co, Eventbrite, etc.)
4. **Recipes** — Recipe links with metadata
5. **Books** — Book links with cover art, metadata
6. **Movies** — Movie links with ratings, metadata
7. **General** — Threaded discussions without link requirements

### Core Features

#### User Management
- [x] User registration (username + password, email optional)
- [x] Admin approval required for registration
- [x] User profiles (bio, profile picture)
- [x] Session-based authentication (30-day duration)
- [x] Password hashing via bcrypt

#### Content Creation & Sharing
- [x] Create posts in any section with optional link
- [x] Create comments (threaded replies)
- [x] Tag users with @username (generates notifications)
- [x] Add emoji reactions to posts and comments
- [x] Delete own content (soft delete, user can restore within 7 days)
- [x] View user profile (posts + comments history)

#### Feed & Discovery
- [x] Chronological feeds per section
- [x] Global search (scoped to current section by default, optionally global or multi-section)
- [x] Search indexed on post/comment content and link metadata
- [x] Option to sort comments by creation time or by latest activity on post

#### Notifications
- [x] Real-time push via WebSocket
- [x] Notification types: new post in subscribed section, new comment, @mention, reaction
- [x] Mark notifications as read
- [x] Section opt-out (users opt-in to all sections by default, can opt-out)
- [x] Web Push API support for browser notifications

#### Admin Features
- [x] Content moderation (soft delete posts/comments)
- [x] Restore soft-deleted content
- [x] Manage user registrations
- [x] Toggle link metadata fetching globally
- [x] View audit logs of all moderation actions

#### Content Links & Metadata
- [x] Auto-fetch metadata (title, description, image) during post/comment creation
- [x] Store metadata as JSONB in database
- [x] Rich embeds where possible (Spotify player, YouTube embed, etc.)
- [x] Admin toggle to disable metadata fetching

### Out of Scope (Future)
- Direct messaging
- User suspension/bans
- Custom emoji
- Data export (user download)
- Federation/ActivityPub
- End-to-end encryption
- Algorithmic feeds

---

## System Architecture

### High-Level Diagram

```
┌─────────────────────────────────────────────┐
│      Svelte PWA (Web + Mobile Web)          │
│  - Responsive UI                            │
│  - Service Workers (offline support)        │
│  - Web Push API (notifications)             │
└────────────────┬────────────────────────────┘
                 │
        ┌────────┴────────┐
        │                 │
    HTTP REST         WebSocket
    /api/v1/*         /api/v1/ws
        │                 │
┌───────┴─────────────────┴──────────┐
│   Go HTTP Server (Monolith)        │
│                                    │
│  ├─ Auth handlers                  │
│  ├─ Post/Comment/Reaction CRUD     │
│  ├─ Feed & Search                  │
│  ├─ Notification dispatch          │
│  ├─ Link metadata fetcher (sync)   │
│  ├─ WebSocket connection manager   │
│  └─ Middleware (auth, OTel, etc.)  │
└───────────┬───────────────────────┬┘
            │                       │
    ┌───────┴─────────┐            │
    │                 │            │
PostgreSQL        Redis          OTel
(Data)        (Sessions +      Exporters
              Pub/Sub)         (OTLP)
                                │
                    ┌───────────┼───────────┐
                    │           │           │
                  Loki      Prometheus   Tempo
                 (Logs)     (Metrics)   (Traces)
                    │           │           │
                    └───────────┴───────────┘
                         │
                    Grafana UI
                 (Observability)
```

### Key Components

#### Backend (Go)
- **Entry point:** `cmd/server/main.go`
- **HTTP server:** Standard library `net/http` with custom router
- **Database:** PostgreSQL 16+, migrations via `sql-migrate` or similar
- **Session storage:** Redis 7+
- **Real-time:** Redis pub/sub + WebSocket
- **Observability:** OpenTelemetry SDK (all three signals)

#### Frontend (Svelte)
- **Framework:** SvelteKit (for routing, SSR optional)
- **UI:** Svelte components, Tailwind CSS
- **State:** Svelte stores (authStore, feedStore, etc.)
- **API client:** Centralized service layer
- **PWA:** Service workers, manifest, offline support
- **Notifications:** Web Push API + service worker listeners

#### Infrastructure
- **Docker Compose:** Local dev and production deployment
- **PostgreSQL 16+:** Primary data store
- **Redis 7+:** Sessions + pub/sub
- **Grafana Stack:** Loki (logs), Prometheus (metrics), Tempo (traces)

---

## Database Design

### Core Tables

#### users
```sql
CREATE TABLE users (
  id UUID PRIMARY KEY,
  username VARCHAR(255) UNIQUE NOT NULL,
  email VARCHAR(255) UNIQUE, -- optional
  password_hash VARCHAR(255) NOT NULL,
  profile_picture_url TEXT,
  bio TEXT,
  is_admin BOOLEAN DEFAULT false,
  approved_at TIMESTAMP,  -- NULL until admin approves
  created_at TIMESTAMP DEFAULT now(),
  deleted_at TIMESTAMP
);
```

#### sections
```sql
CREATE TABLE sections (
  id UUID PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  type VARCHAR(50) NOT NULL,  -- 'music', 'recipe', 'book', 'movie', 'event', 'photo', 'general'
  created_at TIMESTAMP DEFAULT now()
);
```

#### posts
```sql
CREATE TABLE posts (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id),
  section_id UUID NOT NULL REFERENCES sections(id),
  content TEXT NOT NULL,
  created_at TIMESTAMP DEFAULT now(),
  updated_at TIMESTAMP,
  deleted_at TIMESTAMP,
  deleted_by_user_id UUID REFERENCES users(id)
);
```

#### comments
```sql
CREATE TABLE comments (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id),
  post_id UUID NOT NULL REFERENCES posts(id),
  parent_comment_id UUID REFERENCES comments(id),  -- NULL = top-level
  content TEXT NOT NULL,
  created_at TIMESTAMP DEFAULT now(),
  updated_at TIMESTAMP,
  deleted_at TIMESTAMP,
  deleted_by_user_id UUID REFERENCES users(id)
);
```

#### reactions
```sql
CREATE TABLE reactions (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id),
  post_id UUID REFERENCES posts(id),
  comment_id UUID REFERENCES comments(id),
  emoji VARCHAR(10) NOT NULL,
  created_at TIMESTAMP DEFAULT now(),
  deleted_at TIMESTAMP,

  CONSTRAINT reaction_target CHECK (
    (post_id IS NOT NULL AND comment_id IS NULL) OR
    (post_id IS NULL AND comment_id IS NOT NULL)
  ),
  UNIQUE(user_id, post_id, emoji),
  UNIQUE(user_id, comment_id, emoji)
);
```

#### links
```sql
CREATE TABLE links (
  id UUID PRIMARY KEY,
  post_id UUID REFERENCES posts(id),
  comment_id UUID REFERENCES comments(id),
  url TEXT NOT NULL,
  metadata JSONB,  -- {title, description, image_url, provider, ...}
  created_at TIMESTAMP DEFAULT now(),

  CONSTRAINT link_target CHECK (
    (post_id IS NOT NULL AND comment_id IS NULL) OR
    (post_id IS NULL AND comment_id IS NOT NULL)
  )
);
```

#### mentions
```sql
CREATE TABLE mentions (
  id UUID PRIMARY KEY,
  post_id UUID REFERENCES posts(id),
  comment_id UUID REFERENCES comments(id),
  mentioned_user_id UUID NOT NULL REFERENCES users(id),
  created_at TIMESTAMP DEFAULT now(),

  CONSTRAINT mention_target CHECK (
    (post_id IS NOT NULL AND comment_id IS NULL) OR
    (post_id IS NULL AND comment_id IS NOT NULL)
  )
);
```

#### section_subscriptions
```sql
CREATE TABLE section_subscriptions (
  user_id UUID NOT NULL REFERENCES users(id),
  section_id UUID NOT NULL REFERENCES sections(id),
  opted_out_at TIMESTAMP DEFAULT now(),

  PRIMARY KEY (user_id, section_id)
);
-- No row = user is subscribed (default)
-- Row exists = user is opted out
```

#### notifications
```sql
CREATE TABLE notifications (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id),
  type VARCHAR(50) NOT NULL,  -- 'new_post', 'new_comment', 'mention', 'reaction'
  related_post_id UUID REFERENCES posts(id),
  related_comment_id UUID REFERENCES comments(id),
  related_user_id UUID REFERENCES users(id),  -- who triggered notification
  read_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT now()
);
```

#### push_subscriptions
```sql
CREATE TABLE push_subscriptions (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id),
  endpoint TEXT NOT NULL UNIQUE,
  auth_key TEXT NOT NULL,
  p256dh_key TEXT NOT NULL,
  created_at TIMESTAMP DEFAULT now(),
  deleted_at TIMESTAMP
);
```

#### audit_logs
```sql
CREATE TABLE audit_logs (
  id UUID PRIMARY KEY,
  admin_user_id UUID NOT NULL REFERENCES users(id),
  action VARCHAR(50) NOT NULL,  -- 'delete_post', 'delete_comment', 'restore_post', etc.
  related_post_id UUID REFERENCES posts(id),
  related_comment_id UUID REFERENCES comments(id),
  created_at TIMESTAMP DEFAULT now()
);
```

### Indexing Strategy

```sql
-- Foreign keys (implicit indexes)
CREATE INDEX idx_posts_user_id ON posts(user_id);
CREATE INDEX idx_posts_section_id ON posts(section_id);
CREATE INDEX idx_comments_post_id ON comments(post_id);
CREATE INDEX idx_comments_user_id ON comments(user_id);

-- Time-based queries (feeds)
CREATE INDEX idx_posts_section_created ON posts(section_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_comments_post_created ON comments(post_id, created_at DESC) WHERE deleted_at IS NULL;

-- Search (full-text)
CREATE INDEX idx_posts_content_fts ON posts USING GIN(to_tsvector('english', content));
CREATE INDEX idx_comments_content_fts ON comments USING GIN(to_tsvector('english', content));
CREATE INDEX idx_links_metadata ON links USING GIN(metadata);

-- Lookups
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_mentions_mentioned_user ON mentions(mentioned_user_id);
CREATE INDEX idx_notifications_user_read ON notifications(user_id, read_at);
```

### Data Retention

- **Soft-deleted content:** Owners can restore within 7 days; admins can restore anytime. No automated purge job in the repo.
- **Notifications:** Retained until deleted by related-content cleanup or manual deletion
- **Audit logs:** Retained indefinitely unless manually purged
- **Sessions (Redis):** 30-day expiry, auto-deleted by Redis

---

## API Specification

### Base URL
`/api/v1`

### Standard Response Format

**Success (2xx):** returns the response payload defined by the endpoint (no envelope).
```json
{
  "post": {
    "id": "uuid",
    "user_id": "uuid",
    "section_id": "uuid",
    "content": "Hello world",
    "comment_count": 0,
    "created_at": "2026-01-16T10:00:00Z"
  }
}
```

**Error (4xx/5xx):**
```json
{
  "error": "Human-readable message",
  "code": "ERROR_CODE"
}
```

### Common Error Codes
- `INVALID_REQUEST` — Bad request format
- `UNAUTHORIZED` — Missing/invalid auth
- `FORBIDDEN` — Authenticated but lacks permission
- `NOT_FOUND` — Resource not found
- `CONFLICT` — Duplicate/constraint violation
- `RATE_LIMITED` — Too many requests
- `INTERNAL_ERROR` — Server error

### Pagination
- **Cursor-based** for feeds/comments
- Request: `?limit=20&cursor=post-id`
- Response includes: `nextCursor`, `hasMore`

### Rate Limiting
- **General API:** 600 req/min per user (10/sec)
- **Aggressive:** 100 req/min for post creation (prevent spam)
- **Headers:** `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`

### Endpoints

#### Authentication

**Register User**
```
POST /auth/register
Body: { username, password, email? }
Response: { id, username, email, message }
```

**Login**
```
POST /auth/login
Body: { username, password }
Response: { id, username, email, is_admin, message }
Headers: Set-Cookie: session_id=...; HttpOnly; Secure; SameSite=Lax; Max-Age=2592000
```

**Logout**
```
POST /auth/logout
Response: {}
Headers: Set-Cookie: session=; Max-Age=0
```

#### Posts

**Create Post**
```
POST /posts
Auth: Required
Body: {
  sectionId: "uuid",
  content: "text",
  links: [{ url: "https://..." }]  // optional
}
Response: { post: { id, userId, sectionId, content, links, createdAt } }
```

**Get Post**
```
GET /posts/{id}
Response: {
  post: { ... },
  comments: [ { id, userId, content, ... } ]
}
```

**Get Feed (Section)**
```
GET /sections/{sectionId}/feed?limit=20&cursor=post-id
Response: {
  posts: [ ... ],
  meta: { cursor, hasMore }
}
```

**Delete Post (Soft)**
```
DELETE /posts/{id}
Auth: Required (owner or admin)
Response: {}
```

**Restore Post**
```
POST /posts/{id}/restore
Auth: Required (owner or admin)
Response: { post: { ... } }
```

#### Comments

**Create Comment**
```
POST /comments
Auth: Required
Body: {
  postId: "uuid",
  parentCommentId: "uuid",  // optional, for replies
  content: "text",
  links: [{ url: "https://..." }]  // optional
}
Response: { comment: { id, userId, postId, parentCommentId, content, createdAt } }
```

**Get Thread (Post + Comments)**
```
GET /posts/{postId}/comments?limit=50&cursor=comment-id
Response: {
  comments: [ ... ],
  meta: { cursor, hasMore }
}
```

**Delete Comment (Soft)**
```
DELETE /comments/{id}
Auth: Required (owner or admin)
Response: {}
```

**Restore Comment**
```
POST /comments/{id}/restore
Auth: Required (owner or admin)
Response: { comment: { ... } }
```

#### Reactions

**Add Reaction**
```
POST /posts/{postId}/reactions
Auth: Required
Body: { emoji: "👍" }
Response: { reaction: { id, userId, emoji, createdAt } }
```

**Remove Reaction**
```
DELETE /posts/{postId}/reactions/{emoji}
Auth: Required
Response: {}
```

**Add Reaction to Comment**
```
POST /comments/{commentId}/reactions
Auth: Required
Body: { emoji: "👍" }
Response: { reaction: { ... } }
```

**Remove Reaction from Comment**
```
DELETE /comments/{commentId}/reactions/{emoji}
Auth: Required
Response: {}
```

#### Search

**Global Search**
```
GET /search?q=query&scope=global&limit=20
Scope: section (current section), global, or multi-section
Response: {
  results: [
    { type: "post", data: { ... } },
    { type: "comment", data: { ... } },
    { type: "link_metadata", data: { ... } }
  ]
}
```

#### Sections

**List Sections**
```
GET /sections
Response: { sections: [ { id, name, type } ] }
```

**Get Section**
```
GET /sections/{id}
Response: { section: { id, name, type } }
```

#### Users

**Get User Profile**
```
GET /users/{id}
Response: {
  user: { id, username, bio, profilePictureUrl, createdAt },
  stats: { postCount, commentCount }
}
```

**Get User's Posts**
```
GET /users/{id}/posts?limit=20&cursor=post-id
Response: {
  posts: [ ... ],
  meta: { cursor, hasMore }
}
```

**Get User's Comments**
```
GET /users/{id}/comments?limit=20&cursor=comment-id
Response: {
  comments: [ ... ],
  meta: { cursor, hasMore }
}
```

**Update Own Profile**
```
PATCH /users/me
Auth: Required
Body: { bio, profilePictureUrl }
Response: { user: { ... } }
```

#### Notifications

**Get Notifications**
```
GET /notifications?limit=50&cursor=notification-id
Auth: Required
Response: {
  notifications: [ ... ],
  meta: { cursor, hasMore, unreadCount }
}
```

**Mark Notification as Read**
```
PATCH /notifications/{id}
Auth: Required
Body: { read: true }
Response: { notification: { ... } }
```

#### Admin

**Delete Post (Hard)**
```
DELETE /admin/posts/{id}
Auth: Required, Admin only
Response: {}
```

**Delete Comment (Hard)**
```
DELETE /admin/comments/{id}
Auth: Required, Admin only
Response: {}
```

**Restore Soft-Deleted Post**
```
POST /admin/posts/{id}/restore
Auth: Required, Admin only
Response: { post: { ... } }
```

**Approve User Registration**
```
POST /admin/users/{id}/approve
Auth: Required, Admin only
Response: { user: { ... } }
```

**Reject User Registration**
```
DELETE /admin/users/{id}
Auth: Required, Admin only
Response: {}
```

**Toggle Link Metadata Fetching**
```
PATCH /admin/config
Auth: Required, Admin only
Body: { linkMetadataEnabled: true }
Response: { config: { linkMetadataEnabled: true } }
```

**View Audit Logs**
```
GET /admin/audit-logs?limit=100&cursor=log-id
Auth: Required, Admin only
Response: {
  logs: [ { id, adminUserId, action, relatedPostId, createdAt } ],
  meta: { cursor, hasMore }
}
```

---

## Authentication & Authorization

### Session-Based Authentication

1. **User registers** → Account created, marked `approved_at = NULL`
2. **Admin approves** → Sets `approved_at = NOW()`
3. **User logs in (username + password)** → Session created in Redis, httpOnly cookie set
4. **Cookie stored** → Automatically sent with all requests
5. **Middleware validates** → Checks Redis session, extracts `user_id`
6. **Session expires** → 30 days, auto-deleted by Redis TTL

**Email policy:** Email is optional at registration. If provided, it must be valid and unique.

### JWT (if needed internally)
Not used for client auth. Consider for:
- API keys for internal services (future)
- WebSocket auth (optional, can use session cookie)

### Password Hashing
- Algorithm: **bcrypt**
- Cost factor: 12 (default)
- Never store plaintext

### Authorization Rules
- **Public endpoints:** Registration, login
- **User endpoints:** Authenticated users only
- **Own-content endpoints:** Owner or admin
- **Admin endpoints:** `is_admin = true` only

### CSRF Protection

**Purpose:** Prevent Cross-Site Request Forgery attacks on state-changing operations.

**Token Lifecycle:**
1. Client calls `GET /api/v1/auth/csrf` (authenticated endpoint) to obtain a token
2. Server generates a 256-bit cryptographically secure random token
3. Token is stored in Redis with key `csrf:{token}` → value `{sessionID}:{userID}`
4. Token has 1-hour TTL (auto-expires via Redis)
5. Client includes token in `X-CSRF-Token` header for all state-changing requests

**Validation:**
- Middleware `RequireCSRF` validates tokens on POST/PUT/PATCH/DELETE requests
- GET/HEAD/OPTIONS requests bypass CSRF validation (read-only operations)
- Token must match the current session ID and user ID
- Invalid or missing tokens return `403 Forbidden`

**Token Format:**
- 32 bytes (256 bits) of cryptographically secure random data
- Base64-URL encoded for safe HTTP transport
- Example: `a7b3c9d2e1f4g5h6i7j8k9l0m1n2o3p4q5r6s7t8u9v0w1x2y3z4a5b6c7d8e9f0`

**Exempted Endpoints:**
- `POST /api/v1/auth/register` - no session exists yet
- `POST /api/v1/auth/login` - no session exists yet
- `GET /api/v1/auth/csrf` - token issuance endpoint (read-only for token generation)
- All GET/HEAD/OPTIONS requests - read-only operations

**Token Refresh:**
- Tokens are reusable within their 1-hour TTL
- Clients should fetch a new token when receiving `403 INVALID_CSRF_TOKEN`
- No automatic rotation on each request (reduces Redis load)

**Error Responses:**
```json
{
  "error": "CSRF token is required for this request",
  "code": "CSRF_TOKEN_REQUIRED"
}
```
```json
{
  "error": "Invalid or expired CSRF token",
  "code": "INVALID_CSRF_TOKEN"
}
```

---

## Real-Time Communication

### WebSocket Connection

**Endpoint:** `GET /api/v1/ws`

**Authentication:** Session cookie (sent automatically)

**Connection Lifecycle:**
1. Client initiates WebSocket handshake
2. Middleware validates session
3. Go server adds client to in-memory connection map
4. Client subscribes to Redis channels for their sections + own notifications
5. On disconnect, remove from map and unsubscribe

### WebSocket Event Format

**Client → Server:**
```json
{
  "type": "subscribe",
  "data": {
    "sectionIds": ["section-uuid-1", "section-uuid-2"]
  }
}
```

**Server → Client:**
```json
{
  "type": "event_type",
  "data": { ... },
  "timestamp": "2026-01-16T10:00:00Z"
}
```

### Event Types

#### Broadcast Events (via Redis pub/sub)

**new_post**
```json
{
  "type": "new_post",
  "data": {
    "post": { id, userId, sectionId, content, createdAt },
    "user": { id, username, profilePictureUrl }
  }
}
```

**new_comment**
```json
{
  "type": "new_comment",
  "data": {
    "comment": { id, userId, postId, parentCommentId, content, createdAt },
    "user": { id, username },
    "postId": "uuid"
  }
}
```

**post_deleted**
```json
{
  "type": "post_deleted",
  "data": { postId: "uuid" }
}
```

**comment_deleted**
```json
{
  "type": "comment_deleted",
  "data": { commentId: "uuid" }
}
```

**reaction_added**
```json
{
  "type": "reaction_added",
  "data": {
    "postId": "uuid",
    "commentId": "uuid",  // or null
    "userId": "uuid",
    "emoji": "👍"
  }
}
```

**reaction_removed**
```json
{
  "type": "reaction_removed",
  "data": {
    "postId": "uuid",
    "commentId": "uuid",  // or null
    "userId": "uuid",
    "emoji": "👍"
  }
}
```

#### User-Specific Events

**mention**
```json
{
  "type": "mention",
  "data": {
    "mentioningUser": { id, username },
    "postId": "uuid",
    "commentId": "uuid",  // or null
    "excerpt": "excerpt of content with @mention"
  }
}
```

**notification**
```json
{
  "type": "notification",
  "data": {
    "id": "uuid",
    "type": "new_post",
    "sectionName": "Music",
    "data": { ... }
  }
}
```

### Redis Pub/Sub Channels

- `section:{sectionId}` — All posts/comments in section
- `post:{postId}` — Reactions, comments, deletes on post
- `user:{userId}:mentions` — Mentions of user
- `user:{userId}:notifications` — Notifications for user

---

## Link Metadata & Embeds

### Synchronous Fetching

When user creates post or comment with link:
1. **Parse URL** → Extract domain, path
2. **Fetch metadata** → HTTP request to target URL (timeout: 5s)
3. **Extract OG tags** — `og:title`, `og:description`, `og:image`, etc.
4. **Provider detection** — Spotify, YouTube, etc. for rich embeds
5. **Store JSONB** — Save metadata to `links` table
6. **Return to client** — Include in response immediately

### Metadata Structure (JSONB)

```json
{
  "url": "https://open.spotify.com/track/...",
  "provider": "spotify",
  "title": "Song Name",
  "description": "Artist - Album",
  "image": "https://...",
  "author": "Artist Name",
  "duration": 180,
  "embedUrl": "https://open.spotify.com/embed/track/..."
}
```

### Content-Type Specific Extraction

**Music (Spotify, SoundCloud, YouTube):**
- Track/album title
- Artist name
- Duration
- Cover art
- Embed URL for player

**Movies (IMDB, Rotten Tomatoes):**
- Title, year
- Rating, reviews score
- Poster image
- Synopsis

**Books (Goodreads, ISBN):**
- Title, author
- Cover image
- Description
- Rating

**Events (ra.co, Eventbrite):**
- Event name, date/time
- Location, organizers
- Ticket info (if public)

**Recipes (AllRecipes, etc.):**
- Recipe name
- Ingredients list
- Cook time
- Image

**Photos:**
- Image URL
- Dimensions
- Alt text (if provided)

### Rich Embed Rendering (Frontend)

Each provider type has a custom Svelte component:
- `SpotifyEmbed.svelte` — Embedded player
- `YouTubeEmbed.svelte` — Video player
- `EventEmbed.svelte` — Date/time/location
- `RecipeEmbed.svelte` — Ingredients list
- `BookEmbed.svelte` — Cover + description
- `GenericEmbed.svelte` — Fallback (image + title + description)

### Admin Control

Endpoint to toggle globally:
```
PATCH /admin/config
Body: { linkMetadataEnabled: false }
```

If disabled:
- No metadata fetching on post creation
- Existing metadata still displayed
- Links show as plain URLs

---

## Observability

### OpenTelemetry Signals

#### 1. Traces
- **Sampler:** No sampling (trace 100% of requests)
- **Scope:** Every HTTP request, WebSocket message, database query
- **Attributes:** `user_id`, `section_id`, `post_id`, error details
- **Exporters:** OTLP to Grafana Tempo

#### 2. Metrics
- **HTTP requests:** Request count, duration, status codes (per endpoint)
- **Database:** Query count, latency, errors
- **WebSocket:** Connections, messages, subscriptions
- **Business logic:** Posts created, comments added, reactions, deletes
- **Exporters:** OTLP to Grafana Prometheus

#### 3. Logs
- **Structured logging:** JSON format via OpenTelemetry logs API
- **Fields:** timestamp, level, message, trace_id, span_id, user_id, error stack
- **Exporters:** OTLP to Grafana Loki

### Retention & Storage
- **Traces:** Tempo local config uses `compacted_block_retention: 10m` (see `tempo.yml`)
- **Metrics:** Prometheus uses default retention unless overridden (scrape interval: 15s in `prometheus.yml`)
- **Logs:** Loki uses its default local config unless overridden

### Local Development
```yaml
# docker-compose.yml includes:
services:
  loki:
    image: grafana/loki:latest
  prometheus:
    image: prom/prometheus:latest
  tempo:
    image: grafana/tempo:latest
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
```

---

## Deployment & Operations

### Docker Compose (Local & Production)

**Local Dev:**
```bash
docker compose up -d
# Starts: PostgreSQL, Redis, Grafana Stack, backend, frontend
```

**Production:**
```bash
docker compose -f docker-compose.prod.yml up -d
# Same services, production-grade configs
```

### Database Migrations
```bash
# Create migration
migrate create -ext sql -dir backend/migrations -seq add_users_table

# Run migrations
docker compose exec backend migrate -path migrations -database "postgres://..." up
```

### Secrets Management
- Store in `.env` file (dev) or environment variables (production)
- Never commit secrets
- Rotate regularly

### Monitoring & Alerting
- Grafana dashboard for real-time metrics
- Loki for log searching and debugging
- Tempo for request tracing
- Set up alerts for:
  - High error rate (>1%)
  - Database connection pool exhaustion
  - Redis pub/sub lag
  - Disk usage on PostgreSQL

### Scaling Considerations
- Go server is stateless (sessions in Redis) → scale horizontally
- Redis pub/sub distributes real-time events → can add multiple servers
- PostgreSQL is single instance (upgrade hardware, or later: replication + read replicas)
- No sharding needed for 500 users

---

**End of Design Document**
