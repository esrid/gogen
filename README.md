# gogen

Rails-style Go project generator. Generates production-ready hexagonal architecture projects with a single command.

## Install

```sh
go install github.com/esrid/gogen@latest
```

Requires Go 1.26+.

## Commands

| Command | Alias | Description |
|---------|-------|-------------|
| `gogen new` | | Create a new project |
| `gogen generate migration` | `gogen g migration` | Add a migration file |
| `gogen generate auth` | `gogen g auth` | Add auth to an existing project |
| `gogen generate scaffold` | `gogen g s` | Generate full CRUD for a model |
| `gogen generate attribute` | `gogen g a` | Add fields to an existing scaffold |
| `gogen generate api` | `gogen g api` | Add JSON API handler to an SSR scaffold |
| `gogen generate controller` | `gogen g controller` | Generate a simple page/API controller |
| `gogen destroy scaffold` | `gogen d s` | Remove a generated scaffold |
| `gogen destroy controller` | `gogen d controller` | Remove a generated controller |

---

## gogen new

Create a new project. Prompts for anything not supplied via flags.

```sh
gogen new <project-name> [flags]
```

**Flags**

| Flag | Short | Description |
|------|-------|-------------|
| `--module` | `-m` | Go module path (e.g. `github.com/you/myapp`) |
| `--db` | `-d` | Database: `sqlite` or `postgres` |
| `--render` | `-r` | Render mode: `ssr`, `api`, or `both` |
| `--auth` | | Include authentication |
| `--no-auth` | | Skip auth (skip the prompt) |
| `--force` / `--dry-run` / `--skip` | | File conflict behaviour |

**Examples**

```sh
# Interactive — prompts for everything
gogen new myapp

# Fully non-interactive
gogen new myapp -m github.com/you/myapp -d sqlite -r ssr --auth
gogen new myapi -m github.com/you/myapi -d postgres -r api --no-auth
```

**What gets generated**

```
myapp/
├── main.go                              # bootstrap.Run()
├── go.mod                               # go 1.26
├── .env
├── .air.toml
├── Makefile
├── Dockerfile
├── docker-compose.yml
├── .gogen.yaml                          # project metadata for generate commands
├── bootstrap/
│   ├── app.go                           # Run() — DB init + server start
│   ├── config.go                        # env-based config
│   ├── server.go                        # graceful shutdown
│   ├── router.go                        # chi router + middleware (auto-updated)
│   └── wire_gen.go                      # Handlers struct + WireHandlers (auto-updated)
├── internal/
│   ├── domain/
│   │   ├── errors.go                    # ErrNotFound, ErrConflict, ErrUnauthorized, etc.
│   │   ├── session_port.go              # SessionStore, SessionService interfaces
│   │   ├── user.go                      # User, NewUser, Validate(), context helpers (with --auth)
│   │   ├── auth_port.go                 # UserStore, UserService interfaces (with --auth)
│   │   └── email_port.go                # EmailProvider interface (with --auth)
│   ├── application/
│   │   ├── auth_service.go              # login/signup/reset logic (with --auth)
│   │   └── session_service.go           # in-memory session cache (with --auth)
│   ├── utils/
│   │   ├── http_utils.go                # WriteJSON, DecodeJSON, cookies
│   │   └── validation.go                # HashPassword, PreHashing (with --auth)
│   └── adapters/
│       ├── api/
│       │   ├── middleware.go            # SecurityHeaders, LimitRequestBody, NoCache
│       │   ├── middleware_auth.go       # RequireAuth (with --auth)
│       │   ├── errors.go                # writeError — maps domain errors to HTTP status codes
│       │   └── auth_handler.go          # login/signup/reset routes (with --auth)
│       ├── web/                         # SSR only
│       │   └── renderer.go              # web.Render, web.RenderError — templ renderer
│       ├── db/
│       │   ├── store.go                 # DB connection + pool
│       │   ├── migrations.go            # goose embed runner
│       │   ├── auth_store.go            # user/session queries (with --auth)
│       │   └── migrations/
│       │       └── 00001_init.sql
│       └── external/email/
│           └── noop.go                  # email provider stub (with --auth)
└── web/                                 # SSR only
    ├── static.go
    ├── static/robots.txt
    └── components/
        ├── components.templ             # shared components (nav, etc.)
        ├── landing.templ
        ├── dashboard.templ              # (with --auth)
        └── auth/                        # (with --auth)
            ├── login.templ
            ├── signup.templ
            ├── forgot_password.templ
            ├── reset_password.templ
            └── settings.templ
```

**Stack**

