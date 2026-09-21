# syntax=docker/dockerfile:1.7

# Build on the host arch to avoid QEMU overhead, cross-compile per target platform
ARG BUILDPLATFORM=linux/amd64
ARG TARGETPLATFORM=linux/amd64
ARG TARGETOS=linux
ARG TARGETARCH=amd64

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
WORKDIR /src
ENV GOTOOLCHAIN=auto
# Copy the server source tree after the refactor to a nested layout
COPY . .
# Cache go modules
RUN --mount=type=cache,target=/go/pkg/mod \
	GOPROXY=https://proxy.golang.org,direct go mod download
# Build with cache mounts
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
	go build -buildvcs=false -trimpath -ldflags "-s -w" -tags timetzdata -o /out/api ./cmd/server && \
	CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
	go build -buildvcs=false -trimpath -ldflags "-s -w" -o /out/migrate ./cmd/migrate

# Extract CA certificates (for outbound HTTPS if needed)
FROM --platform=$BUILDPLATFORM alpine:3.24 AS certs
RUN apk add --no-cache ca-certificates

FROM --platform=$TARGETPLATFORM alpine:3.24
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata bash curl grep sed findutils
COPY --from=build /out/api /app/api
COPY --from=build /out/migrate /app/migrate
COPY ./migrations /app/migrations
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENV API_PORT=8080
EXPOSE 8080 9464

# Entrypoint runs the API; backup scheduling is owned by Airflow.
COPY ./scripts/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
