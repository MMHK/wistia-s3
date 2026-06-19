# AGENTS.md — wistia-s3

Go 1.21 CLI + HTTP service that migrates Wistia videos to S3 (with optional CloudFront). Uses `gorilla/mux`, BoltDB, `aws-sdk-go` v1, vendor mode. Frontend in `web/` is rspack (use **yarn**, not npm).

## Commands

Local dev uses Docker with `golang:1.21` image (no local Go install needed). Frontend uses yarn directly.

```bash
# Build (output binary to ./wistia-s3)
docker run --rm -v "${PWD}:/app" -w /app golang:1.21 go build -o wistia-s3

# Test — ALL tests are integration tests (hit real Wistia API + S3)
docker run --rm -v "${PWD}:/app" -w /app --env-file .env golang:1.21 go test ./pkg/...
docker run --rm -v "${PWD}:/app" -w /app --env-file .env golang:1.21 go test ./pkg/ -v -run TestFuncName

# After any dependency change, vendor/ MUST be regenerated
docker run --rm -v "${PWD}:/app" -w /app golang:1.21 sh -c "go get github.com/pkg@version && go mod vendor"

# Frontend (run from web/ directory)
yarn build                           # production → web/dist/
yarn serve                           # dev server

# E2E tests (multi-service workflows: Wistia + S3 + DashScope + BoltDB)
docker compose -f docker-compose.e2e.yml up --build
# Equivalent docker run:
docker run --rm -v "${PWD}:/app" -w /app --env-file .env golang:1.21 go test ./pkg/ -v -run TestE2E -timeout 600s
```

**Verification order**: `docker build` → `docker test ./pkg/...` (needs `.env` with creds) → `yarn build` (from `web/`) → `docker compose -f docker-compose.e2e.yml up --build` (e2e)

## Testing gotchas

- **No mocks.** Every test in `pkg/*_test.go` calls real Wistia API and/or real S3. `go test ./pkg/...` will fail without valid `.env` credentials.
- `conf_test.go` requires a `config.json` file at repo root — it does not exist by default, must be created manually.
- `wistia_test.go` has a test (`TestWistiaHelper_reUploadAllDemoPage`) that reads `ALL_VIDEO_JSON` env var pointing to a JSON file.
- `tests/testing.go` auto-loads `../.env` via `godotenv` in its `init()`.
- **E2E tests** (`TestE2E_*`) are multi-service workflow tests, distinct from per-feature integration tests. Run via `docker compose -f docker-compose.e2e.yml up --build` (10-min timeout). Requires `.env` with Wistia + S3 + DashScope credentials.
  - `TestE2E_FindNotUploadedVideos` — compares Wistia library vs BoltDB to find un-migrated videos.
  - `TestE2E_IndexVideoToS3_DashScope` — full AI indexing pipeline (transcribe → analyze → S3 upload → BoltDB round-trip). ~51s runtime. Skips if `DASHSCOPE_API_KEY` is missing.
  - The e2e compose file uses named volumes (`go-cache`, `go-mod`) to persist build/module cache across runs.

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
- Package-level `Log` global (log/slog), level from `LOG_LEVEL` env

## Frontend specifics

- Two rspack entry points: `src/main.js` → `wistia-s3.min.js`, `src/demo.js` → `demo.min.js`
- Production build **inlines** all CSS and JS into HTML files (via custom `InlineJSPlugin` in `rspack.config.js`) — `demo.html` gets `demo.min.js` inlined, `index.html` gets `wistia-s3.min.js` inlined
- Dev mode uses `DevTemplatePlugin` to replace `{{.VideoName}}`, `{{.HashId}}`, `{{.WistiaS3JSUrl}}` from `web/.env`; production build preserves these for Go template replacement
- `experiments.css = true` bundles CSS into JS; CSS is also inlined into HTML as `<style>` tags
- Single bundle output: `splitChunks: false`, `runtimeChunk: false`, `performance.hints: false` — no chunk splitting
- `web/dist/` files are used by Go as Go templates. `wistia-s3.min.js` has `{{.MediaEndPoint}}` and `{{.TrackingID}}` injected at runtime by `WistiaHelper.BuildTemplate()`
- Rspack reads `web/.env` for dev variables: `VIDEO_NAME`, `HASH_ID`, `WISTIA_S3_JS_URL`
- `yarn serve` prompts for FRP tunnel (public access via `mmhk-frp`). FRP env vars: `FRP_ENDPOINT`, `FRP_ENDPOINT_PORT`, `FRP_API_PORT`, `FRP_API_USER`, `FRP_API_PWD`, `FRP_PUBLIC_DOMAIN`

