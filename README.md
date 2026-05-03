# gogen

Rails-style Go project generator. Generates production-ready hexagonal architecture projects with a single command.

## Install

```sh
go install github.com/esrid/gogen@latest
```

## Commands

| Command | Alias | Description |
|---------|-------|-------------|
| `gogen new` | | Create a new project |
| `gogen generate migration` | `gogen g migration` | Add a migration file |
| `gogen generate auth` | `gogen g auth` | Add auth to an existing project |
| `gogen generate scaffold` | `gogen g s` | Generate full CRUD for a model |
| `gogen destroy scaffold` | `gogen d s` | Remove a generated scaffold |

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
gogen new myapp --module github.com/you/myapp --db sqlite --render ssr --auth
gogen new myapi --module github.com/you/myapi --db postgres --render api --no-auth
```

**What gets generated**

```
myapp/
├── main.go
├── go.mod
├── .env
├── .air.toml
├── Makefile
├── Dockerfile
├── docker-compose.yml
├── .gogen.yaml                          # project config for generate commands
├── internal/
│   ├── server/
│   │   ├── config.go                    # env-based config
│   │   ├── server.go                    # graceful shutdown
│   │   └── routes.go                    # chi router, middleware, Handler struct
│   ├── adapters/
│   │   ├── http/
│   │   │   ├── middleware.go            # SecurityHeaders, LimitRequestBody, NoCache
│   │   │   ├── middleware_auth.go       # RequireAuth (with --auth)
│   │   │   └── auth_handler.go         # login/signup/reset (with --auth)
│   │   ├── store/
│   │   │   ├── store.go                 # DB connection + pool
│   │   │   ├── migrations.go            # goose embed runner
│   │   │   ├── auth_store.go           # user/session queries (with --auth)
│   │   │   └── migrations/
│   │   │       └── 00001_init.sql
│   │   └── external/email/
│   │       └── noop.go                  # email provider stub (with --auth)
│   └── core/
│       ├── domains/
│       │   ├── errors.go
│       │   └── user.go                  # User struct, context helpers (with --auth)
│       ├── ports/
│       │   ├── session_port.go
│       │   ├── auth_port.go            # (with --auth)
│       │   └── email_port.go           # (with --auth)
│       ├── services/
│       │   ├── auth_service.go         # (with --auth)
│       │   └── session_service.go      # (with --auth)
│       └── utils/
│           ├── http_utils.go            # WriteJSON, DecodeJSON, cookies
│           └── validation.go            # password hashing (with --auth)
└── web/                                 # SSR only
    ├── renderer.go                      # html/template + go:embed
    ├── static.go
    ├── static/robots.txt
    └── templates/
        ├── layout.html                  # {{define "layout"}} with slots
        ├── components/components.html   # {{define "flash"}} etc.
        └── pages/
            ├── landing.html
            ├── error.html
            ├── login.html               # (with --auth)
            ├── signup.html              # (with --auth)
            ├── forgot-password.html     # (with --auth)
            ├── reset-password.html      # (with --auth)
            └── settings.html            # (with --auth)
```

**Stack**

| Concern | Library |
|---------|---------|
| Router | [chi](https://github.com/go-chi/chi) |
| SQLite | [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO) |
| Postgres | [pgx/v5](https://github.com/jackc/pgx) |
| Migrations | [goose v3](https://github.com/pressly/goose) (embedded SQL) |
| Templates | custom `html/template` fork with `{{component}}` / `{{slot}}` / `{{fill}}` |
| Password | bcrypt with sha256 pre-hashing |

---

## gogen generate migration

Create a numbered migration file in `internal/adapters/store/migrations/`.

```sh
gogen g migration <name>
```

Must be run from inside a gogen project (reads `.gogen.yaml` for DB dialect).

**Example**

```sh
gogen g migration add_avatar_to_users
# creates: internal/adapters/store/migrations/00002_add_avatar_to_users.sql
```

SQLite output:
```sql
-- +goose Up
-- +goose StatementBegin

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd
```

Postgres output:
```sql
-- +goose Up


-- +goose Down
```

---

## gogen generate auth

Add authentication to a project that was created without it.

```sh
gogen g auth
```

Must be run from inside a gogen project with `auth: false` in `.gogen.yaml`.

**What it does**

- Creates all auth files (domains, ports, services, handler, store, email stub)
- Regenerates `main.go`, `routes.go`, and `errors.go` to wire auth in
- Creates a new migration (`NNNNN_add_auth.sql`) with the auth tables
- Adds SSR auth pages if the project uses SSR
- Updates `.gogen.yaml` to `auth: true`

**Auth tables created**

- `users` — email, password_hash, full_name, avatar_url, timezone, soft delete
- `sessions` — token-based, 30-day expiry
- `password_reset_tokens` — single-use, expiring
- `password_reset_attempts` — rate limiting (3 per hour per email)

**Auth routes**

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/auth/login` | Login page (SSR) / — (API) |
| `POST` | `/auth/login` | Authenticate |
| `GET` | `/auth/signup` | Signup page (SSR) / — (API) |
| `POST` | `/auth/signup` | Register |
| `POST` | `/auth/logout` | Clear session |
| `POST` | `/auth/forgot-password` | Request reset link |
| `POST` | `/auth/reset-password` | Reset with token |
| `POST` | `/auth/change-password` | Change password (authenticated) |
| `DELETE` | `/auth/delete-account` | Soft-delete account (authenticated) |

