# Service с ИИ-аналитикой на Go

Высоконагруженный сервис для обработки потоковых данных от IoT-устройств с аналитикой на основе статистических методов, развернутый в Kubernetes с автомасштабированием.

## Возможности

- Обработка потоковых метрик от IoT-устройств (CPU, RPS, Memory)
- Кэширование в Redis
- Статистическая аналитика:
  - Rolling Average (скользящее среднее) с окном 50 событий
  - Z-Score детекция аномалий (порог > 2σ)
- Prometheus метрики для мониторинга
- Автомасштабирование (HPA) на основе CPU/Memory
- Grafana дашборды для визуализации
- Обработка до 1000+ RPS

## Архитектура

```
IoT Devices → Go Service → Redis Cache
                ↓
         Analytics (Goroutines)
                ↓
         Prometheus Metrics
                ↓
         Grafana Dashboards
```

## Технологический стек

- **Go 1.22+** - основной язык разработки
- **Redis 7** - кэширование метрик
- **Kubernetes** - оркестрация контейнеров
- **Prometheus** - сбор метрик
- **Grafana** - визуализация
- **HPA** - автомасштабирование

## Быстрый старт

### Предварительные требования

- Go 1.22+
- Docker
- Kubernetes (Minikube/Kind) или доступ к облачному кластеру
- kubectl

### Локальная разработка

```bash
# Клонирование репозитория
git clone <repository-url>
cd iot

# Установка зависимостей
go mod download

# Запуск локально (требуется Redis)
docker-compose up -d redis
go run main.go
```

### Развертывание в Kubernetes

1. **Установите зависимости:**
   - Metrics Server (для HPA): `minikube addons enable metrics-server`
   - NGINX Ingress Controller: `minikube addons enable ingress`

2. **Соберите и загрузите образ:**
   ```bash
   make docker-build
   minikube image load go-microservice:latest
   ```

3. **Разверните сервис:**
   ```bash
   make k8s-deploy
   ```

4. **Проверьте статус:**
   ```bash
   kubectl get pods
   kubectl get services
   kubectl get hpa
   ```

## API Endpoints

### POST /api/metrics
Прием метрик от IoT-устройств.

**Request:**
```json
{
  "timestamp": 1234567890,
  "device_id": "device_001",
  "cpu": 65.5,
  "rps": 250.0,
  "memory": 55.0
}
```

**Response:**
```json
{
  "status": "accepted",
  "message": "Metric received and queued for processing"
}
```

### GET /api/analyze?device_id={device_id}
Получение аналитики для устройства.

**Response:**
```json
{
  "device_id": "device_001",
  "rolling_average": 62.3,
  "window_size": 50
}
```

### GET /api/anomalies
Список обнаруженных аномалий.

**Response:**
```json
{
  "count": 5,
  "anomalies": [
    {
      "device_id": "device_001",
      "rolling_average": 62.3,
      "z_score": 2.5,
      "is_anomaly": true,
      "timestamp": 1234567890,
      "value": 95.0
    }
  ]
}
```

### GET /health
Проверка здоровья сервиса.

### GET /metrics
Prometheus метрики.

## Нагрузочное тестирование

```bash
# Запуск теста (требуется Python 3)
python3 tests/load_test.py

# Простой тест API
bash tests/simple_test.sh
```

## Мониторинг

### Prometheus

```bash
kubectl port-forward service/prometheus 9090:9090
# Откройте http://localhost:9090
```

### Grafana

```bash
kubectl port-forward service/grafana 3000:3000
# Откройте http://localhost:3000
# Логин: admin / Пароль: admin
```

#### Дашборды Grafana

При первом запуске Grafana автоматически импортирует следующие дашборды из ConfigMap `grafana-dashboards`:

1. **RPS Dashboard** (`rps-dashboard`)
   - **Current RPS** - текущее значение запросов в секунду (метрика `highload_current_rps`)
   - **Total Requests Rate** - скорость запросов по эндпоинтам (метрика `rate(highload_requests_total[5m])`)

2. **Latency Dashboard** (`latency-dashboard`)
   - **Request Duration (95th percentile)** - 95-й процентиль задержки запросов
   - **Request Duration (50th percentile)** - медианная задержка запросов
   - **Average Request Duration** - средняя задержка запросов

3. **Anomalies Dashboard** (`anomalies-dashboard`)
   - **Anomalies Detected Rate** - скорость обнаружения аномалий (метрика `rate(highload_anomalies_detected_total[5m])`)
   - **Total Anomalies** - общее количество обнаруженных аномалий (метрика `highload_anomalies_detected_total`)
   - **Metrics Processed Rate** - скорость обработки метрик (метрика `rate(highload_metrics_processed_total[5m])`)

Все дашборды используют Prometheus как источник данных и автоматически обновляются каждые 10 секунд.

## Автомасштабирование (HPA)

HPA настроен на:
- **minReplicas:** 2
- **maxReplicas:** 5
- **CPU threshold:** 70%
- **Memory threshold:** 80%

Проверка:
```bash
kubectl get hpa go-microservice-hpa
kubectl describe hpa go-microservice-hpa
```

## Структура проекта

```
.
├── main.go                 # Основной код сервиса
├── Dockerfile              # Docker образ
├── docker-compose.yml      # Локальная разработка
├── go.mod                  # Go зависимости
├── k8s/                    # Kubernetes манифесты
│   ├── deployment.yaml     # Deployment Go сервиса
│   ├── service.yaml        # Service
│   ├── hpa.yaml           # Horizontal Pod Autoscaler
│   ├── ingress.yaml       # Ingress
│   ├── redis-deployment.yaml
│   ├── prometheus-deployment.yaml
│   ├── prometheus-config.yaml
│   ├── grafana-deployment.yaml
│   └── grafana-dashboards.yaml  # Дашборды Grafana
├── scripts/                # Скрипты развертывания
├── tests/                  # Тесты
└── README.md              # Этот файл
```

## Разработка

### Добавление новых метрик

Метрики Prometheus определены в `main.go`:
- `highload_requests_total` - общее количество запросов
- `highload_request_duration_seconds` - длительность запросов
- `highload_anomalies_detected_total` - обнаруженные аномалии
- `highload_metrics_processed_total` - обработанные метрики
- `highload_current_rps` - текущий RPS

### Тестирование

```bash
# Unit тесты
go test -v ./...

# Интеграционные тесты
bash tests/simple_test.sh

# Нагрузочное тестирование
python3 tests/load_test.py
```

## Производительность

- **Целевой RPS:** 1000+
- **Latency (p95):** < 50ms
- **Точность детекции аномалий:** > 70%
- **False positive rate:** < 10%

## Лицензия

MIT

## Автор

Проект разработан в рамках учебного задания.
