FROM golang:1.26.5-bookworm AS batch-builder

WORKDIR /src
COPY batches/go.mod batches/go.sum ./batches/
WORKDIR /src/batches
RUN go mod download
COPY batches/ ./
RUN set -eux; \
    mkdir -p /out/bin; \
    go build -trimpath -o /out/bin/backup-mysql ./backup-mysql; \
    go build -trimpath -o /out/bin/moderation ./moderation; \
    go build -trimpath -o /out/bin/day-shift-rotations ./day-shift-rotations; \
    go build -trimpath -o /out/bin/notifications-que-manager ./notifications/que-manager; \
    cd notifications/delivery-channels/email && go build -trimpath -o /out/bin/notifications-delivery-channels-email .; \
    cd /src/batches && cd notifications/delivery-channels/discord && go build -trimpath -o /out/bin/notifications-delivery-channels-discord .; \
    cd /src/batches && cd notifications/delivery-channels/web-push && go build -trimpath -o /out/bin/notifications-delivery-channels-web-push .

FROM apache/airflow:3.3.0-python3.14

USER root
RUN apt-get update && \
    apt-get install -y --no-install-recommends git ca-certificates default-mysql-client gzip curl && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /opt/airflow/bin && chown -R airflow:root /opt/airflow/bin
COPY --from=batch-builder --chown=airflow:root /out/bin/ /opt/airflow/bin/

USER airflow

# Python deps for integrations: Sentry, Datadog tracer, Discord provider
# IMPORTANT: Use Airflow constraints to avoid upgrading Airflow itself and keep provider compatibility
ARG AIRFLOW_VERSION=3.2.1
ARG AIRFLOW_CONSTRAINTS_URL="https://raw.githubusercontent.com/apache/airflow/constraints-${AIRFLOW_VERSION}/constraints-3.14.txt"
RUN pip install --no-cache-dir --constraint "${AIRFLOW_CONSTRAINTS_URL}" \
    sentry-sdk \
    ddtrace \
    apache-airflow-providers-discord \
    grpcio \
    protobuf \
    pymysql \
    icalendar \
    python-dotenv