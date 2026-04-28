# ELK-логирование для vitamins-backend

Пайплайн:

1. Go-приложение пишет структурированные JSON-логи в `stdout` (`slog`).
2. Docker logging driver `gelf` отправляет логи контейнеров в UDP input Logstash.
3. Logstash нормализует/фильтрует поля и пишет события в Elasticsearch.
4. Kibana читает индексы `vitamins-logs-*`.

## Запуск

```bash
docker compose -f deploy/docker-compose.yml up -d --build
```

## Endpoint'ы

- Elasticsearch: `http://localhost:9200`
- Logstash API: `http://localhost:9600`
- Kibana: `http://localhost:5601`

## Что настраивается автоматически

- ILM policy: `logs-hot-delete-14d`
- Index template: `logs-template-v1`
- Kibana data view: `vitamins-logs-*`

## Полезные фильтры в Kibana

- только app-логи: `channel: "app"`
- только audit-логи: `channel: "audit"`
- один запрос: `request.id: "<request-id>"`
- один trace: `trace.id: "<trace-id>"`
- один пользователь: `user.id: "123"`
- только ошибки: `log.level: "error"`
