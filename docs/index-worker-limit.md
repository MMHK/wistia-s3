# Transcribe & IndexVideo Worker Limit

## Goal / Background

`POST /index/{hash}` (single video indexing) spawns a goroutine to run `indexVideoToS3` without any concurrency control. Multiple single-video requests can run unlimited Transcribe + IndexVideo DashScope API calls in parallel. The batch endpoint `POST /index` already uses `s.uploadQueue` (sized by `WISTIA_WORKER_LIMIT`), so the fix is to apply the same semaphore to the single endpoint.

## Technical Approach

Reuse the existing `s.uploadQueue` semaphore channel on `HTTPService`. Wrap the goroutine in `IndexVideo` handler with the same acquire/defer-release pattern used in `IndexAllVideo` and `MoveVideoToS3`.

## Affected Files / Routes

| File | Route | Change |
|------|-------|--------|
| `pkg/http.go:458` | `POST /index/{hash}` | Wrap `go s.indexVideoToS3(...)` with semaphore |

## Tasks

- [x] Add semaphore acquire/release around `indexVideoToS3` call in `IndexVideo` handler (`http.go:458`)

## Testing Strategy

- `docker run --rm -v "${PWD}:/app" -w /app golang:1.19 go build -o wistia-s3` to verify compilation
- Manual: fire multiple `POST /index/{hash}` requests and observe only `WISTIA_WORKER_LIMIT` run concurrently

## Open Questions

None.
