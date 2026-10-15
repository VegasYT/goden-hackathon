FROM busybox:1.37.0-uclibc AS tools

FROM grafana/loki:3.6.17

COPY --from=tools /bin/busybox /usr/bin/wget
