FROM golang:1.19-alpine as builder

# Add Maintainer Info
LABEL maintainer="Sam Zhou <sam@mixmedia.com>"

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN go version \
 && export GO111MODULE=on \
 && export GOPROXY=https://goproxy.io,direct \
 && go mod vendor \
 && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o wistia-s3 \
 && chmod +x wistia-s3

######## Start a new stage from scratch #######
FROM alpine:latest  

RUN apk update \
 && apk add --update libintl \
 && apk add --no-cache tzdata dumb-init

WORKDIR /app

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/wistia-s3 .
COPY ./web /app/web
COPY ./webroot /app/webroot

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
 TZ="Asia/Hong_Kong" \
 LOG_LEVEL=INFO \
 WEBROOT=/app/web

EXPOSE 3031

ENTRYPOINT ["dumb-init", "--"]

CMD echo "{}" > /app/temp.json \
 && /app/wistia-s3 -c /app/temp.json
