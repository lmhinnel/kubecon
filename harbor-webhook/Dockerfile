# ── Stage 1: fetch Go module dependencies ─────────────────────────────────────
FROM golang:1.25-alpine AS deps
RUN apk add --no-cache git ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

# ── Stage 2: build the binary ─────────────────────────────────────────────────
FROM deps AS builder
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -trimpath -o /harbor-webhook-signer .

# ── Stage 3: minimal distroless runtime ───────────────────────────────────────
# cosign v2 is used as a Go library — no cosign CLI binary needed.
# The distroless image runs as uid 65532 (nonroot) with no shell.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /harbor-webhook-signer /harbor-webhook-signer

EXPOSE 8080
ENTRYPOINT ["/harbor-webhook-signer"]
