FROM golang:alpine AS builder

ARG MAIN_PATH

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Use another cache mount for the Go build cache itself, which stores
# compiled packages.
RUN --mount=type=cache,target=/root/.cache/go-build \
    go env -w GOCACHE=/root/.cache/go-build

# Copy source code
COPY . .

# Build the application
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -v -o server $MAIN_PATH

FROM alpine AS certs
RUN apk add -U --no-cache ca-certificates

FROM scratch

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/server .
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Expose port
EXPOSE 4444

# Run the application
CMD ["./server"]
