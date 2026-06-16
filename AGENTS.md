# AGENTS.md — wistia-s3

Go 1.19 CLI + HTTP service that migrates Wistia videos to S3 (with optional CloudFront). Uses `gorilla/mux`, BoltDB, `aws-sdk-go` v1, vendor mode. Frontend in `web/` is webpack 5 (use **yarn**, not npm).

## Commands

Local dev uses Docker with `golang:1.19` image (no local Go install needed). Frontend uses yarn directly.

```bash
# Build (output binary to ./wistia-s3)
docker run --rm -v "${PWD}:/app" -w /app golang:1.19 go build -o wistia-s3

# Test — ALL tests are integration tests (hit real Wistia API + S3)
docker run --rm -v "${PWD}:/app" -w /app --env-file .env golang:1.19 go test ./pkg/...
docker run --rm -v "${PWD}:/app" -w /app --env-file .env golang:1.19 go test ./pkg/ -v -run TestFuncName

# After any dependency change, vendor/ MUST be regenerated
docker run --rm -v "${PWD}:/app" -w /app golang:1.19 sh -c "go get github.com/pkg@version && go mod vendor"

# Frontend (run from web/ directory)
yarn build                           # production → web/dist/
yarn serve                           # dev server
```

**Verification order**: `docker build` → `docker test ./pkg/...` (needs `.env` with creds) → `yarn build` (from `web/`)

## Testing gotchas

- **No mocks.** Every test in `pkg/*_test.go` calls real Wistia API and/or real S3. `go test ./pkg/...` will fail without valid `.env` credentials.
- `conf_test.go` requires a `config.json` file at repo root — it does not exist by default, must be created manually.
- `wistia_test.go` has a test (`TestWistiaHelper_reUploadAllDemoPage`) that reads `ALL_VIDEO_JSON` env var pointing to a JSON file.
- `tests/testing.go` auto-loads `../.env` via `godotenv` in its `init()`.

## Architecture

### Migration flow

```
Wistia API → fetch video + assets
  → download assets concurrently (WaitGroup + semaphore channel)
  → upload to S3 (and CloudFront path if configured)
  → write index.json to S3 + BoltDB (bucket: "media")
  → render HTML pages via Go text/template → upload to S3
```

### Async task model

POST `/move/{hash}` or `/move` → returns task ID immediately → goroutine does work → poll `GET /tasks/{id}` for status. Tasks stored in `map[string]*Task` + `sync.Mutex` (in-memory, not persisted). Same pattern for `/refresh/media`.

### API routes

| Method | Path | Notes |
|--------|------|-------|
| GET | `/` | Redirects to `/swagger/index.html` |
| GET | `/media` | `?hash=` for single video, omit for all (from BoltDB) |
| POST | `/move/{hash}` | Single video migration. `?forceRefresh=true` to overwrite |
| POST | `/move` | Batch: `{"media": ["hash1", ...]}` |
| POST | `/refresh/media` | Re-index all S3 `index.json` files into BoltDB |
| GET | `/tasks/{id}` | Poll task status |
| GET | `/swagger/*` | Static Swagger UI from `webroot/swagger/` |

### S3 key layout

```
{prefix}/media/{hashId}/{height}.mp4       # video files by resolution
{prefix}/media/{hashId}/cover.jpg          # thumbnail
{prefix}/media/{hashId}/original.{ext}     # original upload
{prefix}/media/{hashId}/index.json         # video metadata (WistiaRespVideo JSON)
{prefix}/media/{hashId}/index.html         # player page
{prefix}/media/{hashId}/demo.html          # demo page
{prefix}/media/wistia-s3.min.js            # player JS bundle (Go template!)
{prefix}/cloudfront/media/...              # full mirror of above
```

### Concurrency

- `WISTIA_WORKER_LIMIT` env (default 3) → channel-based semaphore in both `HTTPService` and `WistiaHelper`
- Parallel asset downloads per video via `sync.WaitGroup`
- Package-level `Log` global (go-logging), level from `LOG_LEVEL` env

## Frontend specifics

- Two webpack entry points: `src/main.js` → `wistia-s3.min.js`, `src/demo.js` → `demo.min.js`
- Production build **inlines** all CSS and JS into HTML files (via `HtmlInlineScriptPlugin` / `HTMLInlineCSSWebpackPlugin`) — `demo.html` gets `demo.min.js` inlined, `index.html` is standalone
- `web/dist/` files are used by Go as Go templates. `wistia-s3.min.js` has `{{.MediaEndPoint}}` and `{{.TrackingID}}` injected at runtime by `WistiaHelper.BuildTemplate()`
- Webpack reads `web/.env` for dev variables: `VIDEO_NAME`, `HASH_ID`, `WISTIA_S3_JS_URL`

## Config

`conf.json` (via `-c` flag) → env vars fill empty fields via `MarginWithENV()` methods. Docker uses env vars only (no config file).

Env vars: `S3_KEY`, `S3_SECRET`, `S3_BUCKET`, `S3_REGION`, `S3_PREFIX`, `S3_CLOUDFRONT_DOMAIN`, `S3_CLOUDFRONT_DIST_ID`, `WISTIA_API_KEY`, `WISTIA_WORKER_LIMIT`, `TEMPLATE_DIR_PATH`, `LOG_LEVEL`, `LISTEN`, `DB_FILE_PATH`, `WEBROOT`, `GA_TRACKING_ID`

