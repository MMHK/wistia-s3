# Subtitle Refinement via Video Model

## Goal

ASR (qwen3-asr-flash-filetrans) produces subtitles with occasional errors (homophones, wrong characters). Since `IndexVideo` already sends the video + ASR subtitles to `qwen3.5-omni-plus` (multimodal with native audio+vision), we can ask it to refine the subtitle text in the same API call — no extra round-trip needed.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Subtitle strategy | Replace ASR text, keep ASR timestamps (merge-by-index) | Hard guarantee on timing correctness. Model fixes text only. |
| MaxTokens | 32768 | Subtitles can be long; 32K handles most videos. |
| `response_format` | `{"type": "json_object"}` | Forces valid JSON output, no markdown fences. |
| `enable_thinking` | `true` | Model reasons about audio+visual context for better corrections. ~30-50% token cost increase. |
| Fallback | If model returns 0 subtitles or count mismatch, use ASR subtitles | Graceful degradation. |
| Parse failure fallback | Fall back to ASR subtitles | Never fail the whole index job over subtitle refinement. |

## Technical Approach

### Prompt extension

`buildVideoPrompt` will instruct the model to output a `subtitles` array alongside `summary` and `chapters`. Each entry must:
- Preserve the original `start`/`end` timestamps verbatim
- Fix only the `text` field based on audio+visual context
- Maintain the same count and order as the input transcript

### Merge-by-index logic (in `indexVideoToS3`)

```
if len(videoResult.Subtitles) == len(audioResult.Subtitles) && len > 0:
    for i in range(len):
        finalSubtitles[i].Text = videoResult.Subtitles[i].Text
        # start/end preserved from ASR
elif len(videoResult.Subtitles) > 0:
    log warning, fall back to ASR
else:
    fall back to ASR
```

### API request changes

- `MaxTokens`: 4096 → 32768
- `ResponseFormat`: `{"type": "json_object"}`
- `EnableThinking`: `true`
- Streaming parser unchanged (thinking goes to `reasoning_content`, ignored by existing parser)

## Affected Files

| File | Change |
|------|--------|
| `pkg/dashscope.go` | Add `ResponseFormat`/`EnableThinking` to request struct; add `Subtitles` to `DashScopeVideoAnalysis`; extend `buildVideoPrompt`; raise `MaxTokens`; add `ReasoningContent` to stream chunk delta (explicitly ignored) |
| `pkg/http.go` | In `indexVideoToS3`: merge-by-index logic after parsing `videoResult` |
| `pkg/dashscope_test.go` | Update JSON fixture tests; verify Subtitles round-trip |

## Tasks

- [ ] 1. Add `ResponseFormat` and `EnableThinking` fields to `dashscopeChatRequest`
- [ ] 2. Add `dashscopeResponseFmt` struct
- [ ] 3. Add `ReasoningContent` field to `dashscopeStreamChunk.Delta` (explicitly ignored)
- [ ] 4. Add `Subtitles []DashScopeSubtitleEntry` to `DashScopeVideoAnalysis` (omitempty)
- [ ] 5. Extend `buildVideoPrompt` to instruct model to output corrected subtitles
- [ ] 6. Update `IndexVideo` to set `ResponseFormat`, `EnableThinking`, `MaxTokens=32768`
- [ ] 7. In `indexVideoToS3`: implement merge-by-index logic with fallback
- [ ] 8. Update `dashscope_test.go` JSON fixtures
- [ ] 9. Verify: `docker build`

## Open Questions

None — all resolved during planning.
