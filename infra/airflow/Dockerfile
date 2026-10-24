# syntax=docker/dockerfile:1.7

FROM debian:bookworm-slim AS brotli-build

ARG BROTLI_COMMIT=db24a98aed38a16de78cc05ca15cdd8dc2a0d92d
ARG BROTLI_SHA256=113b9685dfc14dd2f3360d077283266eec2fbdfb0fe3e5600f11be151da0d68e

RUN apt-get update && \
    apt-get install -y --no-install-recommends build-essential ca-certificates cmake curl gzip tar && \
    curl --fail --location --retry 5 \
      --output /tmp/brotli.tar.gz \
      "https://codeload.github.com/google/brotli/tar.gz/${BROTLI_COMMIT}" && \
    echo "${BROTLI_SHA256}  /tmp/brotli.tar.gz" | sha256sum --check - && \
    mkdir /src && \
    tar --extract --gzip --file /tmp/brotli.tar.gz --directory /src --strip-components=1 && \
    cmake -S /src -B /build \
      -DCMAKE_BUILD_TYPE=Release \
      -DCMAKE_INSTALL_PREFIX=/opt/brotli \
      -DCMAKE_INSTALL_LIBDIR=lib \
      -DBROTLI_DISABLE_TESTS=ON && \
    cmake --build /build --parallel && \
    cmake --install /build && \
    mkdir /opt/brotli-runtime && \
    cp -a /opt/brotli/lib/libbrotli*.so* /opt/brotli-runtime/

FROM --platform=$BUILDPLATFORM golang:1.26.10-bookworm AS batch-builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY batches/go.mod batches/go.sum ./batches/
COPY batches/notifications/delivery-channels/common/go.mod batches/notifications/delivery-channels/common/go.sum ./batches/notifications/delivery-channels/common/
WORKDIR /src/batches
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    go mod download
COPY batches/ ./
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    set -eux; \
    mkdir -p /out/bin; \
    export CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH"; \
    go build -trimpath -o /out/bin/backup-mysql ./backup-mysql; \
    go build -trimpath -o /out/bin/moderation ./moderation; \
    go build -trimpath -o /out/bin/day-shift-rotations ./day-shift-rotations; \
    go build -trimpath -o /out/bin/notifications-que-manager ./notifications/que-manager; \
    (cd notifications/delivery-channels/email && go build -trimpath -o /out/bin/notifications-delivery-channels-email .); \
    (cd notifications/delivery-channels/discord && go build -trimpath -o /out/bin/notifications-delivery-channels-discord .); \
    (cd notifications/delivery-channels/web-push && go build -trimpath -o /out/bin/notifications-delivery-channels-web-push .)

# MariaDB's dump client includes MySQL generated columns in INSERT statements.
# Use the client matching the production MySQL server so the dump can be restored.
FROM mysql:8.4 AS mysql-client

FROM apache/airflow:3.3.0-python3.14

USER root
RUN apt-get update && \
    apt-get install -y --no-install-recommends git ca-certificates default-mysql-client gzip curl && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Debian bookworm has not published a fixed libbrotli package yet. Prefer the
# pinned upstream 1.2.1 libraries and fail the build if curl cannot see them.
COPY --from=brotli-build /opt/brotli-runtime/ /usr/local/lib/
RUN ldconfig && curl --version | grep -q 'brotli/1.2.1'

RUN mkdir -p /opt/airflow/bin /opt/airflow/backups /opt/airflow/logs && \
    chown -R airflow:root /opt/airflow/bin /opt/airflow/backups /opt/airflow/logs

USER airflow

# Python deps for integrations: Sentry, Datadog tracer, Discord provider
# IMPORTANT: Use Airflow constraints to avoid upgrading Airflow itself and keep provider compatibility
ARG AIRFLOW_VERSION=3.3.0
ARG AIRFLOW_CONSTRAINTS_URL="https://raw.githubusercontent.com/apache/airflow/constraints-${AIRFLOW_VERSION}/constraints-3.14.txt"
RUN pip install --no-cache-dir --constraint "${AIRFLOW_CONSTRAINTS_URL}" \
    sentry-sdk \
    ddtrace \
    apache-airflow-providers-discord \
    grpcio \
    protobuf \
    pymysql \
    icalendar \
    python-dateutil \
	python-dotenv \
	boto3

# Keep the expensive OS/Python dependency layers independent from batch source changes.
COPY --from=batch-builder --chown=airflow:root /out/bin/ /opt/airflow/bin/
COPY --from=mysql-client /usr/bin/mysqldump /usr/local/bin/mysqldump

# Avoid loading Airflow and its providers for every periodic health probe.
COPY --chown=airflow:root infra/airflow/healthcheck.py /opt/airflow/healthcheck.py
