# Monitoring stack (Prometheus + Grafana + Alertmanager)

## Запуск

```bash
docker compose -f deploy/docker-compose.yml up -d --build
```

## Основные endpoint'ы

- Метрики gateway: `http://localhost:8080/metrics`
- Prometheus: `http://localhost:9090`
- Alertmanager: `http://localhost:9093`
- Grafana: `http://localhost:3000` (`admin` / `admin`)

## Включенные дашборды

- API Health
- Runtime / Resources
- Auth / Business
- DB / Cache
