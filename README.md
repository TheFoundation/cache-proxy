# cache-proxy

A simple caching forward HTTP proxy.

Can use a [Badger DB](https://dgraph.io/badger) backend for a standalone cache proxy or a [Redis](https://redis.io) backend for a shared cache proxy (multiple proxy instance can share the same cache).

## Quick start

Proxy something:

```bash
docker run --rm -it \
    -e UPSTREAM_URL=https://github.com \
    -p 8080:80 \
    pierredavidbelanger/cache-proxy:latest
```

Query it:

```bash
$ time curl localhost:8080/pierredavidbelanger/cache-proxy > /dev/null
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100 88362    0 88362    0     0   123k      0 --:--:-- --:--:-- --:--:--  123k

real    0m0.712s
user    0m0.009s
sys     0m0.005s

$ time curl localhost:8080/pierredavidbelanger/cache-proxy > /dev/null
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100 88362    0 88362    0     0  84.2M      0 --:--:-- --:--:-- --:--:-- 84.2M

real    0m0.011s
user    0m0.011s
sys     0m0.000s
```

## Configs

`UPSTREAM_URL`: (no default, must be set)

`LOG_LEVEL`: verbosity: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic` (default `info`)

`ATTEMPT_HTTP2`: should go http client attempt to use `HTTP2` (default `false`)

`TTL`: time to live of the entries in the cache (default `5m`)

`FRONTEND_URL`: (default `:8080` for the go version, or `:80` for the docker version)

`BACKEND_URL`: the cache backed, a valid `redis://` (see [redis options](https://github.com/go-redis/redis/blob/789ee0484f1126ca6aef3b56bb36cc8b51e16d20/options.go#L186)) or `badger://` URL spec (default `badger:///tmp/cache-proxy` for the go version, or `badger:///var/cache-proxy` for the docker version)
