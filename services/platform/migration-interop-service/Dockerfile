# syntax=docker/dockerfile:1.12

# ── build ──────────────────────────────────────────────────────────────────
FROM golang:1.25-bookworm AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /bin/server ./cmd/server

# ── final — distroless (no shell, no libc, minimal attack surface) ─────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /bin/server /server

EXPOSE 50051 8080 9090

ENTRYPOINT ["/server"]
