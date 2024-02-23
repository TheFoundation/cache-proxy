#FROM golang:1.14-buster AS golang
FROM golang:alpine as build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN go build -o cache-proxy

#FROM debian:buster
FROM alpine
CMD ["/usr/local/bin/cache-proxy"]
EXPOSE 80
ENV LOG_LEVEL="info" \
    ATTEMPT_HTTP2="false" \
    TTL="5m" \
    FRONTEND_URL=":80" \
    BACKEND_URL="badger:///var/cache-proxy"
COPY --from=golang /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=golang /src/cache-proxy /usr/local/bin/cache-proxy
