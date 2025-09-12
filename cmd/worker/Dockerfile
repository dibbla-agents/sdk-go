FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o go-toolserver .

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS requests and subversion
RUN apk --no-cache add ca-certificates subversion

# Copy the binary from builder stage
COPY --from=builder /app/go-toolserver .

# Create .env file mount point
RUN mkdir -p /app/config

# Expose the port
EXPOSE 3010

# Set environment variables
ENV SERVER_NAME=go-toolserver
# Redis env removed (gRPC-only)

# Command to run the executable
CMD ["./go-toolserver"] 