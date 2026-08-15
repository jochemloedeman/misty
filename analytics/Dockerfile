FROM python:3.13-slim

ENV DUCKDB_VERSION=1.5.3 \
    SUPERCRONIC_VERSION=0.2.46 \
    TZ=UTC

ARG TARGETARCH
ARG DUCKDB_EXTENSION_DIR=/opt/duckdb/extensions

RUN set -eux; \
    case "$TARGETARCH" in \
      amd64) duckdb_sha=35caef1fecbc8d7e2c07de4fd2cdefc5189ec9ba9e1cca228fb1a1c48cc52a8a; \
             supercronic_sha=5bcefed628e32adc08e32634db2d10e9230dbca0 ;; \
      arm64) duckdb_sha=5e2399428793642e994f1584c47d49f4c58b7b4ec2297ea4a522353a6c553835; \
             supercronic_sha=639ab81a72771990790df7ee87d9acfe88e5fa83 ;; \
      *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates curl unzip; \
    \
    curl -fsSLO "https://github.com/duckdb/duckdb/releases/download/v${DUCKDB_VERSION}/duckdb_cli-linux-${TARGETARCH}.zip"; \
    echo "${duckdb_sha}  duckdb_cli-linux-${TARGETARCH}.zip" | sha256sum -c -; \
    unzip -q "duckdb_cli-linux-${TARGETARCH}.zip" -d /usr/local/bin; \
    rm "duckdb_cli-linux-${TARGETARCH}.zip"; \
    chmod +x /usr/local/bin/duckdb; \
    \
    curl -fsSLO "https://github.com/aptible/supercronic/releases/download/v${SUPERCRONIC_VERSION}/supercronic-linux-${TARGETARCH}"; \
    echo "${supercronic_sha}  supercronic-linux-${TARGETARCH}" | sha1sum -c -; \
    chmod +x "supercronic-linux-${TARGETARCH}"; \
    mv "supercronic-linux-${TARGETARCH}" /usr/local/bin/supercronic; \
    \
    apt-get purge -y curl unzip; \
    apt-get autoremove -y; \
    rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir mcp-server-motherduck==1.0.7

RUN set -eux; \
    duckdb -c "SET extension_directory = '${DUCKDB_EXTENSION_DIR}'; INSTALL postgres;"; \
    chmod -R a+rX /opt/duckdb

COPY --chmod=0755 snapshot.sh /snapshot.sh
COPY snapshot.sql /snapshot.sql
COPY crontab /etc/supercronic/misty

RUN install -d -o 65534 -g 65534 /data

USER 65534:65534

CMD ["supercronic", "-passthrough-logs", "/etc/supercronic/misty"]
