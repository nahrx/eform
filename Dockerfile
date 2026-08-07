# ============================================================
# Stage 1 — Build
# ============================================================
FROM golang:1.23-alpine AS builder

WORKDIR /src

# Download dependencies (this layer stays cached while go.mod/go.sum are unchanged)
COPY go.mod go.sum ./
RUN go mod download

# Copy the whole source tree
COPY . .

# Compile dua binary: server utama + seeder wilayah
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/eform-backend .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/eform-seeder ./cmd/seeder

# ============================================================
# Stage 2 — Runtime (image minimal)
# ============================================================
FROM alpine:3.20

# ca-certificates  : needed for outbound HTTPS (Google OAuth and so on)
# tzdata           : time zones (Asia/Makassar and so on)
# wget             : used by the Docker Compose healthcheck
RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

# Binaries from the build stage
COPY --from=builder /bin/eform-backend /bin/eform-seeder ./

# Static assets (web pages + landing page)
COPY web/    ./web/
COPY public/ ./public/

# Region data for the seeder
COPY data/   ./data/

EXPOSE 8080

CMD ["./eform-backend"]