## Gotchas

- BoltDB file path in Docker defaults to `/app/wista-s3.db` — the typo "wista" is intentional (backward compat with existing deployments)
- `S3_PREFIX` defaults to `"email2db"` in code (`storage_s3.go:25`), not `"wistia-backup"` as `.env.example` suggests — legacy default from when the project was named email2db
- Receiver name is `this` throughout the codebase, not `s` / `r` / etc.
- JSON error format: `{"status": false, "error": "..."}` (`APIStandardError`). Success: `{"status": true, "data": ...}` (`APIResponse`)
- All S3 uploads use `PublicRead: true`
- Go 1.19 is pinned — do not upgrade without testing all vendored dependencies
- Docker compose service is named `email2db` (legacy name, not renamed for backward compat)
- `SaveVideoInfo` in `MoveVideoToS3` runs in a **detached goroutine** (`http.go:300`) — task status becomes `FINISHED` before BoltDB write completes. Polling `/tasks/{id}` immediately may find the video missing from BoltDB.
- `MoveVideoToS3` **skips migration** for videos already in BoltDB (unless `?forceRefresh=true`) — it returns pre-computed S3/CloudFront URLs without verifying assets exist on S3
- `RefreshVideoInfo` fetches `index.json` directly from S3 (`s3.{region}.amazonaws.com`), bypassing CloudFront even when configured
- `UploadWistiaS3JS` (player JS) is a standalone function — it is **not** called automatically by `/move`. The player JS must be uploaded separately before video pages work

## Workflow & Process

### Clarify before plan

The main agent **must** continuously ask the user to clarify planning and implementation details before any work begins. Do not assume intent — confirm:

1. **Goal**: What is the desired outcome? Acceptance criteria?
2. **Scope**: Which routes / S3 paths / BoltDB operations are affected? Does this change the migration flow, API, or frontend templates?
3. **Constraints**: Edge cases, backward compatibility with existing S3 data or BoltDB entries, API contract changes?
4. **Priority**: Which parts must be done first? Any blockers?

Keep asking until understanding is fully aligned with the user's intent. Only then proceed to planning.

### Planning-first rule — doc BEFORE implementation

Before **any** code is written, the main agent **must** create or update a planning document at `docs/{feature-name}.md`. No implementation starts without this doc being finalized.

The doc must include:
- **Goal / background** — what problem this solves
- **Technical approach** — architecture decisions, trade-offs (especially S3 key layout changes, BoltDB schema changes, or async task model changes)
- **Task breakdown** — numbered checklist with `- [ ]` / `- [x]`
- **Affected files / routes / S3 paths** — explicit list
- **Testing strategy** — what to test and how (note: all tests are integration tests requiring real credentials)
- **Open questions** — anything still unresolved

### Sub-agent delegation

The main agent **never writes code directly**. All implementation is delegated to sub agents:

| Step | Agent type | Purpose |
|------|-----------|---------|
| Clarify | Main agent | Ask user questions to align understanding |
| Plan | Main agent | Write `docs/{feature}.md` (must be done **before** any code) |
| Implement | `general` or `explore` sub agent | Write the actual code changes |
| Review | `code-reviewer` sub agent | Inspect changes for correctness, style, concurrency safety, S3 backward compat |

#### Implementation flow

```
1. Main agent asks clarifying questions until understanding is aligned
2. Main agent writes planning doc → docs/{feature}.md (with task checklist)
3. Main agent spawns implementation sub agent with:
   - Clear task description referencing the doc
   - File paths to modify
   - Expected outcome
4. Sub agent implements changes, returns summary
5. Main agent spawns code-reviewer sub agent with:
   - git diff or list of changed files
   - Review checklist (correctness, conventions, concurrency safety, S3 key layout compat, error handling)
6. If review finds issues → spawn another implementation sub agent to fix
7. Repeat 4-6 until review passes
8. Main agent runs verification: docker build → docker test ./pkg/... → yarn build (from web/)
```

### Sub agent prompt template

When spawning an implementation sub agent, include:

- **What**: Concise description of the task
- **Context**: Reference to `docs/{feature}.md` for full plan
- **Files**: Specific paths to create/modify
- **Constraints**: Follow existing conventions (see AGENTS.md), Go 1.19 compat, S3 key layout backward compat
- **Verification**: How to verify (e.g. `docker run --rm -v "${PWD}:/app" -w /app --env-file .env golang:1.19 go test ./pkg/ -run TestXxx`)
- **Return**: Summarize changes made and any issues encountered

When spawning a review sub agent, include:

- **Scope**: List of changed files or `git diff`
- **Checklist**: Correctness, code style, concurrency safety, error handling, S3 backward compatibility, test coverage
- **Return**: List of issues found (or "LGTM"), severity, and suggested fixes

### Docs directory convention

- One MD file per feature/feature-area: `docs/{feature-name}.md`
- Subdirectories for grouped features: `docs/migration/*.md`, `docs/api/*.md`
- Each doc must include a **Tasks** section with checkboxes (`- [ ]` / `- [x]`)
- Keep docs updated as tasks progress
