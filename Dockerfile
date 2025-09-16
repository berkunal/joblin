# Multi-stage Dockerfile for Joblin CLI

# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o joblin \
    ./cmd/joblin

# Final stage - use distroless for security
FROM gcr.io/distroless/static-debian12:nonroot

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from builder stage
COPY --from=builder /app/joblin /usr/local/bin/joblin

# Copy documentation
COPY --from=builder /app/README.md /app/LICENSE /docs/

# Set the entrypoint
ENTRYPOINT ["joblin"]

# Default command
CMD ["--help"]

# Metadata
LABEL org.opencontainers.image.title="Joblin CLI" \
      org.opencontainers.image.description="CLI tool for deploying Python scripts to Kubernetes clusters" \
      org.opencontainers.image.vendor="berkunal" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.url="https://github.com/berkunal/joblin" \
      org.opencontainers.image.source="https://github.com/berkunal/joblin" \
      org.opencontainers.image.documentation="https://github.com/berkunal/joblin/blob/main/README.md"