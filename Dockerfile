FROM golang:1.24-alpine as builder

# Add Maintainer Info
LABEL maintainer="Sam Zhou <sam@mixmedia.com>"

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN go version \
 && go mod vendor \
 && apk add --no-cache git ca-certificates \
 && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o wistia-s3 \
 && chmod +x wistia-s3

######## Start a new stage from scratch #######
FROM alpine:latest  

RUN apk update \
 && apk add --update libintl \
 && apk add --no-cache tzdata dumb-init mailcap \
 && addgroup -S appgroup \
 && adduser -S appuser -G appgroup -h /home/appuser -s /sbin/nologin

WORKDIR /app

# Copy the Pre-built binary file from the previous stage
COPY --from=builder --chown=appuser:appgroup /app/wistia-s3 .
COPY --chown=appuser:appgroup ./web /app/web
COPY --chown=appuser:appgroup ./webroot /app/webroot

RUN chown -R appuser:appgroup /app

USER appuser

 ENV LISTEN="0.0.0.0:8843" \
 WISTIA_API_KEY="" \
 WISTIA_WORKER_LIMIT=3 \
 TEMPLATE_DIR_PATH=/app/web/dist \
 S3_KEY="" \
 S3_SECRET="" \
 S3_SECRET="" \
 S3_REGION="ap-southeast-1" \
 S3_PREFIX="wistia-backup" \
 S3_BUCKET="s3.test.mixmedia.com" \
 S3_CLOUDFRONT_DOMAIN="" \
 DASHSCOPE_API_KEY="" \
 DASHSCOPE_BASE_URL="https://dashscope-intl.aliyuncs.com" \
 DASHSCOPE_ASR_MODEL="qwen3-asr-flash" \
 DASHSCOPE_VIDEO_MODEL="qwen3.5-omni-plus" \
 TZ="Asia/Hong_Kong" \
 LOG_LEVEL=INFO \
 DB_FILE_PATH=/app/wista-s3.db \
 WEBROOT=/app/webroot

EXPOSE 3031

ENTRYPOINT ["dumb-init", "--"]

CMD echo "{}" > /app/temp.json \
 && /app/wistia-s3 -c /app/temp.json
