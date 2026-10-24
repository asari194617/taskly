FROM node:24.22.1-bookworm-slim AS node-runtime

FROM debian:bookworm-slim AS brotli-build

ARG BROTLI_COMMIT=db24a98aed38a16de78cc05ca15cdd8dc2a0d92d
ARG BROTLI_SHA256=113b9685dfc14dd2f3360d077283266eec2fbdfb0fe3e5600f11be151da0d68e

RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential ca-certificates cmake curl gzip tar \
    && curl --fail --location --retry 5 \
      --output /tmp/brotli.tar.gz \
      "https://codeload.github.com/google/brotli/tar.gz/${BROTLI_COMMIT}" \
    && echo "${BROTLI_SHA256}  /tmp/brotli.tar.gz" | sha256sum --check - \
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

FROM mcr.microsoft.com/playwright:v1.64.0-noble

ARG RUNNER_VERSION=2.338.1
ARG RUNNER_SHA256=08e9ccb3604d2462c24b5e33604dc30810296ca15cc65f23cffd4317eb144c37

ENV DEBIAN_FRONTEND=noninteractive

# Playwright 1.64.0 still ships Node 24.21.0. Replace it with the patched
# runtime and override Ubuntu's vulnerable libbrotli for curl and other tools.
COPY --from=node-runtime /usr/local/ /usr/local/
COPY --from=brotli-build /opt/brotli-runtime/ /usr/local/lib/

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
    && ldconfig \
    && test "$(node --version)" = "v24.22.1" \
    && curl --version | grep -q 'brotli/1.2.1' \
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
    && test "$(/home/runner/actions-runner/externals/node24/bin/node --version)" = "v24.22.1" \
    && chown -R runner:runner /home/runner/actions-runner

COPY entrypoint.sh /usr/local/bin/taskly-runner-entrypoint

RUN chmod 0755 /usr/local/bin/taskly-runner-entrypoint

USER runner
WORKDIR /home/runner/actions-runner

ENTRYPOINT ["/usr/local/bin/taskly-runner-entrypoint"]
