# syntax=docker/dockerfile:1.7

# Build on the host arch to avoid QEMU overhead, cross-compile per target platform
ARG BUILDPLATFORM=linux/amd64
ARG TARGETPLATFORM=linux/amd64
ARG TARGETOS=linux
ARG TARGETARCH=amd64

FROM --platform=$TARGETPLATFORM alpine:3.24 AS brotli-build
ARG BROTLI_COMMIT=db24a98aed38a16de78cc05ca15cdd8dc2a0d92d
ARG BROTLI_SHA256=113b9685dfc14dd2f3360d077283266eec2fbdfb0fe3e5600f11be151da0d68e
RUN apk add --no-cache build-base ca-certificates cmake curl gzip tar \
	&& curl --fail --location --retry 5 \
		--output /tmp/brotli.tar.gz \
		"https://codeload.github.com/google/brotli/tar.gz/${BROTLI_COMMIT}" \
	&& echo "${BROTLI_SHA256}  /tmp/brotli.tar.gz" | sha256sum -c - \
	&& mkdir /src \
	&& tar --extract --gzip --file /tmp/brotli.tar.gz --directory /src --strip-components=1 \
	&& cmake -S /src -B /build \
		-DCMAKE_BUILD_TYPE=Release \
		-DCMAKE_INSTALL_PREFIX=/opt/brotli \
		-DCMAKE_INSTALL_LIBDIR=lib \
		-DBROTLI_DISABLE_TESTS=ON \
	&& cmake --build /build --parallel \
	&& cmake --install /build \
	&& mkdir /opt/brotli-runtime \
	&& cp -a /opt/brotli/lib/libbrotli*.so* /opt/brotli-runtime/

FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine AS build
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
	go build -buildvcs=false -trimpath -ldflags "-s -w" -tags timetzdata -o /out/api ./cmd/server

# Extract CA certificates (for outbound HTTPS if needed)
FROM --platform=$BUILDPLATFORM alpine:3.24 AS certs
RUN apk add --no-cache ca-certificates

FROM --platform=$TARGETPLATFORM alpine:3.24
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata bash curl grep sed findutils
COPY --from=brotli-build /opt/brotli-runtime/ /usr/local/lib/
ENV LD_LIBRARY_PATH=/usr/local/lib
RUN curl --version | grep -q 'brotli/1.2.1'
COPY --from=build /out/api /app/api
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENV API_PORT=8080
EXPOSE 8080 9464

# Entrypoint runs the API; backup scheduling is owned by Airflow.
COPY ./scripts/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
