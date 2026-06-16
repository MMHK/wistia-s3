# CloudFront Cache Invalidation (Flush)

## Goal

When CloudFront is configured (`S3_CLOUDFRONT_DOMAIN` set), automatically invalidate the corresponding CloudFront cache paths after S3 uploads complete. This ensures viewers get the latest content without waiting for CloudFront's default TTL expiry.

## Background

The project currently mirrors uploads to both S3 and CloudFront prefix paths, but never invalidates CloudFront cache. When a video is re-migrated (`forceRefresh=true`) or updated via `/index`, stale content may be served from CloudFront until TTL expires.

## Technical Approach

### New env var: `S3_CLOUDFRONT_DIST_ID`

CloudFront `CreateInvalidation` API requires the Distribution ID (e.g., `E1234ABCDE`), which cannot be derived from the domain name. A new env var is mandatory.

If `S3_CLOUDFRONT_DOMAIN` is set but `S3_CLOUDFRONT_DIST_ID` is missing, flush is skipped with a warning log (uploads still succeed).

### Architecture

- New file `pkg/cloudfront.go` — `CloudFrontHelper` wrapping `cloudfront.CloudFront` client
- `CloudFrontDistID` field added to `S3Config`
- Flush is called inline after uploads complete in each relevant function
- Flush failure → warning log only, no error propagation

### Invalidation paths per operation

| Function | Invalidated paths (per request) |
|----------|-------------------------------|
| `MoveToS3` | `/{prefix}/cloudfront/media/{hashId}/*` |
| `UploadWistiaS3JS` | `/{prefix}/cloudfront/media/wistia-s3.min.js` |
| `indexVideoToS3` | `/{prefix}/cloudfront/media/{hashId}/index-ai.json`, `/{prefix}/cloudfront/media/{hashId}/subtitles.vtt` |

Using wildcard `/*` for `MoveToS3` covers all files under that hashId (index.json, cover, videos, original, html pages) in a single invalidation path — very efficient against the 1000 paths/month free quota.

### IAM Minimum Policy

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "S3Upload",
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:PutObjectAcl",
        "s3:GetObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::YOUR_BUCKET",
        "arn:aws:s3:::YOUR_BUCKET/YOUR_PREFIX/*"
      ]
    },
    {
      "Sid": "CloudFrontInvalidation",
      "Effect": "Allow",
      "Action": [
        "cloudfront:CreateInvalidation",
        "cloudfront:GetInvalidation"
      ],
      "Resource": "arn:aws:cloudfront::ACCOUNT_ID:distribution/YOUR_DIST_ID"
    }
  ]
}
```

## Tasks

- [x] 1. Add `CloudFrontDistID` field to `S3Config` struct (`pkg/storage.go`)
- [x] 2. Load `S3_CLOUDFRONT_DIST_ID` env in `LoadS3ConfigWithEnv()` (`pkg/storage_s3.go`)
- [x] 3. Add `S3_CLOUDFRONT_DIST_ID` to `.env.example`
- [x] 4. Run `go mod vendor` to vendor `cloudfront` package from `aws-sdk-go`
- [x] 5. Create `pkg/cloudfront.go` with `CloudFrontHelper`
  - [x] `NewCloudFrontHelper(conf *S3Config) *CloudFrontHelper` — returns nil if DistID is empty
  - [x] `InvalidatePaths(paths []string) error` — calls `CreateInvalidation` with caller reference = timestamp
- [x] 6. Integrate flush into `MoveToS3()` in `pkg/wistia.go` — after all cloudfront uploads complete
- [x] 7. Integrate flush into `UploadWistiaS3JS()` in `pkg/wistia.go` — after cloudfront JS upload
- [x] 8. Integrate flush into `indexVideoToS3()` in `pkg/http.go` — after cloudfront index-ai.json + subtitles.vtt uploads
- [x] 9. Build verification: `docker build`
- [x] 10. Update AGENTS.md with new env var documentation

## Affected files

| File | Change |
|------|--------|
| `pkg/storage.go` | Add `CloudFrontDistID` to `S3Config` |
| `pkg/storage_s3.go` | Load `S3_CLOUDFRONT_DIST_ID` env |
| `pkg/cloudfront.go` | **New file** — CloudFrontHelper |
| `pkg/wistia.go` | Add flush calls in `MoveToS3`, `UploadWistiaS3JS` |
| `pkg/http.go` | Add flush call in `indexVideoToS3` |
| `.env.example` | Add `S3_CLOUDFRONT_DIST_ID` |
| `go.mod` / `vendor/` | Vendor cloudfront package |

## Testing strategy

All tests are integration tests requiring real credentials. Manual verification:
1. Set `S3_CLOUDFRONT_DIST_ID` in `.env`
2. Run `POST /move/{hash}` with a test video
3. Check CloudFront console for new invalidation
4. Verify the invalidated paths return fresh content

## Open questions

None — all design decisions confirmed.
