# Logger Upgrade: go-logging → log/slog

**Created**: 2026-06-18
**Status**: 🟢 Complete
**Priority**: High

---

## Background & Goals

The project currently uses `github.com/op/go-logging` (last updated 2016) for logging. This library:
- Is unmaintained and outdated
- Does not support structured logging
- Produces unstructured text output (not suitable for log aggregation)
- Requires Go 1.19 (preventing use of modern Go features)

**Goals**:
1. Upgrade Go from 1.19 to 1.21+ to enable `log/slog` (standard library structured logging)
2. Replace `go-logging` with `log/slog` for structured JSON logging
3. Add contextual information to error logs (currently 58% of logs are bare `Log.Error(err)`)
4. Maintain backward compatibility with existing `LOG_LEVEL` environment variable
5. Preserve the global `Log` variable pattern for minimal API disruption

## Technical Approach

### Go Version Upgrade
- Update `go.mod` from `go 1.19` to `go 1.21`
- Update `Dockerfile` to use `golang:1.21` image
- Update `docker-compose.yml` to use `golang:1.21` image
- Update `AGENTS.md` to reflect new Go version
- Regenerate `vendor/` directory with new Go version

### Logger Migration Strategy
- Create new `pkg/log.go` using `log/slog` with JSON handler
- Maintain global `Log` variable (now `*slog.Logger`) for backward compatibility
- Support `LOG_LEVEL` env var (map to slog levels: DEBUG, INFO, WARN, ERROR)
- Output JSON to stderr (same destination as before)
- Remove `github.com/op/go-logging` from `go.mod` and `vendor/`

### Log Call Site Migration (125+ calls across 14 files)

**Pattern mapping**:
| Old (go-logging) | New (log/slog) | Notes |
|------------------|----------------|-------|
| `Log.Error(err)` | `Log.Error("operation failed", "error", err)` | Add context message |
| `Log.Errorf("msg %v", x)` | `Log.Error("msg", "key", x)` | Structured key-value |
| `Log.Info("msg")` | `Log.Info("msg")` | Direct mapping |
| `Log.Infof("msg %v", x)` | `Log.Info("msg", "key", x)` | Structured key-value |
| `Log.Debug("msg")` | `Log.Debug("msg")` | Direct mapping |
| `Log.Debugf("msg %v", x)` | `Log.Debug("msg", "key", x)` | Structured key-value |
| `Log.Warningf("msg %v", x)` | `Log.Warn("msg", "key", x)` | Note: Warning → Warn |
| `Log.Fatal(err)` | `Log.Error("fatal", "error", err); os.Exit(1)` | Fatal doesn't exist in slog |

**Context enhancement for errors**:
- Analyze each `Log.Error(err)` call site
- Add meaningful context: operation name, resource identifiers, relevant parameters
- Example: `Log.Error(err)` → `Log.Error("S3 upload failed", "error", err, "bucket", bucketName, "key", s3Key)`

### Files to Modify

**Core infrastructure**:
- `go.mod` - Update Go version, remove go-logging dependency
- `go.sum` - Regenerate
- `vendor/` - Regenerate entire vendor directory
- `pkg/log.go` - Complete rewrite for slog
- `main.go` - Update `pkg.Log` usage (3 calls)
- `tests/testing.go` - Update to use slog instead of go-logging

**Application code** (12 files with ~125 log calls):
- `pkg/http.go` - 3 calls (including 1 Fatal)
- `pkg/conf.go` - 5 calls
- `pkg/storage.go` - 1 call
- `pkg/storage_s3.go` - 3 calls
- `pkg/cloudfront.go` - 2 calls
- `pkg/db.go` - 20 calls
- `pkg/wistia.go` - 52 calls (largest)
- `pkg/handler_media.go` - 11 calls
- `pkg/handler_move.go` - 1 call
- `pkg/handler_index.go` - 17 calls
- `pkg/dashscope.go` - 6 calls

**Documentation**:
- `AGENTS.md` - Update Go version references
- `.env.example` - Keep `LOG_LEVEL` as-is (still supported)
- `docker-compose.yml` - Update Go image version

## Tasks

| # | Task | Status | Sub Agent | Notes |
|---|------|--------|-----------|-------|
| 1 | Upgrade Go version in go.mod, Dockerfile, docker-compose.yml | ✅ Complete | general | Changed from 1.19 to 1.21 |
| 2 | Rewrite pkg/log.go to use log/slog with JSON output | ✅ Complete | general | Maintain global Log variable, support LOG_LEVEL env |
| 3 | Update main.go log calls (3 calls) | ✅ Complete | general | Simple migration |
| 4 | Update tests/testing.go logger | ✅ Complete | general | Remove duplicate logger setup |
| 5 | Migrate pkg/db.go log calls (20 calls) | ✅ Complete | general | Add context to error logs |
| 6 | Migrate pkg/wistia.go log calls (52 calls) | ✅ Complete | general | Largest file, add context |
| 7 | Migrate pkg/handler_index.go log calls (17 calls) | ✅ Complete | general | Add context |
| 8 | Migrate pkg/handler_media.go log calls (11 calls) | ✅ Complete | general | Add context |
| 9 | Migrate remaining pkg files (http, conf, storage, storage_s3, cloudfront, handler_move, dashscope) | ✅ Complete | general | 21 calls total |
| 10 | Regenerate vendor/ directory | ✅ Complete | general | Remove go-logging, ensure slog is available |
| 11 | Update AGENTS.md documentation | ✅ Complete | general | Update Go version references |
| 12 | Code review | ✅ Complete | general | LGTM - 1 minor fix applied |
| 13 | Build verification | ✅ Complete | general | docker build succeeded |
| 14 | Test verification | ⬜ Pending | general | Requires .env with credentials |

### Status Legend
- ⬜ Pending — not started
- 🔄 In Progress — sub agent working on it
- ✅ Complete — implemented and reviewed
- 🔴 Blocked — dependency or issue, needs resolution
- ⏭️ Skipped — not needed, with reason

## Testing Strategy

**Note**: All tests are integration tests requiring real Wistia API and S3 credentials.

1. **Build test**: `docker run --rm -v "${PWD}:/app" -w /app golang:1.21 go build -o wistia-s3`
   - Verifies compilation succeeds with new Go version and slog
   - Catches any syntax errors or missing imports

2. **Integration tests**: `docker run --rm -v "${PWD}:/app" -w /app --env-file .env golang:1.21 go test ./pkg/...`
   - Requires valid `.env` with WISTIA_API_KEY, S3 credentials
   - Tests actual functionality (not just logging)
   - Verify logs are output as JSON to stderr

3. **Manual verification**:
   - Run the service locally
   - Trigger various operations (migration, media fetch)
   - Verify JSON log output format
   - Verify LOG_LEVEL env var controls log verbosity
   - Verify error logs include contextual information

## Review Log

| Date | Reviewer | Findings | Action Taken |
|------|----------|----------|--------------|
| 2026-06-18 | general | LGTM - 1 minor issue: AGENTS.md line 79 still mentioned go-logging | Fixed: Changed "(go-logging)" to "(log/slog)" |

## Open Questions

None at this time. All key decisions confirmed with user:
- ✅ Upgrade to Go 1.21+
- ✅ Use JSON structured output
- ✅ Add context to error logs
