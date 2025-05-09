FROM golang:1.24.3-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go.mod and go.sum files first for better caching
COPY src/go.mod src/go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY src/ .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o aisubs2 .

# Use a smaller base image for the final image
FROM alpine:3.21.3

# Install FFmpeg
RUN apk add --no-cache ffmpeg ca-certificates

# Create a non-root user to run the application
RUN adduser -D -h /app appuser
USER appuser

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/aisubs2 .

# Create directory for data persistence
RUN mkdir -p /app/data

# Expose the web service port
EXPOSE 8080

# Set the entrypoint to run the web service by default
ENTRYPOINT ["./aisubs2", "-s"]
