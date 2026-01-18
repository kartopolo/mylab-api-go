# Plugin config examples

These JSON files are examples for `PLUGIN_DIR` configuration used by the gateway (`mylab-api-go`).

## Usage

- Set `PLUGIN_DIR` to a directory containing `*.json` plugin configs.
- Gateway serves plugin proxy under `/v1/plugins/*`.

Example:

```bash
export PLUGIN_DIR=/var/www/mylab-api-go/Docs/dev/plugin-config-examples
```

Then call:

- `/v1/plugins/dokter/healthz` → forwards to `micro-dokter-go` `/healthz`
- `/v1/plugins/satusehat/v1/satusehat/status` → forwards to `micro-satusehat-go` `/v1/satusehat/status`

## Gateway health aggregation

If `PLUGIN_DIR` is configured, `GET /healthz` on the gateway will also probe `/healthz` on each configured plugin upstream and return an aggregated JSON report.
