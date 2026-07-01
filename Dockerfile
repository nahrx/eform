# ============================================================
# Stage 1 — Build
# ============================================================
FROM golang:1.23-alpine AS builder

WORKDIR /src

# Download dependencies (layer di-cache selama go.mod/go.sum tidak berubah)
COPY go.mod go.sum ./
RUN go mod download

# Copy seluruh source
COPY . .

# Compile dua binary: server utama + seeder wilayah
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/eform-backend .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/eform-seeder ./cmd/seeder

# ============================================================
# Stage 2 — Runtime (image minimal)
# ============================================================
FROM alpine:3.20

# ca-certificates  : diperlukan untuk HTTPS keluar (OAuth Google, dll.)
# tzdata           : zona waktu (Asia/Makassar, dll.)
# wget             : dipakai healthcheck di Docker Compose
RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

# Binary dari stage build
COPY --from=builder /bin/eform-backend /bin/eform-seeder ./

# Aset statis (halaman web + landing page)
COPY web/    ./web/
COPY public/ ./public/

# Data wilayah untuk seeder
COPY data/   ./data/

EXPOSE 8080

CMD ["./eform-backend"]
