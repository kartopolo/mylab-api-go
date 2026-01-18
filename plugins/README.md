# Plugins (Microservices)

This directory contains locally built plugin microservice binaries and gateway plugin configs.

## Layout

- `plugins/bin/` — microservice binaries
- `plugins/plugins.d/` — gateway plugin configs (`PLUGIN_DIR`)
- `plugins/env/` — env files for running plugin services locally
- `plugins/run-micro-*.sh` — helper scripts to run the binaries

## Quick test

1) Start microservices:

```bash
cd /var/www/mylab-api-go
./plugins/run-micro-dokter.sh
```

In another terminal:

```bash
cd /var/www/mylab-api-go
./plugins/run-micro-satusehat.sh
```

2) Ensure gateway `.env` has:

- `PLUGIN_DIR=/var/www/mylab-api-go/plugins/plugins.d`

3) Run gateway, then call:

- `GET /healthz` → aggregated gateway + plugins

## Run everything (one command)

`./plugins/run-all.sh` starts both microservices and the gateway.

Note: it defaults gateway `HTTP_ADDR` to `:18081` to avoid clashing with an existing dev gateway on `:18080`. Override via `GATEWAY_HTTP_ADDR=:18080 ./plugins/run-all.sh`.

## Only run microservices

If your gateway is already running (e.g. on `:18080`), you can start only the microservices:

`./plugins/run-micros.sh`

