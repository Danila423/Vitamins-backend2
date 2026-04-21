# Monitoring stack (Prometheus + Grafana + Alertmanager)

## Start

```bash
docker compose up -d --build
```

## Endpoints

- API metrics: `http://localhost:8080/metrics`
- Prometheus: `http://localhost:9090`
- Alertmanager: `http://localhost:9093`
- Grafana: `http://localhost:3000` (`admin` / `admin`)

## Included dashboards

- API Health
- Runtime / Resources
- Auth / Business
- DB / Cache

## Notes

- Metrics intentionally avoid high-cardinality labels and PII.
- HTTP metrics use bounded labels: `method`, `route`, `status_class`, `outcome`.

