FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o spotflac-server \
    ./cmd/server/

# ── Runtime ───────────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache \
    ca-certificates \
    ffmpeg \
    tzdata

WORKDIR /app
COPY --from=builder /app/spotflac-server .

RUN mkdir -p /data/music /data/cache && \
    addgroup -S spotflac && adduser -S spotflac -G spotflac && \
    chown -R spotflac:spotflac /app /data

USER spotflac

ENV PORT=8001
ENV SPOTFLAC_DATA_DIR=/data/cache

EXPOSE 8001

CMD ["./spotflac-server"]
