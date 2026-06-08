# Stage 1: Build
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git gcc musl-dev && \
    go install github.com/a-h/templ/cmd/templ@latest

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN templ generate && \
    CGO_ENABLED=0 go build -ldflags="-w -s" -o /neptune ./cmd/neptune

# Stage 2: Runtime
FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata curl && \
    adduser -D -u 65532 -h /app -s /sbin/nologin neptune

WORKDIR /app
COPY --from=builder /neptune .
COPY migrations/ ./migrations/

RUN mkdir -p data && chown -R neptune:neptune /app
USER neptune

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --retries=3 CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["./neptune"]
