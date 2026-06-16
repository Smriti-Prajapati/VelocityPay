# ── Build stage ────────────────────────────────────────────────────────────────
FROM golang:1.26.3-alpine3.23 AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY . .

RUN go mod download && go mod verify

RUN CGO_ENABLED=0 \
    go build -ldflags="-w -s" \
    -o /app/velocitypay ./cmd/api

# ── Runtime stage ───────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

COPY --from=builder /app/velocitypay .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080

ENTRYPOINT ["./velocitypay"]
