# E2E Test: Find Wistia Videos Not Yet Uploaded to S3

**Created**: 2026-06-19
**Status**: 🟢 Complete
**Priority**: Medium

---

## Background & Goals
Create an e2e integration test that compares Wistia API video list against local BoltDB "media" bucket data using hashId matching, and outputs which Wistia videos have NOT been uploaded to S3 yet.

## Technical Approach
1. Fetch all videos from Wistia API via `ListAllVideos()`
2. Get all migrated videos from BoltDB "media" bucket via `GetAllVideoInfo()`
3. Build a `map[string]bool` set of migrated hashIds from BoltDB
4. Iterate Wistia videos, find those whose hashId is NOT in the migrated set
5. Log each un-uploaded video's info (hashId, name, status, archived, duration, created)
6. Log summary: total Wistia count, migrated count, not-uploaded count

## Affected Files
- `pkg/e2e_test.go` — new test file

## Tasks

| # | Task | Status | Sub Agent | Notes |
|---|------|--------|-----------|-------|
| 1 | Create `TestE2E_FindNotUploadedVideos` in `pkg/e2e_test.go` | ✅ Complete | general | go vet passed |
| 2 | Review test code | ✅ Complete | explore | LGTM, minor fixes applied |

### Status Legend
- ⬜ Pending — not started
- 🔄 In Progress — sub agent working on it
- ✅ Complete — implemented and reviewed
- 🔴 Blocked — dependency or issue, needs resolution
- ⏭️ Skipped — not needed, with reason

## Testing Strategy
- Run via existing e2e compose (matches `-run TestE2E` pattern):
  ```bash
  docker compose -f docker-compose.e2e.yml up --build
  ```
- Requires valid `.env` with `WISTIA_API_KEY` and `DB_FILE_PATH` pointing to a BoltDB with existing media entries

## Open Questions
None
