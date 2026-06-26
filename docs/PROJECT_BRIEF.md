# BookWise — Go Backend Learning Project Brief

## 1. Project Summary

**BookWise** is a personal reading library and book community project built primarily to learn modern backend engineering, infrastructure, and production-oriented software development.

The project starts as a simple system for recording books I have read, but it gradually evolves into a full backend-focused product with authentication, moderation, book submissions, comments, ratings, ranking algorithms, Redis caching, Docker, CI/CD, deployment, and eventually Kubernetes.

The goal is not only to build a book application. The real goal is to understand **why backend technologies exist**, what problems they solve, and how they work together in a realistic project.

---

## 2. Main Goal

The main goal is to build a production-like backend system using **Go**, while learning the surrounding backend ecosystem step by step.

The project should answer these questions through implementation:

- Why use Go instead of Node.js for backend services?
- How does a compiled backend service differ from a JavaScript runtime-based backend?
- How should a clean backend project be structured?
- How do authentication, authorization, and admin permissions work?
- How do PostgreSQL relationships work in a real domain?
- How do comments, ratings, approval flows, and moderation systems work?
- Why and where should Redis be used?
- Why do we need Docker?
- What problem does NGINX solve?
- When does Kubernetes become useful?
- How can a project be built using mostly free or low-cost tools?

---

## 3. Why This Project Exists

This project exists as a long-term learning playground.

Instead of learning Docker, Redis, PostgreSQL, NGINX, Kubernetes, CI/CD, and deployment as separate abstract topics, this project gives each technology a concrete reason to exist.

Examples:

- PostgreSQL is used because books, users, genres, comments, and ratings are relational data.
- Redis is used because top books, popular books, sessions, and rate limits need fast access.
- Docker is used because the backend, database, Redis, and other services should run consistently across environments.
- NGINX is used because the frontend and backend should be served behind a single entry point.
- Kubernetes is introduced later because multiple containers and services need orchestration.
- CI/CD is added because the project should be tested, built, and deployed automatically.

The key principle is:

> Do not add a technology just because it is popular. Add it when the project has a real problem that the technology solves.

---

## 4. Product Vision

BookWise is a platform where:

- I can record books I have read.
- I can add notes and personal thoughts about books.
- Visitors can view approved books.
- Registered users can suggest new books.
- Suggested books are not published immediately; they require admin approval.
- Users can comment on books.
- Only the admin can delete comments.
- Books can belong to one or more genres.
- Users can rate books.
- Book ranking is not based only on average rating.
- A book with one `10/10` rating should not rank higher than a book with thousands of `8.5/10` ratings.
- The system should use a reliable weighted rating algorithm.
- Admin users can moderate submitted books and comments.

---

## 5. Core Roles

### Guest

A guest is a non-authenticated visitor.

Can:

- View published books.
- View book details.
- View genres.
- View public ratings and comments.
- Search and filter books.

Cannot:

- Add comments.
- Rate books.
- Suggest books.
- Access admin pages.

### User

A registered user.

Can:

- Login and manage profile.
- Suggest new books.
- Comment on books.
- Rate books.
- Edit or delete their own rating.
- View their submitted book suggestions.

Cannot:

- Directly publish books.
- Delete other users' comments.
- Manage genres.
- Access admin-only moderation features.

### Admin

The owner/moderator of the platform.

Can:

- Approve or reject suggested books.
- Create, update, and delete books.
- Manage genres.
- Delete comments.
- View moderation queues.
- View rating statistics.
- Recalculate book scores.
- Manage reported comments or suspicious activity.

---

## 6. Main Features

## 6.1 Authentication and Authorization

The system should support:

- User registration.
- User login.
- Password hashing.
- JWT-based authentication.
- Refresh token support in a later phase.
- Role-based authorization.
- Admin-only endpoints.

Recommended roles:

```text
USER
ADMIN
```

Later optional roles:

```text
MODERATOR
```

SECURITY BASELINE (from review — this is the security-sensitive phase; slow down here in
Phase 3). Full details in [GO_CONVENTIONS.md](GO_CONVENTIONS.md) §12:

