# Stage 1: Build the binary
FROM golang:1.26.1-alpine AS builder

# Install git and ca-certificates
RUN apk update && apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy dependency files and download them
COPY go.mod go.sum ./
RUN go mod download

# Copy the actual source code
COPY . .

# Build the server binary statically
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o stac-server ./cmd/server

# Stage 2: Create the tiny production image
FROM alpine:latest

# We MUST copy the SSL certificates so Go can fetch https:// schemas
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/stac-server .

# Expose the default port
EXPOSE 8080

# Run the binary
CMD ["./stac-server"]