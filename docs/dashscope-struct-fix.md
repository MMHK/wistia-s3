# DashScope Struct Fix

## Goal
Fix `dashscopeFiletransTaskResponse` and all related structs in `pkg/dashscope.go` to match the official Alibaba Cloud Qwen-ASR API documentation, and fix the runtime test failure caused by the MaaS workspace endpoint returning a different response wrapper.

## Status: ✅ COMPLETED

## Problem
1. **Test failure**: `TestE2E_IndexVideoToS3_DashScope` fails with `unknown ASR task status: ` (empty status) when using the MaaS workspace endpoint (`llm-is3ow1huu0u2g59c.ap-southeast-1.maas.aliyuncs.com`).
2. **Root cause**: The MaaS platform wraps API responses in `{"code": "", "message": "", "data": {...}}` envelope instead of the standard `{"request_id": "...", "output": {...}}` format. The poll response's `output.task_status` is nested inside `data`, so `json.Unmarshal` into the current struct leaves `TaskStatus` empty.
3. **Struct incompleteness**: Multiple structs are missing fields documented in the official API reference (request_id, submit_time, enable_itn, emotion, words, file_url, audio_info, etc.)

## Technical Approach

### 1. Add MaaS response wrapper support
- Create a `dashscopeMaaSEnvelope` struct: `{"code": "", "message": "", "data": json.RawMessage}`
- `unwrapMaaSResponse()` helper: tries unmarshalling as MaaS envelope first. If `code` is non-empty → error; if `data` is present → return `data` bytes for standard parsing; otherwise returns original body
- Handles both standard DashScope API (`dashscope-intl.aliyuncs.com`) and MaaS workspace endpoints

### 2. Fix all structs to match official docs
- `dashscopeFiletransSubmitResponse`: add `RequestId`
- `dashscopeFiletransParameters`: add `EnableItn`, `Language`
- `dashscopeFiletransTaskResponse`: add `RequestId`, `SubmitTime`, `ScheduledTime`, `EndTime`, `TaskMetrics`
- `dashscopeFiletransResult`: add `FileUrl`, `AudioInfo`
- `dashscopeFiletransSentence`: add `Emotion`, `Words` (word-level timestamps)
- New struct `dashscopeFiletransWord` (word-level result)

### 3. Add error handling improvements
- Check HTTP status code on poll response (not just submit)
- Log response body when unexpected task status is encountered
- Remove `X-DashScope-Async` header from GET poll request (not required per docs)

## Task Breakdown
- [x] Add `dashscopeMaaSEnvelope` wrapper struct + `unwrapMaaSResponse` helper
- [x] Update `dashscopeFiletransSubmitResponse` with `RequestId`
- [x] Update `dashscopeFiletransParameters` with `EnableItn`, `Language`
- [x] Update `dashscopeFiletransTaskResponse` with all missing fields
- [x] Update `dashscopeFiletransResult` with `FileUrl`, `AudioInfo`
- [x] Update `dashscopeFiletransSentence` with `Emotion`, `Words`
- [x] Add `dashscopeFiletransWord` struct
- [x] Add MaaS envelope unwrapping in submit parsing
- [x] Add MaaS envelope unwrapping in poll parsing
- [x] Add HTTP status code check in poll loop
- [x] Add response body logging on unexpected status
- [x] Remove `X-DashScope-Async` from poll GET request
- [x] Build succeeds (Go 1.19)
- [x] `TestE2E_IndexVideoToS3_DashScope` passes

## Affected files
- `pkg/dashscope.go` — main implementation file (structs + Transcribe method)

## Test Results (2026-06-15)

### Unit tests
- `TestDashScopeFormatVTTTime` — PASS
- `TestDashScopeIndexResult_ToVTT` — PASS
- `TestDashScopeIndexResult_ToVTT_Empty` — PASS
- `TestDashScopeIndexResult_JSON` — PASS
- `TestDashScopeConf_MarginWithENV` — FAIL (pre-existing: .env overrides defaults)

### E2E test
- `TestE2E_IndexVideoToS3_DashScope` — PASS (50.91s)
- Submit → Poll (RUNNING → SUCCEEDED) → Download → Parse: all work
- 9 subtitles extracted, 7 chapters generated, uploaded to S3, BoltDB round-trip verified

## Key Findings

### MaaS vs Standard DashScope Response Format
| Endpoint | Format | Wrapper |
|----------|--------|---------|
| `dashscope-intl.aliyuncs.com` | Standard DashScope | `{"request_id": "...", "output": {...}}` |
| `{workspaceId}.maas.aliyuncs.com` | MaaS Envelope | `{"code": "", "message": "", "data": {"request_id": "...", "output": {...}}}` |

The MaaS endpoint wraps the **entire standard response** inside the `data` field. The `unwrapMaaSResponse()` function handles both transparently.

### Poll GET Request Headers
- Only `Authorization: Bearer {key}` is needed
- `X-DashScope-Async: enable` is **only** for the POST submit, not the GET poll
