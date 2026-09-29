# Build stage: Compile static Go binary
FROM golang:alpine AS builder

WORKDIR /src

# Pre-fetch dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binary with stripped symbols and zero CGO
ARG VERSION=1.0.0
ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -extldflags '-static' -X main.Version=${VERSION}" \
    -o /bin/gateway-data ./cmd/gateway-data

# Final stage: Distroless static non-root (UID 65532)
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy statically linked binary
COPY --from=builder /bin/gateway-data /app/gateway-data

# HTTP ingress and gRPC ingress ports
EXPOSE 8080 9090

# Default bootstrap environment variables
ENV LISTEN_HTTP=":8080" \
    LISTEN_GRPC=":9090" \
    GATEWAY_NAMESPACE="default" \
    ENV="production"

LABEL org.opencontainers.image.title="torana-data" \
      org.opencontainers.image.description="Torana Enterprise Data Plane Gateway" \
      org.opencontainers.image.version="1.0.0" \
      org.opencontainers.image.licenses="Apache-2.0-with-managed-service-clause"

# Non-root user is already configured in distroless:nonroot (USER 65532:65532)
ENTRYPOINT ["/app/gateway-data"]
