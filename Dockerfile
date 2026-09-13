# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy only the necessary source code files explicitly
COPY main.go ./
COPY internal/ ./internal/

# Build the Go app statically
# CGO_ENABLED=0 ensures a static binary
# -ldflags="-w -s" strips debug info to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o oops .

# Final stage - Minimal image
FROM alpine:latest

# Add CA certificates (needed for HTTPS webhooks/API calls)
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/oops .

# Expose port 80 to the outside world
EXPOSE 80

# Command to run the executable
CMD ["./oops"]