---

## gogen generate scaffold

Generate a full CRUD resource: migration, domain, port, store, service, and HTTP handler.

```sh
gogen g scaffold <ModelName> [field:type ...] [--protected]
```

Must be run from inside a gogen project (reads `.gogen.yaml`).

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

`references` is convention-based: `user:references` → `user_id` column → FK to `users(id)`. Table name is auto-pluralized (`category` → `categories`, `post` → `posts`).

Go field names follow standard acronym rules: `user_id` → `UserID`, `avatar_url` → `AvatarURL`.

**Example**

```sh
gogen g scaffold Post title:string body:text user:references published:bool
```

Generates:

```
internal/core/domains/post.go
internal/core/ports/post_port.go
internal/adapters/store/post_store.go
internal/core/services/post_service.go
internal/adapters/http/post_handler.go
internal/adapters/store/migrations/NNNNN_create_posts.sql
```

**HTTP endpoints**

| Method | Path | Handler |
|--------|------|---------|
| `GET` | `/posts` | list all |
| `POST` | `/posts` | create |
| `GET` | `/posts/{id}` | get one |
| `PUT` | `/posts/{id}` | update |
| `DELETE` | `/posts/{id}` | delete |

**Association queries**

When a `references` field is present, gogen generates a filtered query method at every layer for each foreign key.

`user:references` on `Post` generates:

| Layer | Method | SQL |
|-------|--------|-----|
| Store | `ListPostsByUserID(ctx, userID)` | `WHERE user_id = ?` |
| Service | `ListByUserID(ctx, userID)` | delegates to store |

Multiple references each get their own method. Example with two refs:

```sh
gogen g scaffold Comment body:text user:references post:references
```

Generates both `ListCommentsByUserID` and `ListCommentsByPostID` in store and service.

**`--protected` flag**

Requires `auth: true` in `.gogen.yaml`. Adds a session user check to every handler method.

```sh
gogen g scaffold Post title:string body:text user:references --protected
```

Three things happen automatically:

1. Every handler checks `domains.GetContextUser(r.Context())` — returns 401 if not authenticated
2. `create` injects the current user's ID into the `user_id` field (when `user:references` is present)
3. `list` uses `service.ListByUserID(ctx, userID)` instead of `service.List(ctx)` — scoped to current user

The wiring hint shows the mount inside the `RequireAuth` protected group.

**Wiring (printed after generation)**

```go
// 1. internal/server/routes.go — Handler struct
Post *api.PostHandler

// 2. internal/server/routes.go — NewRouter
// public:
if h.Post != nil {
    r.Mount("/posts", h.Post.Route())
}
// or inside the protected r.Group (with --protected):
if h.Post != nil {
    r.Mount("/posts", h.Post.Route())
}

// 3. main.go
postService := services.NewPostService(dbStore)
handlers.Post = api.NewPostHandler(postService)
```

---

## gogen destroy scaffold

Remove all files generated by `gogen g scaffold`. Use this to undo a scaffold if you made a mistake.

```sh
gogen d scaffold <ModelName>
```

**Example**

```sh
gogen d scaffold Post
```

Removes:

```
internal/core/domains/post.go
internal/core/ports/post_port.go
internal/adapters/store/post_store.go
internal/core/services/post_service.go
internal/adapters/http/post_handler.go
internal/adapters/store/migrations/*_create_posts.sql
```

**Migration warning**

If a matching migration file is found it is deleted, but a warning is printed:

```
warning migration 00003_create_posts.sql was deleted — run goose down manually if already applied
```

If you already ran `goose up` against a real database, run the down migration first before destroying:

```sh
goose -dir internal/adapters/store/migrations sqlite3 myapp.db down
gogen d scaffold Post
```

`--dry-run` prints what would be removed without deleting anything.

---

## Templates (SSR)

Generated projects use a custom `html/template` fork with a component system.

**Define a component** (`templates/components/card.html`):
```html
{{define "card"}}
<div class="card">
  <h2>{{slot "title"}}Untitled{{end}}</h2>
  <p>{{slot "body"}}{{end}}</p>
</div>
{{end}}
```

**Use a component** (any page or partial):
```html
{{component "card"}}
  {{fill "title"}}Hello{{end}}
  {{fill "body"}}World{{end}}
{{end}}
```

**Layout** (`templates/layout.html`) uses slots for `title`, `nav`, `content`, `head`, `scripts`.

**Pages** wrap themselves in the layout:
```html
{{component "layout"}}
  {{fill "title"}}My Page{{end}}
  {{fill "content"}}
    <h1>Hello</h1>
  {{end}}
{{end}}
```

`web.Render(w, "page.html", data)` executes a page by its filename.

---

## Global flags

Available on all commands:

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Overwrite existing files |
| `--dry-run` | `-p` | Preview without writing |
| `--skip` | `-s` | Skip existing files silently |
