FROM golang:1.25.14-alpine AS builder
ARG VERSION=0.0.1
ARG BUILD_DATE=2026-09-14

WORKDIR /app
COPY events-service/go.mod ./
RUN if [ -f events-service/go.sum ]; then cp events-service/go.sum .; fi
COPY shared /shared
RUN go mod download
COPY events-service/. .
RUN CGO_ENABLED=0 GOOS=linux go build \
  -ldflags="-w -s -X main.version=$VERSION -X main.buildDate=$BUILD_DATE" \
  -o events-server ./cmd/server/main.go && \
  CGO_ENABLED=0 GOOS=linux go build \
  -ldflags="-w -s -X main.version=$VERSION -X main.buildDate=$BUILD_DATE" \
  -o events-worker ./cmd/worker/main.go

FROM alpine:3.18
ARG VERSION=0.0.1
ARG BUILD_DATE=2026-09-14
LABEL version=$VERSION \
  build-date=$BUILD_DATE \
  maintainer="me@xomrkob.ru"
RUN apk add --no-cache ca-certificates
RUN addgroup -g 1000 appgroup && \
  adduser -D -u 1000 -G appgroup appuser
WORKDIR /app
COPY --from=builder --chown=appuser:appgroup /app/events-server .
COPY --from=builder --chown=appuser:appgroup /app/events-worker .
RUN printf '#!/bin/sh\ncase "$1" in\n  worker) shift; exec /app/events-worker "$@" ;;\n  server|"") shift; exec /app/events-server "$@" ;;\n  *) exec "$@" ;;\nesac\n' > /app/entrypoint && \
  chmod +x /app/entrypoint
USER appuser
ENTRYPOINT ["/app/entrypoint"]
CMD ["server"]