FROM golang:1.21-bookworm AS builder

# Install libvips
RUN apt-get update && apt-get install -y libvips-dev && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o /dajtu ./cmd/dajtu

# Runtime
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y libvips42 ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /dajtu /usr/local/bin/dajtu

RUN useradd -r -s /bin/false dajtu
USER dajtu

EXPOSE 8080
CMD ["dajtu"]
