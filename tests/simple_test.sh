#!/bin/bash

# Простой скрипт для тестирования API без Python

BASE_URL="http://localhost:8080"

echo "=========================================="
echo "Тестирование Highload Service"
echo "=========================================="

# Проверка здоровья
echo -e "\n1. Проверка /health"
curl -s "$BASE_URL/health" | jq .

# Отправка метрики
echo -e "\n2. Отправка тестовой метрики"
curl -s -X POST "$BASE_URL/api/metrics" \
  -H "Content-Type: application/json" \
  -d '{
    "timestamp": '$(date +%s)',
    "device_id": "device_test",
    "cpu": 65.5,
    "rps": 250.0,
    "memory": 55.0
  }' | jq .

# Небольшая задержка
sleep 1

# Получение анализа
echo -e "\n3. Получение анализа для устройства"
curl -s "$BASE_URL/api/analyze?device_id=device_test" | jq .

# Отправка нескольких метрик для генерации данных
echo -e "\n4. Отправка серии метрик..."
for i in {1..10}; do
  CPU=$(awk -v min=20 -v max=80 'BEGIN{srand(); print min+rand()*(max-min)}')
  curl -s -X POST "$BASE_URL/api/metrics" \
    -H "Content-Type: application/json" \
    -d '{
      "timestamp": '$(date +%s)',
      "device_id": "device_test",
      "cpu": '$CPU',
      "rps": 300.0,
      "memory": 60.0
    }' > /dev/null
  echo -n "."
done

echo -e "\n\n5. Получение обновленного анализа"
curl -s "$BASE_URL/api/analyze?device_id=device_test" | jq .

# Отправка аномальной метрики
echo -e "\n6. Отправка аномальной метрики (высокий CPU)"
curl -s -X POST "$BASE_URL/api/metrics" \
  -H "Content-Type: application/json" \
  -d '{
    "timestamp": '$(date +%s)',
    "device_id": "device_test",
    "cpu": 95.0,
    "rps": 500.0,
    "memory": 85.0
  }' | jq .

sleep 1

# Проверка аномалий
echo -e "\n7. Проверка обнаруженных аномалий"
curl -s "$BASE_URL/api/anomalies" | jq .

# Prometheus метрики
echo -e "\n8. Prometheus метрики (первые 20 строк)"
curl -s "$BASE_URL/metrics" | head -20

echo -e "\n=========================================="
echo "Тестирование завершено!"
echo "=========================================="

