FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o clubhouse-server ./cmd/server

FROM alpine:latest

RUN apk add --no-cache ca-certificates curl postgresql-client

# Install golang-migrate
ARG TARGETARCH
RUN set -eux; \
    arch="$TARGETARCH"; \
    if [ -z "$arch" ]; then \
        arch="$(uname -m)"; \
        case "$arch" in \
            x86_64) arch=amd64 ;; \
            aarch64) arch=arm64 ;; \
            *) echo "unsupported arch: $arch" >&2; exit 1 ;; \
        esac; \
    fi; \
    case "$arch" in \
        amd64|arm64) ;; \
        *) echo "unsupported TARGETARCH: $arch" >&2; exit 1 ;; \
    esac; \
    curl -L "https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-${arch}.tar.gz" | tar xvz; \
    mv migrate /usr/local/bin/migrate

WORKDIR /app

COPY --from=builder /app/clubhouse-server .
COPY --from=builder /app/migrations ./migrations
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["./entrypoint.sh"]
