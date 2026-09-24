FROM mcr.microsoft.com/playwright:v1.63.0-noble

ARG RUNNER_VERSION=2.337.0
ARG RUNNER_SHA256=70920811a4f8ad4328818682bca5c6469c1c942fab52448868071d0063816613

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      docker.io \
      docker-compose-v2 \
      build-essential \
      python3 \
      python3-venv \
      zstd \
      git \
      jq \
      libcurl4 \
      libicu74 \
      libkrb5-3 \
      liblttng-ust1 \
      libssl3 \
      libxml2 \
      openssh-client \
      rsync \
      sudo \
      unzip \
      zip \
      zlib1g \
    && rm -rf /var/lib/apt/lists/* \
    && userdel --remove pwuser \
    && useradd --create-home --uid 1001 --shell /bin/bash runner \
    && install -d -o runner -g runner /home/runner/actions-runner \
    && curl --fail --location --retry 5 \
      --output /tmp/actions-runner.tar.gz \
      "https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz" \
    && echo "${RUNNER_SHA256}  /tmp/actions-runner.tar.gz" | sha256sum --check - \
    && tar --extract --gzip --file /tmp/actions-runner.tar.gz \
      --directory /home/runner/actions-runner \
    && rm /tmp/actions-runner.tar.gz \
    && chown -R runner:runner /home/runner/actions-runner

COPY entrypoint.sh /usr/local/bin/taskly-runner-entrypoint

RUN chmod 0755 /usr/local/bin/taskly-runner-entrypoint

USER runner
WORKDIR /home/runner/actions-runner

ENTRYPOINT ["/usr/local/bin/taskly-runner-entrypoint"]
