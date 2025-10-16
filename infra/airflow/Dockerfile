FROM apache/airflow:2.9.2-python3.11

USER root
RUN apt-get update && \
    apt-get install -y --no-install-recommends golang-go && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

ENV PATH="/usr/lib/go/bin:/usr/local/go/bin:${PATH}"
USER airflow
