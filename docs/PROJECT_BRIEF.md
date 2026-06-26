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

Book should store:

- Average rating
- Rating count
- Weighted score

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
Database driver: pgx
Query layer: sqlc or pgx manually at first
Migrations: goose or golang-migrate
Validation: go-playground/validator or custom validation
Auth: JWT
Logging: slog or zerolog
Config: environment variables
```

Recommended starting choice:

```text
Go + Chi + pgx + goose
```

Reason:

- Chi keeps HTTP concepts visible.
- pgx is a strong PostgreSQL driver.
- goose is simple for migrations.
- This stack avoids too much framework magic.

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
- Can be run locally with Docker.
- Can be hosted for free or low cost using services like Supabase or Neon.

---

## 7.3 Cache

```text
Redis
```

Local:

```text
Docker Redis container
```

Cloud later:

```text
Upstash Redis free tier or similar
```

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

Use Docker Compose for:

- Go API
- PostgreSQL
- Redis
- Optional MinIO

Initial local architecture:

```text
Frontend
   ↓
Go API
   ↓
PostgreSQL
   ↓
Redis
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

Suggested first Codex prompt:

```text
Read PROJECT_BRIEF.md. Create the initial Go backend structure for Phase 1 only. Use Go, Chi router, a health check endpoint, basic configuration loading from environment variables, and a clean folder structure. Do not add PostgreSQL, Redis, Docker, auth, or frontend yet. Keep the code simple and explain the structure after creating it.
```

Suggested second Codex prompt:

```text
Continue from Phase 1. Add PostgreSQL support for books and genres only. Add migrations using goose, connect with pgx, and implement basic book and genre CRUD. Do not add auth, Redis, Docker, or ratings yet.
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

