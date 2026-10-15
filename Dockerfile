FROM alpine:3.23@sha256:85fe1e81d6758c208f3e1eed4338a1997e19d4be002d4dd32d3100c9a8c010a0
ARG TARGETPLATFORM
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -S -D -H -u 10001 -G users -h /nonexistent -s /sbin/nologin energything && \
    mkdir /data && chown energything:users /data
COPY bin/${TARGETPLATFORM}/energything /usr/local/bin/energything
COPY bin/${TARGETPLATFORM}/energything-enqueue /usr/local/bin/energything-enqueue
ENV ENERGYTHING_HTTP_ADDR=:8080 ENERGYTHING_DATA_DIR=/data ENERGYTHING_TIMEZONE=Europe/London TZ=Europe/London
VOLUME ["/data"]
EXPOSE 8080
USER 10001:100
ENTRYPOINT ["/usr/local/bin/energything"]
