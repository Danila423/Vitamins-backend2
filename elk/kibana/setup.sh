#!/bin/sh
set -e

KIBANA_URL="http://kibana:5601"

until curl -fsS "${KIBANA_URL}/api/status" >/dev/null 2>&1; do
  sleep 2
done

curl -fsS -X POST "${KIBANA_URL}/api/data_views/data_view" \
  -H "kbn-xsrf: true" \
  -H "Content-Type: application/json" \
  -d '{
    "data_view": {
      "name": "Vitamins logs",
      "title": "vitamins-logs-*",
      "timeFieldName": "@timestamp"
    }
  }' >/dev/null 2>&1 || true

curl -fsS -X POST "${KIBANA_URL}/api/saved_objects/search/logs-errors-search" \
  -H "kbn-xsrf: true" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "title": "Errors by request",
      "description": "Error logs with request correlation",
      "columns": ["@timestamp", "service.name", "channel", "operation", "request.id", "trace.id", "user.id", "error.message"],
      "sort": [["@timestamp", "desc"]],
      "kibanaSavedObjectMeta": {
        "searchSourceJSON": "{\"index\":\"vitamins-logs-*\",\"query\":{\"language\":\"kuery\",\"query\":\"log.level: error\"},\"filter\":[]}"
      }
    }
  }' >/dev/null 2>&1 || true

echo "kibana setup complete"