## Config

`conf.json` (via `-c` flag) → env vars fill empty fields via `MarginWithENV()` methods. Docker uses env vars only (no config file).

Env vars: `S3_KEY`, `S3_SECRET`, `S3_BUCKET`, `S3_REGION`, `S3_PREFIX`, `S3_CLOUDFRONT_DOMAIN`, `S3_CLOUDFRONT_DIST_ID`, `WISTIA_API_KEY`, `WISTIA_WORKER_LIMIT`, `TEMPLATE_DIR_PATH`, `LOG_LEVEL`, `LISTEN`, `DB_FILE_PATH`, `WEBROOT`, `GA_TRACKING_ID`

## Gotchas

- BoltDB file path in Docker defaults to `/app/wista-s3.db` — the typo "wista" is intentional (backward compat with existing deployments)
- `S3_PREFIX` defaults to `"email2db"` in code (`storage_s3.go:25`), not `"wistia-backup"` as `.env.example` suggests — legacy default from when the project was named email2db
- Receiver name is `this` throughout the codebase, not `s` / `r` / etc.
- JSON error format: `{"status": false, "error": "..."}` (`APIStandardError`). Success: `{"status": true, "data": ...}` (`APIResponse`)
- All S3 uploads use `PublicRead: true`
- Go 1.21 is pinned — do not upgrade without testing all vendored dependencies
- Docker compose service is named `email2db` (legacy name, not renamed for backward compat)
- `SaveVideoInfo` in `MoveVideoToS3` runs in a **detached goroutine** (`http.go:300`) — task status becomes `FINISHED` before BoltDB write completes. Polling `/tasks/{id}` immediately may find the video missing from BoltDB.
- `MoveVideoToS3` **skips migration** for videos already in BoltDB (unless `?forceRefresh=true`) — it returns pre-computed S3/CloudFront URLs without verifying assets exist on S3
- `RefreshVideoInfo` fetches `index.json` directly from S3 (`s3.{region}.amazonaws.com`), bypassing CloudFront even when configured
- `UploadWistiaS3JS` (player JS) is a standalone function — it is **not** called automatically by `/move`. The player JS must be uploaded separately before video pages work

## Workflow & Process (Mandatory)

### Role Separation

- **Main Agent** (this session): Orchestrator only. **NEVER writes code directly.**
  - Creates task plans in `docs/plans/` before any implementation
  - Delegates all code changes to **Sub Agents** via the `task` tool
  - Maintains long-running task loops until the project goal is complete
  - Tracks progress, resolves blockers, and coordinates sub agent outputs
  - Continuously asks clarifying questions until understanding is aligned

- **Sub Agents**: Executors. Implement code, run tests, perform reviews.
  - `explore` agent: Codebase analysis, file search, architecture questions
  - `general` agent: Code implementation, test writing, file modifications, multi-step tasks
  - `code-reviewer` agent: Code review, concurrency safety, S3 backward compatibility checks

### Workflow (Every Task MUST Follow This Order)

```
1. CLARIFY → Main agent asks user questions to align understanding
2. PLAN    → Main agent creates/updates plan in docs/plans/<task-name>-plan.md
3. DELEGATE → Main agent spawns sub agent(s) to implement
4. REVIEW  → Main agent spawns sub agent to review code against plan
5. UPDATE  → Sub agent updates plan document with task status
6. VERIFY  → Main agent runs: docker build → docker test → yarn build
7. LOOP    → Main agent checks progress, spawns next task or loops until done
```

### Clarify Before Plan

The main agent **must** continuously ask the user to clarify planning and implementation details before any work begins. Do not assume intent — confirm:

1. **Goal**: What is the desired outcome? Acceptance criteria?
2. **Scope**: Which routes / S3 paths / BoltDB operations are affected? Does this change the migration flow, API, or frontend templates?
3. **Constraints**: Edge cases, backward compatibility with existing S3 data or BoltDB entries, API contract changes?
4. **Priority**: Which parts must be done first? Any blockers?

Keep asking until understanding is fully aligned with the user's intent. Only then proceed to planning.

### Planning-First Rule — Doc BEFORE Implementation

Before **any** code is written, the main agent **must** create or update a planning document at `docs/plans/{feature-name}-plan.md`. No implementation starts without this doc being finalized.

