FROM postgres:18-alpine

ENV SUPERCRONIC_URL=https://github.com/aptible/supercronic/releases/download/v0.2.46/supercronic-linux-amd64 \
    SUPERCRONIC=supercronic-linux-amd64 \
    SUPERCRONIC_SHA1SUM=5bcefed628e32adc08e32634db2d10e9230dbca0 \
    HOME=/tmp

RUN apk add --no-cache bash rclone curl \
    && curl -fsSLO "$SUPERCRONIC_URL" \
    && echo "${SUPERCRONIC_SHA1SUM}  ${SUPERCRONIC}" | sha1sum -c - \
    && chmod +x "$SUPERCRONIC" \
    && mv "$SUPERCRONIC" /usr/local/bin/supercronic \
    && apk del curl

COPY backup.sh /backup.sh
COPY crontab /etc/crontab
RUN chmod +x /backup.sh

USER postgres

ENTRYPOINT []
CMD ["supercronic", "/etc/crontab"]
