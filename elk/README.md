# ELK logging for vitamins-backend

Pipeline:

1. Go app writes structured JSON logs to `stdout` (`slog`).
2. Docker `gelf` logging driver forwards container logs to Logstash UDP input.
3. Logstash normalizes/filter fields and ships to Elasticsearch.
4. Kibana reads `vitamins-logs-*` indexes.

## Start

```bash
docker compose up -d --build
```

Endpoints:

- Elasticsearch: `http://localhost:9200`
- Logstash API: `http://localhost:9600`
- Kibana: `http://localhost:5601`

## What is provisioned automatically

- ILM policy: `logs-hot-delete-14d`
- Index template: `logs-template-v1`
- Kibana data view: `vitamins-logs-*`

## Useful Kibana filters

- app logs only: `channel: "app"`
- audit logs only: `channel: "audit"`
- one request: `request.id: "<request-id>"`
- one trace: `trace.id: "<trace-id>"`
- one user: `user.id: "123"`
- errors only: `log.level: "error"`