| Concern | Library |
|---------|---------|
| Router | [chi](https://github.com/go-chi/chi) |
| SQLite | [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO) |
| Postgres | [pgx/v5](https://github.com/jackc/pgx) |
| Migrations | [goose v3](https://github.com/pressly/goose) (embedded SQL) |
| Templates | [templ](https://templ.guide) — type-safe Go SSR components |
| Password | bcrypt with sha256 pre-hashing |

**Docker**

Standard 2-stage build using `golang:1.26-alpine`:

```
Stage 1 — builder    go build (CGO_ENABLED=0)
Stage 2 — runtime    alpine:3.21 + ca-certificates + tzdata
```

Both SQLite (`modernc.org/sqlite`) and Postgres (`pgx/v5`) are pure Go — no CGO needed.

---

## gogen generate migration

Create a numbered migration file in `internal/adapters/db/migrations/`.

```sh
gogen g migration <name>
```

Must be run from inside a gogen project (reads `.gogen.yaml` for DB dialect).

**Example**

```sh
gogen g migration add_avatar_to_users
# creates: internal/adapters/db/migrations/00002_add_avatar_to_users.sql
```

---

## gogen generate auth

Add authentication to a project that was created without it.

```sh
gogen g auth
```

Must be run from inside a gogen project with `auth: false` in `.gogen.yaml`.

**What it does**

- Creates all auth files (domain, application, utils, handler, store, email stub)
- Regenerates `main.go`, `bootstrap/router.go`, and `bootstrap/wire_gen.go` to wire auth in
- Re-wires all existing scaffolds in `wire_gen.go` and `router.go`
- Creates a new migration (`NNNNN_add_auth.sql`) with the auth tables
- Adds SSR auth templ components if the project uses SSR
- Updates `.gogen.yaml` to `auth: true`

**Auth tables created**

- `users` — email, password_hash, full_name, avatar_url, timezone, soft delete
- `sessions` — token-based, 30-day expiry
- `password_reset_tokens` — single-use, expiring
- `password_reset_attempts` — rate limiting (3 per hour per email)

**Auth routes**

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/auth/login` | Login page (SSR) |
| `POST` | `/auth/login` | Authenticate |
| `GET` | `/auth/signup` | Signup page (SSR) |
| `POST` | `/auth/signup` | Register |
| `POST` | `/auth/logout` | Clear session |
| `POST` | `/auth/forgot-password` | Request reset link |
| `POST` | `/auth/reset-password` | Reset with token |
| `GET` | `/auth/settings` | Settings page (authenticated) |
| `POST` | `/auth/settings/password` | Change password (authenticated) |
| `POST` | `/auth/settings/delete` | Delete account (authenticated) |

---

## gogen generate scaffold

Generate a full CRUD resource: migration, domain, port, store, service, and HTTP handler.

```sh
gogen g scaffold <ModelName> [field:type ...] [--protected]
```

Must be run from inside a gogen project (reads `.gogen.yaml`). Auto-updates `bootstrap/router.go` and `bootstrap/wire_gen.go`.

**Field types**

| Type | Go type | SQLite | Postgres |
|------|---------|--------|----------|
| `string` / `text` | `string` | `TEXT NOT NULL DEFAULT ''` | `TEXT NOT NULL DEFAULT ''` |
| `int` | `int` | `INTEGER NOT NULL DEFAULT 0` | `INTEGER NOT NULL DEFAULT 0` |
| `bool` | `bool` | `INTEGER NOT NULL DEFAULT 0` | `BOOLEAN NOT NULL DEFAULT false` |
| `float` | `float64` | `REAL NOT NULL DEFAULT 0` | `NUMERIC NOT NULL DEFAULT 0` |
| `time` | `time.Time` | `DATETIME` | `TIMESTAMPTZ` |
| `uuid` | `string` | `TEXT NOT NULL DEFAULT ''` | `UUID NOT NULL DEFAULT gen_random_uuid()` |
| `references` | `string` | `TEXT NOT NULL REFERENCES {table}(id) ON DELETE CASCADE` | `UUID NOT NULL REFERENCES {table}(id) ON DELETE CASCADE` |
| `user:references` | `string` | `TEXT REFERENCES users(id) ON DELETE SET NULL` | `UUID REFERENCES users(id) ON DELETE SET NULL` |

`references` is convention-based: `post:references` → `post_id` column → FK to `posts(id)`. Table name is auto-pluralized (`category` → `categories`).

`user:references` (the literal form, column `user_id`) is generated as **nullable**. On non-protected routes, anonymous callers produce a record with `user_id = NULL`; authenticated callers get their ID injected automatically. On `--protected` routes, `RequireAuth` ensures a user is always present so NULL never occurs in practice.

When you need two FK columns pointing to the same table, use the aliased form `alias:model:references`:

```sh
# Two FKs to users: user_id and manager_id both reference users(id)
gogen g scaffold Employee user:references manager:user:references

# Two FKs to words: word_id and translate_id both reference words(id)
gogen g scaffold WordAssociation word:references translate:word:references
```

- `manager:user:references` → column `manager_id`, FK to `users(id)`, route `/by-manager/{managerID}`
- `translate:word:references` → column `translate_id`, FK to `words(id)`, route `/by-translate/{translateID}`

Aliased refs to `users` are treated as regular (non-auth-scoped) refs. Only the literal `user:references` (column `user_id`) triggers auth scoping with `--protected`.

Go field names follow standard acronym rules: `user_id` → `UserID`, `avatar_url` → `AvatarURL`.

**Example**

```sh
gogen g scaffold Post title:string body:text user:references published:bool
```

**Generated files**

```
internal/domain/post.go
internal/domain/post_port.go
internal/application/post_service.go
internal/adapters/db/post_store.go
internal/adapters/api/post_handler.go
internal/adapters/db/migrations/NNNNN_create_posts.sql
```

With `--render ssr` or `--render both`, a templ component folder is also created:

```
web/components/posts/
├── index.templ
├── show.templ
├── new.templ
└── edit.templ
```

With `--render both`, an SSR web handler and API handler are both generated:

```
internal/adapters/web/post_handler.go      # SSR handler (GET /posts → HTML)
internal/adapters/api/post_handler.go      # API handler (GET /api/posts → JSON)
```

**Validation**

Generated domain structs include a `Validate()` method that checks required string/reference fields. The service layer calls it automatically on create and update — no validation logic leaks into handlers or utils.

```go
// internal/domain/post.go
func (m Post) Validate() error {
    if strings.TrimSpace(m.Title) == "" {
        return fmt.Errorf("%w: title is required", ErrInvalidInput)
    }
    return nil
}
```

**HTTP endpoints**

| Method | Path | Handler |
|--------|------|---------|
| `GET` | `/posts` | list all |
| `POST` | `/posts` | create |
| `GET` | `/posts/{id}` | get one |
| `PUT` / `POST` | `/posts/{id}` | update (PUT for API, POST for SSR forms) |
| `DELETE` / `POST` | `/posts/{id}/delete` | delete (DELETE for API, POST for SSR forms) |

**Association queries**

When a `references` field is present, gogen generates filtered query methods at every layer.

For non-user refs (e.g. `post:references`), a list-by route is also exposed:

| Layer | Method |
|-------|--------|
| Store | `ListCommentsByPostID(ctx, postID)` |
| Service | `ListByPostID(ctx, postID)` |
| Handler | `GET /comments/by-post/{postID}` |

`user:references` is treated specially — no public list-by route is generated. Instead, `--protected` + `user:references` scopes the default `list` endpoint to the current user automatically.

Multiple refs each get their own method and route:

```sh
gogen g scaffold Comment body:text post:references category:references
# GET /comments/by-post/{postID}
# GET /comments/by-category/{categoryID}
```

**`--protected` flag**

Requires `auth: true` in `.gogen.yaml`. Mounts the scaffold routes inside the `RequireAuth` middleware group.

```sh
gogen g scaffold Post title:string body:text user:references --protected
```

Three things happen automatically:

1. Routes are mounted inside the protected group (behind `RequireAuth`)
2. `create` injects the current user's ID into the `user_id` field (when `user:references` is present)
3. `list` uses `service.ListByUserID(ctx, userID)` instead of `service.List(ctx)` — scoped to current user

**Auto-wiring**

After generation, `bootstrap/wire_gen.go` and `bootstrap/router.go` are updated automatically — no manual edits needed:

```go
// bootstrap/wire_gen.go (auto-generated)
type Handlers struct {
    Post *api.PostHandler
}

func WireHandlers(dbStore *db.Store, logger *slog.Logger) *Handlers {
    h := &Handlers{}
    postSvc := application.NewPostService(dbStore)
    h.Post = api.NewPostHandler(postSvc, logger)
    return h
}

// bootstrap/router.go (mount injected automatically)
if h.Post != nil {
    h.Post.Register(r)
}
```

---

## gogen generate attribute

Add new fields to an existing scaffold. Updates the domain, store, and handler; creates an `ALTER TABLE` migration; and regenerates SSR templ components if applicable.

```sh
gogen g attribute <ModelName> field:type [field:type ...]
```

Must be run from inside a gogen project. The model must already exist (created via `gogen g scaffold`).

**Example**

```sh
gogen g attribute Post published:bool views:int
```

What it does:
- Creates `NNNNN_add_published_views_to_posts.sql` with `ALTER TABLE` statements
- Regenerates `post.go`, `post_store.go`, `post_handler.go` with the new fields
- Regenerates SSR templ components (`web/components/posts/*.templ`) only when `--views` is passed
- Updates `.gogen.yaml` with the new field list

Accepts the same field types as `gogen g scaffold`. Duplicate fields are rejected.

> SSR views are not regenerated by default to preserve any customisations you've made. Pass `--views` to overwrite them.

---

## gogen generate api

Add a JSON API handler to an existing SSR scaffold. Useful when you want to expose a REST API alongside your server-rendered pages.

```sh
gogen g api <ModelName>
```

Must be run from inside a gogen project with `render: ssr`. The scaffold must already exist.

**Example**

```sh
gogen g api Post
# generates: internal/adapters/api/post_api_handler.go
# routes:    GET /api/posts, POST /api/posts, GET /api/posts/{id}, etc.
```

Updates `.gogen.yaml` and rewires `bootstrap/wire_gen.go` and `bootstrap/router.go` automatically.

---

## gogen generate controller

Generate a simple page or API controller with no model, store, or service — useful for static-ish pages like contact, about, terms, etc.

```sh
gogen g controller <Name> [--protected] [--route /path]
```

**Flags**

| Flag | Description |
|------|-------------|
| `--protected` | Mount behind `RequireAuth` middleware |
| `--route` | Custom route path (default: `/<name>`) |

**Examples**

```sh
gogen g controller Contact
# GET /contact → web/components/contact/page.templ

gogen g controller Dashboard --protected --route /dashboard
# GET /dashboard → protected, web/components/dashboard/page.templ
```

**Generated files (SSR)**

```
internal/adapters/web/contact_handler.go
web/components/contact/page.templ
```

**Generated files (API)**

```
internal/adapters/api/contact_handler.go
```

Auto-wired into `bootstrap/wire_gen.go` and `bootstrap/router.go`.

---

## gogen destroy scaffold

Remove all files generated by `gogen g scaffold`. Updates `bootstrap/wire_gen.go` and `bootstrap/router.go` automatically.

```sh
gogen d scaffold <ModelName>
```

**Example**

```sh
gogen d scaffold Post
```

Removes:

```
internal/domain/post.go
internal/domain/post_port.go
internal/application/post_service.go
internal/adapters/db/post_store.go
internal/adapters/api/post_handler.go
internal/adapters/api/post_api_handler.go   # both/api mode
internal/adapters/web/post_handler.go       # ssr/both mode
web/components/posts/                        # SSR only
internal/adapters/db/migrations/*_create_posts.sql
```

**Migration warning**

If a matching migration file is found it is deleted, but a warning is printed:

```
warning migration 00003_create_posts.sql was deleted — run goose down manually if already applied
```

If you already ran `goose up` against a real database, run the down migration first:

```sh
goose -dir internal/adapters/db/migrations sqlite3 myapp.db down
gogen d scaffold Post
```

`--dry-run` prints what would be removed without deleting anything.

---

## gogen destroy controller

Remove all files generated by `gogen g controller`. Updates `bootstrap/wire_gen.go` and `bootstrap/router.go` automatically.

```sh
gogen d controller <Name>
```

**Example**

```sh
gogen d controller Contact
```

Removes:

```
internal/adapters/web/contact_handler.go   # SSR
internal/adapters/api/contact_handler.go   # API
web/components/contact/                    # SSR
```

---

## Templates (SSR)

Generated projects use [templ](https://templ.guide) — a type-safe Go templating language that compiles to plain Go functions.

**Define a component** (`web/components/card/card.templ`):
```go
package card

templ Card(title, body string) {
    <div class="card">
        <h2>{ title }</h2>
        <p>{ body }</p>
    </div>
}
```

**Use it in a page** (`web/components/posts/index.templ`):
```go
package posts

import "myapp/web/layouts"
import "myapp/web/components/card"

templ Index(posts []domain.Post) {
    @layouts.Layout("Posts") {
        for _, p := range posts {
            @card.Card(p.Title, p.Body)
        }
    }
}
```

**Render from a handler**:
```go
web.Render(w, r, posts.Index(items))
```

**Error rendering**:
```go
web.RenderError(w, r, err)   // maps domain errors to HTTP status + error page
```

`web.RenderError` maps domain errors to the correct HTTP status automatically:

| Domain error | HTTP status |
|---|---|
| `ErrNotFound` | 404 |
| `ErrUnauthorized` | 401 |
| `ErrForbidden` | 403 |
| `ErrConflict` | 409 |
| anything else | 500 |

---

## Error handling

Domain errors are defined in `internal/domain/errors.go` and mapped to HTTP status codes only at the adapter layer — never inside domain or application code.

**API adapter** (`internal/adapters/api/errors.go`):
```go
writeError(w, err)   // maps domain errors to JSON error responses
```

**SSR adapter** (`internal/adapters/web/renderer.go`):
```go
web.RenderError(w, r, err)   // maps domain errors to HTML error pages
```

---

## Global flags

Available on all commands:

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Overwrite existing files |
| `--dry-run` | `-p` | Preview without writing |
| `--skip` | `-s` | Skip existing files silently |
