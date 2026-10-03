# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code files
COPY main.go ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build the Go app statically
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o oops .

# Final stage - Minimal image
FROM alpine:latest

# Add CA certificates, tzdata, docker CLI, and docker compose
RUN apk --no-cache add ca-certificates tzdata docker-cli docker-cli-compose

WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/oops /usr/local/bin/oops

# Expose port 8080 and 80 to the outside world
EXPOSE 8080 80

# Default entrypoint
ENTRYPOINT ["oops"]
CMD ["server"]
