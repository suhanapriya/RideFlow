# Multi-stage Dockerfile for Go services
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Copy the replaced dependency so go mod download can see it
COPY third_party ./third_party

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build argument for service name
ARG SERVICE_NAME
ARG VERSION
ARG BUILD_TIME

# Build the service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags "-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" -o /app/service ./cmd/${SERVICE_NAME}

# Final stage
FROM alpine:3.20

# Install ca-certificates for HTTPS and wget for healthcheck
RUN apk --no-cache add ca-certificates wget

# Add LABEL instructions
LABEL maintainer="Uber Team" \
      service.name="${SERVICE_NAME}" \
      version="${VERSION}"

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /home/appuser

# Copy the binary from builder
COPY --from=builder /app/service .

# Run as non-root user
USER appuser

# Expose port
EXPOSE 8080

# Healthcheck
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/healthz"] || exit 1

# Run the service
CMD ["./service"]
