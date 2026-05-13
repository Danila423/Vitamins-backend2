# Observability архитектура VitaInfo backend

Актуально для полного стека `deploy/docker-compose.yml`.

```mermaid
flowchart LR
  subgraph APP[Сервисы приложения]
    gw[gateway\n:8080]
    auth[auth-service\n:50051]
    vit[vitamins-service\n:50052]
    an[analytics-service\n:50053]
    notifier[notifier\nworker]
  end

  subgraph MET[Monitoring / Metrics]
    met{{/metrics endpoints}}
    prom[Prometheus :9090]
    graf[Grafana :3000]
    alert[Alertmanager :9093]
    pexp[postgres-exporter :9187]
    rexp[redis-exporter :9121]
  end

  subgraph LOG[ELK / Логи]
    logs{{GELF logs}}
    ls[Logstash\nUDP :12201]
    es[(Elasticsearch :9200)]
    kb[Kibana :5601]
  end

  subgraph DATA[Инфраструктура данных]
    pg[(PostgreSQL 16)]
    redis[(Redis 7)]
  end

  gw --> met
  auth --> met
  vit --> met
  an --> met
  notifier --> met
  pexp --> met
  rexp --> met

  prom -->|scrape| met
  graf --> prom
  prom --> alert

  pg --> pexp
  redis --> rexp

  gw -.-> logs
  auth -.-> logs
  vit -.-> logs
  an -.-> logs
  notifier -.-> logs

  logs --> ls
  ls --> es
  kb --> es
```