- Password hashing: bcrypt (cost ≥ 12) or argon2id — never SHA/MD5.
- JWT: short-lived access (~15 min); **explicitly verify the signing algorithm** (reject
  `alg=none`); secret from env, never committed; set `exp/iat/iss/aud`.
- "Logout": a stateless JWT can't be revoked — use the DB-backed `refresh_tokens` table (§9),
  rotate on use, logout = delete the refresh token (no Redis blacklist just to log out).
- Authorization in the service layer + **resource-ownership checks** (IDOR): e.g.
  `PATCH/DELETE /books/{id}/ratings` must verify the rating belongs to the caller, not merely
  that they are authenticated.
- Whitelisted DTOs (no mass assignment); parameterized queries (no `ILIKE` string concat,
  §6.8); generic auth errors; CORS allowlist; rate-limit login/register; verify/reset tokens
  single-use + expiring + `crypto/rand` + hashed.

---

## 6.2 Book Management

A book should include:

- ID
- Title
- Slug
- Author
- Description
- Cover image URL
- Publication year
- Page count
- Language
- Status
- Average rating
- Rating count
- Weighted score
- Created at
- Updated at

Book statuses:

```text
PENDING
APPROVED
REJECTED
ARCHIVED
```

Only approved books should be publicly visible.

---

## 6.3 Book Suggestions

Users can suggest books.

A suggested book should not immediately become public.

Flow:

```text
User submits book
        ↓
Book status = PENDING
        ↓
Admin reviews
        ↓
Admin approves or rejects
        ↓
If approved, book becomes public
```

Admin should be able to:

- View pending suggestions.
- Approve a suggestion.
- Reject a suggestion with an optional reason.
- Edit book details before approval.

---

## 6.4 Genres

Books can have multiple genres.

Examples:

- Software
- Psychology
- Philosophy
- Novel
- History
- Productivity
- Science Fiction
- Biography

This should be modeled as a many-to-many relationship:

```text
books
book_genres
genres
```

Admin can:

- Create genres.
- Update genres.
- Delete or archive genres.

Users can:

- Filter books by genre.

---

## 6.5 Comments

Users can comment on approved books.

Comment fields:

- ID
- Book ID
- User ID
- Content
- Status
- Created at
- Updated at
- Deleted at
- Deleted by admin ID

Comment statuses:

```text
VISIBLE
DELETED
HIDDEN
```

Important rule:

> Only the admin can delete comments.

Later, users may be allowed to edit their own comments, but deletion remains admin-controlled.

---

## 6.6 Ratings

Users can rate books.

Rating scale:

```text
1 to 10
```

Rules:

- A user can rate a book only once.
- A user can update their previous rating.
- A user can remove their own rating in a later phase.
- Rating changes should update the book's rating statistics.

