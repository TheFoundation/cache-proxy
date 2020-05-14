FROM golang:1.14-alpine3.11 AS golang
WORKDIR /src
COPY . ./
RUN go build -o rcp

FROM alpine:3.11
ENTRYPOINT ["/usr/local/bin/rcp"]
EXPOSE 80
ENV RCP_UPSTREAM_URL="http://upstream" \
    RCP_REDIS_URL="redis://redis:6379" \
    RCP_CACHE_PREFIX="rcp" \
    RCP_CACHE_TTL="5m" \
    RCP_FRONTEND_URL=":80"
COPY --from=golang /src/rcp /usr/local/bin/rcp
