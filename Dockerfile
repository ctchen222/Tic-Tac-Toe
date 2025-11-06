# Stage 1: Build the Go application
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum to download dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
# CGO_ENABLED=0 is used to build a statically linked binary
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

# Stage 2: Create the final lightweight image
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /server .

# Copy the web static files
COPY web ./web

# Expose the port the app runs on
EXPOSE 8080

# Command to run the executable
CMD ["./server"]
