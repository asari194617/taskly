FROM apache/airflow:2.9.2-python3.11

USER root
RUN apt-get update && \
    apt-get install -y --no-install-recommends git ca-certificates default-mysql-client gzip curl && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Configure Go environment
ENV GOPATH=/home/airflow/go \
    GOMODCACHE=/home/airflow/go/pkg/mod \
    GOCACHE=/home/airflow/.cache/go-build \
    PATH="/usr/local/go/bin:/home/airflow/go/bin:/usr/lib/go/bin:/usr/local/bin:/home/airflow/.local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

RUN mkdir -p /home/airflow/go/bin /home/airflow/go/pkg/mod /home/airflow/.cache/go-build && \
    chown -R airflow:root /home/airflow/go /home/airflow/.cache

# Install modern Go toolchain (1.22.x)
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in \
      amd64) GOARCH=amd64 ;; \
      arm64) GOARCH=arm64 ;; \
      *) echo "Unsupported architecture: $arch"; exit 1 ;; \
    esac; \
    curl -fsSL "https://go.dev/dl/go1.22.7.linux-$GOARCH.tar.gz" -o /tmp/go.tgz; \
    rm -rf /usr/local/go; \
    tar -C /usr/local -xzf /tmp/go.tgz; \
        rm /tmp/go.tgz; \
        ln -sf /usr/local/go/bin/go /usr/bin/go; \
        ln -sf /usr/local/go/bin/gofmt /usr/bin/gofmt

USER airflow

# Python deps for integrations: Sentry, Datadog tracer, Discord provider
# IMPORTANT: Use Airflow constraints to avoid upgrading Airflow itself and keep provider compatibility
ARG AIRFLOW_VERSION=2.9.2
ARG AIRFLOW_CONSTRAINTS_URL="https://raw.githubusercontent.com/apache/airflow/constraints-${AIRFLOW_VERSION}/constraints-3.11.txt"
RUN pip install --no-cache-dir --constraint "${AIRFLOW_CONSTRAINTS_URL}" \
    sentry-sdk \
    ddtrace \
    apache-airflow-providers-discord
