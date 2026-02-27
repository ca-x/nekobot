# Frontend build stage
FROM node:22-alpine AS frontend

WORKDIR /build/frontend

COPY pkg/webui/frontend/package.json pkg/webui/frontend/package-lock.json ./
RUN npm ci

COPY pkg/webui/frontend/ .
RUN npm run build

# Build stage
FROM golang:1.26-alpine AS builder

ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG GIT_COMMIT=unknown
ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

COPY --from=frontend /build/frontend/dist ./pkg/webui/frontend/dist

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags="-s -w \
      -X 'nekobot/pkg/version.Version=${VERSION}' \
      -X 'nekobot/pkg/version.BuildTime=${BUILD_TIME}' \
      -X 'nekobot/pkg/version.GitCommit=${GIT_COMMIT}'" \
    -o /out/nekobot ./cmd/nekobot

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata tmux

RUN addgroup -g 1000 app && adduser -D -u 1000 -G app app

WORKDIR /app

COPY --from=builder /out/nekobot /app/nekobot

RUN mkdir -p /app/config /app/workspace && chown -R app:app /app

USER app

ENV NEKOBOT_CONFIG_FILE=/app/config/config.json

EXPOSE 18790 18791

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:18790/health || exit 1

CMD ["./nekobot", "gateway"]