CORRECTNESS (from review — the #1 data-integrity lesson, Phase 5):

- Enforce "rate once" at the DB with `UNIQUE(user_id, book_id)`, and implement
  create-or-update as `INSERT ... ON CONFLICT (user_id, book_id) DO UPDATE` (one statement,
  no read-then-write race).
- `ratings` is the **source of truth**. Never app-read-then-write `average_rating` /
  `rating_count` (concurrent ratings = lost update). Recompute aggregates from the table
  **inside the same transaction** as the rating write (SQL `AVG`/`COUNT`), or compute at read
  time. The same recompute applies on delete. See [GO_CONVENTIONS.md](GO_CONVENTIONS.md) §7
  and [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) §10.

Book should store:

- Average rating (NUMERIC, derived)
- Rating count (derived)
- Weighted score — **computed, not write-denormalized** (see §6.7)

---

## 6.7 Weighted Rating Algorithm

The system should not rank books only by average rating.

Problem:

```text
Book A: 1 rating, average 10.0
Book B: 2000 ratings, average 8.5
```

A naive average would rank Book A higher, but Book B is more trustworthy.

Use a Bayesian/IMDb-style weighted rating formula:

```text
WR = (v / (v + m)) * R + (m / (v + m)) * C
```

Where:

```text
WR = Weighted Rating
R  = Average rating of the book
v  = Number of ratings for the book
m  = Minimum rating count required to be considered reliable
C  = Global average rating across all books
```

Example:

```text
Book A
R = 10
v = 1

Book B
R = 8.5
v = 2000

C = 7.0
m = 100
```

Book A's weighted score should be pulled down because it has too few ratings.
Book B should rank higher because many users rated it highly.

Sorting should use:

```text
weighted_score DESC
```

not simply:

```text
average_rating DESC
```

DECISION (from review — Phase 6):

- Do **not** store `weighted_score` as a per-write denormalized column. `C` (global average)
  and `m` are **global**, so any new rating anywhere changes every book's correct score — a
  stored column silently goes stale for all other books.
- Compute it **at query time** with parameterized `C`/`m` in the `ORDER BY`, or via a
  periodically-refreshed **materialized view** once you can *measure* the sort is slow (that
  measurement is the Phase 7 "why cache" lesson — don't pre-optimize).
- `m` is a **config knob** — start small (~10), not 100 (with few ratings, m=100 collapses
  every score toward `C` and the ranking is useless). `C` is **derived**
  (`SELECT AVG(score) FROM ratings`) with a sane cold-start fallback while the dataset is tiny.

---

## 6.8 Search and Filtering

Initial search can use PostgreSQL `ILIKE`.

Search fields:

- Title
- Author
- Description
- Genre

Later improvements:

- PostgreSQL full-text search.
- Weighted search ranking.
- Search suggestions.
- Popular search terms.
- Elasticsearch or Meilisearch as optional future additions.

---

## 6.9 Redis Usage

Redis should not be added at the beginning just for the sake of using Redis.

Add Redis when there is a clear need.

Possible Redis use cases:

- Cache top-rated books.
- Cache popular books.
- Cache book detail pages.
- Store rate limiting counters.
- Store temporary email verification tokens.
- Store password reset tokens.
- Store session or token blacklist data.
- Queue background jobs with Redis-backed tools later.

Example cache keys:

```text
books:top-rated:v1
books:popular:v1
book:{book_id}:detail
rate-limit:user:{user_id}
rate-limit:ip:{ip_address}
```

---

## 6.10 Admin Panel

Admin panel should include:

- Pending book suggestions.
- Approved books.
- Rejected books.
- Genre management.
- Comment moderation.
- Rating statistics.
- User list.
- Reported comments.

Admin-only pages can be added in the frontend later.

The backend should expose admin-only APIs from the beginning.

---

## 7. Recommended Tech Stack

## 7.1 Backend

```text
Language: Go
Router: Chi
Database driver: pgx (pgxpool)
Query layer: sqlc + pgx   (DECISION: committed from Phase 2 — not "manual or later")
ORM: none — GORM/bun explicitly EXCLUDED
Migrations: goose
Validation: go-playground/validator (handler-layer DTOs)
Auth: JWT (see expanded §6.1 security baseline)
Logging: slog (stdlib, context-aware) — zerolog optional later
Config: environment variables (validated at boot, fail-fast)
```

Recommended starting choice:

```text
Go + Chi + pgx + sqlc + goose
```

Reason:

- Chi keeps HTTP concepts visible.
- pgx is a strong PostgreSQL driver.
- sqlc reads your hand-written SQL and generates type-safe Go — you still learn joins and the
  many-to-many (§6.4), but the `rows.Scan`/`rows.Err()`/NULL footguns are removed. It is NOT
  an ORM (no query DSL, no magic). An ORM would defeat §2's "learn how relations work" goal.
- goose is simple for migrations.
- This stack avoids too much framework magic.

> See [GO_CONVENTIONS.md](GO_CONVENTIONS.md) for the Go idioms (ctx-first, `%w` errors,
> hand-wired DI, table-driven tests) and [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) §6 for *why*
> sqlc+pgx over Prisma/an ORM.

---

## 7.2 Database

```text
PostgreSQL
```

Why:

- Strong relational modeling.
- Good for many-to-many relationships.
- Good aggregation support for ratings.
- Supports full-text search later.

DECISION (hosting): **dev = local Docker Postgres · prod = Supabase free tier.** The app
connects only via a connection string (`DATABASE_URL`), so the same code targets either by
changing one env var (12-factor). This is the professional-standard setup and maximizes
Docker/infra learning.

- **dev** → a Postgres container in Docker Compose (§7.6). Fast, offline, free, disposable —
  run `goose down`, reset the schema, seed garbage freely without touching anything real.
- **prod** → a Supabase project (real user data). Never point dev tooling (`goose down`,
  resets) at it. The dev DSN lives in local `.env`; the prod DSN lives in the deploy
  platform's secret store (Render/Fly/Vercel env) and is **never** committed.

Practical notes for Supabase + pgx in **prod** (these will save you an afternoon):

- Use `sslmode=require` — managed Postgres mandates TLS (local Docker doesn't).
- Connect the app through the **pooler (Supavisor)** with `pgxpool`. With the
  transaction-mode pooler, **disable pgx prepared-statement caching** (simple protocol) —
  prepared statements aren't supported there.
- Run **goose migrations against the direct connection (port 5432)**, not the transaction
  pooler.
- Free projects **pause after ~1 week of inactivity** and add network latency. Free-tier
  limits change — check before relying (see §13).
- Worth recording the dev-local / prod-cloud split as an ADR in `docs/decisions/`.

---

## 7.3 Cache

```text
Redis
```

DECISION (hosting): **dev = local Docker Redis · prod = Upstash free tier** — same split as
the database (§7.2). Connect via a `REDIS_URL` using `redis/go-redis/v9` (local: `redis://`;
Upstash: `rediss://`, TLS). dev URL in local `.env`, prod URL in the deploy platform's secret
store, never committed.

Notes:

- Upstash free tier has a **daily command limit** and is serverless. Fine for learning; check
  current limits before relying (§13).
- Redis is still **not introduced until Phase 7**, and only when a real caching / rate-limit
  need exists (§6.9, §14.5) — choosing a host now does not mean wiring it early.

---

## 7.4 Frontend

Recommended:

```text
React or Next.js
```

Since the main learning goal is backend, frontend should start simple.

Initial frontend can include:

- Book list page.
- Book detail page.
- Login/register pages.
- Suggest book page.
- Admin pending books page.

---

## 7.5 File Storage

For book cover images:

Local first:

```text
Local filesystem or MinIO
```

Cloud later:

```text
Cloudflare R2
Supabase Storage
```

---

## 7.6 Local Development

Docker Compose runs the **dev** stack locally:

- PostgreSQL (dev database — §7.2; prod uses Supabase)
- Redis (dev cache — §7.3; prod uses Upstash, introduced Phase 7)
- Optional MinIO (object-storage experiments)
- The Go API container itself (from Phase 8)

This is where you learn Docker hands-on: images, volumes (DB data persistence), container
networking, `depends_on`/healthchecks, and Compose orchestration. In **prod** the stateful
services are managed cloud (Supabase + Upstash); only your API image is deployed — same code,
different `DATABASE_URL`/`REDIS_URL`.

Local (dev) architecture:

```text
Frontend
   ↓
Go API
   ↓
PostgreSQL (Docker)  +  Redis (Docker, Phase 7+)
```

---

## 8. Suggested Repository Structure

```text
bookwise/
├── backend/
│   ├── cmd/
│   │   └── api/
│   │       └── main.go
│   ├── internal/
│   │   ├── auth/
│   │   ├── books/
│   │   ├── comments/
│   │   ├── config/
│   │   ├── database/
│   │   ├── genres/
│   │   ├── middleware/
│   │   ├── ratings/
│   │   ├── users/
│   │   └── server/
│   ├── migrations/
│   ├── sql/
│   ├── go.mod
│   └── go.sum
├── frontend/
├── docker-compose.yml
├── .env.example
├── README.md
└── PROJECT_BRIEF.md
```

---

## 9. Initial Database Tables

Recommended first version:

```text
users
books
genres
book_genres
comments
ratings
refresh_tokens
```

Later:

```text
book_reports
comment_reports
book_views
reading_lists
reading_list_items
notifications
audit_logs
```

---

## 10. Core API Endpoints

## 10.1 Auth

```text
POST /api/auth/register
POST /api/auth/login
POST /api/auth/refresh
POST /api/auth/logout
GET  /api/auth/me
```

## 10.2 Books

```text
GET    /api/books
GET    /api/books/{id}
POST   /api/books/suggestions
GET    /api/books/top-rated
GET    /api/books/popular
```

Admin:

```text
POST   /api/admin/books
PATCH  /api/admin/books/{id}
DELETE /api/admin/books/{id}
GET    /api/admin/books/pending
POST   /api/admin/books/{id}/approve
POST   /api/admin/books/{id}/reject
```

## 10.3 Genres

```text
GET /api/genres
```

Admin:

```text
POST   /api/admin/genres
PATCH  /api/admin/genres/{id}
DELETE /api/admin/genres/{id}
```

## 10.4 Comments

```text
GET  /api/books/{id}/comments
POST /api/books/{id}/comments
```

Admin:

```text
DELETE /api/admin/comments/{id}
GET    /api/admin/comments/reported
```

## 10.5 Ratings

```text
POST   /api/books/{id}/ratings
PATCH  /api/books/{id}/ratings
DELETE /api/books/{id}/ratings
GET    /api/books/{id}/ratings/summary
```

---

## 11. Learning Roadmap

## Phase 1 — Basic Go API

Goal:

Build a simple Go HTTP API.

Topics:

- Go project structure.
- HTTP handlers.
- JSON request/response.
- Error handling.
- Environment variables.
- Basic routing with Chi.

Deliverables:

- Health check endpoint.
- Basic book list endpoint with mock data.

---

## Phase 2 — PostgreSQL Integration

Goal:

Persist books, users, genres, comments, and ratings.

Topics:

- PostgreSQL basics.
- Migrations.
- pgx.
- Repository/service pattern.
- SQL relationships.
- Many-to-many modeling.

Deliverables:

- Database schema.
- Book CRUD.
- Genre CRUD.
- Book-genre relation.

---

## Phase 3 — Auth and Roles

Goal:

Add user registration, login, and role-based access control.

Topics:

- Password hashing.
- JWT.
- Middleware.
- Admin-only routes.
- Authentication context.

Deliverables:

- Register/login.
- Protected routes.
- Admin-only endpoints.

---

## Phase 4 — Book Suggestions and Moderation

Goal:

Allow users to suggest books and admins to approve/reject them.

Topics:

- Workflow states.
- Moderation queues.
- Admin actions.
- Status transitions.

Deliverables:

- Suggest book endpoint.
- Pending books endpoint.
- Approve/reject endpoints.

---

## Phase 5 — Comments and Ratings

Goal:

Add comment and rating functionality.

Topics:

- One-to-many relationships.
- Unique constraints.
- Aggregations.
- Soft delete.
- Admin moderation.

Deliverables:

- Comment creation.
- Admin comment deletion.
- Rating creation/update.
- Average rating and rating count.

---

## Phase 6 — Weighted Rating Algorithm

Goal:

Rank books fairly using a weighted score.

Topics:

- Bayesian average.
- Aggregation queries.
- Score recalculation.
- Sorting by calculated values.

Deliverables:

- Weighted score calculation.
- Top-rated books endpoint.
- Rating recalculation service.

---

## Phase 7 — Redis Cache

Goal:

Use Redis only where it solves a real performance or state problem.

Topics:

- Cache keys.
- TTL.
- Cache invalidation.
- Rate limiting.
- Redis client in Go.

Deliverables:

- Cache top-rated books.
- Invalidate cache when ratings change.
- Basic rate limit middleware.

---

## Phase 8 — Docker Compose

Goal:

Run the whole backend stack locally with one command.

Topics:

- Dockerfile.
- docker-compose.yml.
- Networks.
- Volumes.
- Environment variables.

Deliverables:

- Go API container.
- PostgreSQL container.
- Redis container.
- Local development environment.

---

## Phase 9 — Frontend

Goal:

Create a simple UI for users and admin.

Topics:

- API integration.
- Auth state.
- Admin pages.
- Form validation.

Deliverables:

- Book list page.
- Book detail page.
- Login/register.
- Suggest book form.
- Admin pending book page.

---

## Phase 10 — Deployment and CI/CD

Goal:

Deploy the application with mostly free or low-cost tools.

Topics:

- GitHub Actions.
- Build and test pipeline.
- Docker image build.
- Environment management.
- Basic deployment.

Deliverables:

- GitHub Actions workflow.
- Deployed backend.
- Deployed frontend.
- Cloud database/cache/storage integration.

---

## Phase 11 — NGINX

Goal:

Use NGINX as a reverse proxy.

Topics:

- Reverse proxy.
- Static frontend serving.
- API proxying.
- HTTPS basics.

Deliverables:

- NGINX in front of frontend and API.
- Single-domain local/prod-like setup.

---

## Phase 12 — Kubernetes

Goal:

Move the Docker Compose-based system to Kubernetes for learning.

Topics:

- Pod.
- Deployment.
- Service.
- ConfigMap.
- Secret.
- Ingress.
- Volumes.

Deliverables:

- Kubernetes manifests.
- Local cluster using kind or minikube.
- API deployment.
- PostgreSQL/Redis for local learning.
- Ingress routing.

---

## 12. Suggested Extra Features

These are optional features that can be added later.

## 12.1 Reading Lists

Users can create lists:

```text
Want to Read
Currently Reading
Completed
Dropped
Favorites
```

## 12.2 Personal Notes

Users can write private notes for each book.

Only the note owner can view their notes.

## 12.3 Quotes

Users can save quotes from books.

Fields:

- Book ID
- Page number
- Quote text
- Personal note

## 12.4 Reports

Users can report inappropriate comments.

Admin can review reports.

## 12.5 Audit Logs

Admin actions should be logged.

Examples:

- Admin approved book.
- Admin rejected book.
- Admin deleted comment.
- Admin changed genre.

## 12.6 Book Import

Users can import books from:

- ISBN lookup API.
- CSV file.
- Manual form.

## 12.7 Recommendation System

Recommend books based on:

- Genres user likes.
- Highly rated books.
- Similar users.
- Books with similar tags.

## 12.8 AI Features

Optional AI-powered features:

- Summarize personal notes.
- Generate book description drafts.
- Suggest genres from book description.
- Recommend similar books.
- Ask questions over personal book notes.

## 12.9 Activity Feed

Show recent activity:

- New approved books.
- Recent comments.
- Top-rated books this week.
- Most discussed books.

## 12.10 Notifications

Notify users when:

- Their suggested book is approved.
- Their suggested book is rejected.
- Someone replies to their comment.
- Admin deletes their comment.

---

## 13. Free or Low-Cost Tooling Strategy

The project should be built with free or nearly free tools where possible.

## Local First

The first version should run locally using Docker Compose.

This keeps the project free while learning:

- Go
- PostgreSQL
- Redis
- Docker
- Local networking
- Migrations
- API design

## Possible Free/Low-Cost Cloud Services Later

Database:

```text
Supabase Free
Neon Free
Local PostgreSQL for development
```

Redis:

```text
Local Redis
Upstash Free
```

Storage:

```text
Local filesystem
MinIO locally
Cloudflare R2 free tier
Supabase Storage free tier
```

Frontend hosting:

```text
Vercel Free
Netlify Free
Cloudflare Pages Free
```

Backend hosting:

```text
Render Free or low-cost tier
Fly.io free/low-cost depending on current limits
Railway low-cost depending on current limits
Cheap VPS later if needed
```

CI/CD:

```text
GitHub Actions
```

Important note:

> Free tiers change over time. Before deploying, check current limits and pricing.

---

## 14. Engineering Principles

## 14.1 Keep It Simple First

Do not start with Kubernetes, queues, microservices, or advanced infrastructure.

Start with:

```text
Go API + PostgreSQL
```

Then add complexity only when there is a reason.

## 14.2 Monolith First

The first version should be a modular monolith.

Do not split into microservices early.

Recommended modules:

```text
auth
users
books
genres
comments
ratings
admin
```

## 14.3 Clear Layers

Use a simple layered structure:

```text
Handler → Service → Repository → Database
```

CORRECTION (from review): this is the **target**, not a day-one rule. **Skip the Service
layer for pure CRUD** (Phase 2 books/genres) where it would only forward calls — an anemic
pass-through layer is a cargo-cult habit a TS/OOP background invites. Add a Service layer only
where real logic lives: auth (Phase 3), moderation/status transitions (Phase 4),
weighted-score recalculation (Phase 6). Define interfaces **at the consumer**, only once a
test fake or a second implementation needs one. See [GO_CONVENTIONS.md](GO_CONVENTIONS.md) §4.

Handler:

- HTTP request/response.
- JSON parsing.
- Status codes.

Service:

- Business logic.
- Authorization rules.
- Rating calculations.
- Workflow decisions.

Repository:

- SQL queries.
- Database interaction.

## 14.4 Explicit Errors

Errors should be handled clearly.

Avoid hiding errors behind too much abstraction.

## 14.5 No Technology Without a Reason

Before adding a tool, answer:

```text
What problem does this solve in this project?
```

---

## 15. Definition of Done for MVP

The MVP is complete when:

- Users can register and login.
- Admin exists.
- Admin can create genres.
- Users can suggest books.
- Admin can approve/reject books.
- Approved books are publicly visible.
- Users can comment on books.
- Only admin can delete comments.
- Users can rate books from 1 to 10.
- Book average rating and rating count are calculated.
- Weighted rating score is calculated.
- Top-rated books use weighted score, not simple average.
- PostgreSQL is used for persistent data.
- The API is written in Go.
- The app runs locally.

---

## 16. Codex Working Instructions

When using Codex inside VS Code, follow these principles:

1. Do not generate the whole project at once.
2. Work phase by phase.
3. Prefer small, reviewable changes.
4. After each change, explain what changed and why.
5. Keep the code beginner-friendly but production-conscious.
6. Avoid overengineering.
7. Do not introduce Kubernetes, Redis, Docker, or queues before the related phase.
8. Use Go idioms where possible.
9. Keep errors explicit.
10. Write meaningful tests for important business logic, especially rating calculations.
11. Follow [GO_CONVENTIONS.md](GO_CONVENTIONS.md) (ctx-first, `%w` errors + sentinels,
    hand-wired DI, graceful shutdown, table-driven tests, conditional layering, the security
    baseline).
12. Run `gofmt`, `go vet`, and `golangci-lint`; keep `go test -race` green.
13. At the end of each phase, fill the relevant [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) placeholder
    with a real `path:Symbol` reference + a short reflection (confirm/falsify), and add ≥1
    entry to its §15 ("where TS/JS wins"). See the per-phase checklist in that doc.

Suggested first Codex prompt:

```text
Read PROJECT_BRIEF.md. Create the initial Go backend structure for Phase 1 only. Use Go, Chi router, a health check endpoint, basic configuration loading from environment variables, and a clean folder structure. Do not add PostgreSQL, Redis, Docker, auth, or frontend yet. Keep the code simple and explain the structure after creating it.
```

Suggested second Codex prompt:

```text
Continue from Phase 1. Add PostgreSQL support for books and genres only, using a local Docker Compose Postgres for dev (prod will be Supabase; same code, different DATABASE_URL). Add migrations using goose, define queries in sql/queries and generate type-safe code with sqlc, connect with pgxpool, and implement CRUD as a thin vertical slice first (one Book end-to-end through all layers + a test) before fanning out to genres and the many-to-many relation. No ORM. Do not add auth, Redis, or ratings yet.
```

Suggested third Codex prompt:

```text
Add authentication and role-based authorization. Implement register, login, JWT middleware, and admin-only route protection. Keep the existing structure clean and add tests for password hashing and authorization behavior where practical.
```

---

## 17. Final Project Direction

BookWise should grow slowly and intentionally.

The project should begin as a simple Go backend and gradually become a realistic backend system.

The most important learning outcome is not memorizing commands or tools.

The most important learning outcome is understanding:

> Which problem does each technology solve, and when is it worth adding?

---

## 18. Review Addendum — Decisions & Corrections (2026-06)

This section consolidates a multi-lens review. Inline sections above were edited where the
original text would otherwise contradict a decision; the rest lives here. The **original
unedited brief** is preserved in git history (the initial commit) — recoverable via
`git show <initial-commit>:docs/PROJECT_BRIEF.md`.

Companion docs created alongside this addendum:

- [GO_CONVENTIONS.md](GO_CONVENTIONS.md) — *how* we write Go (idioms + security baseline).
- [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) — *why* Go vs TS/JS, a living doc filled in per phase.

### 18.1 Roadmap corrections (§11)

- **Phase 0 (new):** `git init`, a Go `.gitignore` protecting `.env`, commit the brief. _Done
  — this repo is now under `mtgezgin/BookHub`._
- **Docker Compose for backing services before Phase 2:** stand up the **dev** Postgres (and
  later Redis) in Compose early (§7.6) so you never hand-install a DB; the Go API still runs
  via `go run` for fast feedback. Full API containerization stays at Phase 8. This teaches
  Docker in two correct stages.
- **Testing is first-class from Phase 1:** every phase's Definition of Done includes tests.
  Phase 1 → first table-driven + `httptest` handler test; Phase 2 → repository/integration
  tests against the Compose Postgres; Phase 6 → the weighted-rating pure function as the
  flagship tested example. Run with `-race`.
- **Phase 2 = thin vertical slice first:** one Book wired end-to-end (migration → repo →
  (service if needed) → handler → JSON → a test) before fanning out to genres and the
  many-to-many relation.
- **Fold into Phase 1 (cheap production hygiene):** graceful `http.Server` shutdown, fail-fast
  config validation, a documented typed-error → HTTP-status convention, and structured-logging
  + request-ID middleware. (See [GO_CONVENTIONS.md](GO_CONVENTIONS.md) §2, §5, §10.)

### 18.2 Scope tiering (de-risk burnout)

- **MVP = Phases 1–7** (the §15 Definition of Done) is the declared **win condition**.
- **Phases 8–12** are an optional infra-learning track, each independently skippable.
- **§12 extras** (especially the AI features §12.8) are post-mastery and outside the
  Go-backend learning objective. Stopping at the MVP counts as success, not failure.

### 18.3 Schema specifics (§9 — decide with the migrations)

- **Types/constraints:** `score SMALLINT CHECK (score BETWEEN 1 AND 10)`; averages `NUMERIC`
  (not `FLOAT` — sort drift); `books.slug UNIQUE`; status/role as `TEXT + CHECK (... IN (...))`
  **not native ENUM** (easier to evolve when `MODERATOR`/new statuses arrive); `timestamptz`
  everywhere; PK = `bigint GENERATED ALWAYS AS IDENTITY` by default (choose UUIDv7 only if you
  specifically want to learn that tradeoff).
- **Indexes (Postgres does NOT auto-index FKs):** partial index `books(status) WHERE
  status='APPROVED'`; an index for the top-rated sort; `comments(book_id, created_at)`; every
  FK you join/filter on; `book_genres` PK `(book_id, genre_id)` **plus** a separate
  `(genre_id)` index for "filter books by genre". Plan these so later Redis adoption (Phase 7)
  is driven by real need, not a missing index.
- **Soft-delete single source of truth:** for comments (§6.5), use `deleted_at IS NULL` as the
  deletion truth and reserve `status` for moderation state (VISIBLE/HIDDEN); standardize reads
  on `WHERE deleted_at IS NULL`.

### 18.4 Repo structure notes (§8)

- Two SQL dirs, distinct roles: `backend/sql/queries/*.sql` (sqlc input) vs
  `backend/migrations/*.sql` (goose); `backend/sqlc.yaml` at the backend root.
- **Avoid the per-domain import cycle:** a comment references a user and a book; a rating
  references both. Reference cross-domain entities **by ID** (`BookID`, `UserID`), or put
  shared entities in a small `domain/` package. Domain packages depend on shared types, not on
  each other.

### 18.5 Git hooks (polyglot, most instructive)

Defer hooks to **Phase 1** (no Go code to lint yet). When added, use **`core.hooksPath` + a
committed `.githooks/pre-commit` shell script** (not husky — a Node dependency in a Go repo).
The script branches on changed files: `backend/**.go` → `gofmt`/`go vet`/`golangci-lint`/`go
test`; `frontend/**` → `eslint`/`prettier`/`tsc`. Most transparent (teaches what a hook is)
and naturally polyglot. `lefthook` is the ergonomic step-up later. Record as an ADR.

### 18.6 Living comparison doc — per-phase fill-in

At the end of every phase, update [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md): replace the relevant
`[Phase X — path — TBD]` placeholder with a real `path:Symbol` reference + a short reflection
(confirm/falsify), and grow its §15 ("where TS/JS wins"). The per-phase section map lives in
that doc's "Living-document mechanism".

