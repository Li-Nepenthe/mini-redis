FROM golang:1.27.1-alpine3.24 AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mini-redis ./cmd/server \
    && mkdir -p /out/data

FROM scratch
COPY --from=build /out/mini-redis /mini-redis
COPY --from=build --chown=65532:65532 /out/data /data
USER 65532:65532
WORKDIR /data
VOLUME ["/data"]
EXPOSE 6379
STOPSIGNAL SIGTERM
ENTRYPOINT ["/mini-redis"]
CMD ["-addr", ":6379", "-aof", "/data/appendonly.aof"]