### Plan Document Format (`docs/plans/<task-name>-plan.md`)

Every plan document MUST use this structure:

```markdown
# <Task Title>

**Created**: YYYY-MM-DD
**Status**: 🟡 In Progress | 🟢 Complete | 🔴 Blocked | ⚪ Pending
**Priority**: High | Medium | Low

---

## Background & Goals
Brief description of why this task exists and what it aims to achieve.

## Technical Approach
Architecture decisions, trade-offs (especially S3 key layout changes, BoltDB schema changes, or async task model changes).

## Affected Files / Routes / S3 Paths
Explicit list of what will be modified.

## Tasks

| # | Task | Status | Sub Agent | Notes |
|---|------|--------|-----------|-------|
| 1 | Task description | ⬜ Pending | — | |
| 2 | Task description | 🔄 In Progress | general | |
| 3 | Task description | ✅ Complete | general | Reviewed |

### Status Legend
- ⬜ Pending — not started
- 🔄 In Progress — sub agent working on it
- ✅ Complete — implemented and reviewed
- 🔴 Blocked — dependency or issue, needs resolution
- ⏭️ Skipped — not needed, with reason

## Testing Strategy
What to test and how (note: all tests are integration tests requiring real credentials).

## Review Log

| Date | Reviewer | Findings | Action Taken |
|------|----------|----------|--------------|
| YYYY-MM-DD | code-reviewer | Description | Fix applied |

## Open Questions
Anything still unresolved.
```

### Sub Agent Delegation Rules

- **Always include full context** in the prompt: file paths, expected behavior, constraints from AGENTS.md.
- **One sub agent per logical task** — do not batch unrelated changes.
- **Parallel when independent** — launch multiple sub agents simultaneously for unrelated tasks.
- **Sequential when dependent** — wait for prior sub agent to finish before spawning the next.
- Use `explore` agent before `general` agent when the task requires understanding unfamiliar code.
- Use `code-reviewer` agent after every implementation task.

### Implementation Sub Agent Prompt Template

When spawning an implementation sub agent, include:

- **What**: Concise description of the task
- **Context**: Reference to `docs/plans/{feature}-plan.md` for full plan
- **Files**: Specific paths to create/modify
- **Constraints**: Follow existing conventions (see AGENTS.md), Go 1.21 compat, S3 key layout backward compat, receiver name is `this`
- **Verification**: How to verify (e.g. `docker run --rm -v "${PWD}:/app" -w /app --env-file .env golang:1.21 go test ./pkg/ -run TestXxx`)
- **Return**: Summarize changes made and any issues encountered

### Review Sub Agent Prompt Template

When spawning a review sub agent, include:

- **Scope**: List of changed files or `git diff`
- **Checklist**: 
  - Correctness and logic
  - Code style (receiver name `this`, JSON format `{"status": bool, ...}`)
  - Concurrency safety (semaphore channels, WaitGroup, mutex)
  - Error handling (`APIStandardError` format)
  - S3 backward compatibility (key layout, PublicRead)
  - BoltDB compatibility (bucket: "media")
  - Test coverage
- **Return**: List of issues found (or "LGTM"), severity, and suggested fixes

### Main Agent Loop Responsibilities

For multi-task or long-running projects, the main agent MUST:
1. Break the project into discrete tasks in the plan document
2. Execute tasks one-by-one or in parallel batches via sub agents
3. After each batch: verify results, update plan status
4. If a sub agent fails or produces incorrect output: diagnose, adjust prompt, re-delegate
5. Continue the loop until ALL tasks in the plan are ✅ Complete or explicitly ⏭️ Skipped
6. After each major milestone, spawn a `code-reviewer` sub agent for a holistic review

### Forbidden Main Agent Actions

The main agent MUST NOT:
- Write or edit source code files directly (use sub agents)
- Skip the plan document step
- Skip the review step
- Mark tasks as complete without sub agent review confirmation
- Abandon a task loop without updating the plan document status to 🔴 Blocked with a reason

### Docs Directory Convention

- Plan documents: `docs/plans/{feature-name}-plan.md`
- Feature docs: `docs/{feature-name}.md` (for architecture/design docs, not task plans)
- Subdirectories for grouped features: `docs/migration/*.md`, `docs/api/*.md`
- Each plan doc must include a **Tasks** table with status tracking
- Keep docs updated as tasks progress
