package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metric представляет входящую метрику от IoT устройства
type Metric struct {
	Timestamp int64   `json:"timestamp"`
	DeviceID  string  `json:"device_id"`
	CPU       float64 `json:"cpu"`
	RPS       float64 `json:"rps"`
	Memory    float64 `json:"memory"`
}

// AnalyticsResult представляет результат анализа
type AnalyticsResult struct {
	DeviceID       string  `json:"device_id"`
	RollingAverage float64 `json:"rolling_average"`
	ZScore         float64 `json:"z_score"`
	IsAnomaly      bool    `json:"is_anomaly"`
	Timestamp      int64   `json:"timestamp"`
	Value          float64 `json:"value"`
}

// MetricsBuffer хранит метрики для анализа
type MetricsBuffer struct {
	mu      sync.RWMutex
	data    map[string][]float64
	window  int
	maxSize int
}

func NewMetricsBuffer(window int) *MetricsBuffer {
	return &MetricsBuffer{
		data:    make(map[string][]float64),
		window:  window,
		maxSize: 1000,
	}
}

func (mb *MetricsBuffer) Add(deviceID string, value float64) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if _, exists := mb.data[deviceID]; !exists {
		mb.data[deviceID] = make([]float64, 0, mb.maxSize)
	}

	mb.data[deviceID] = append(mb.data[deviceID], value)

	// Ограничиваем размер буфера
	if len(mb.data[deviceID]) > mb.maxSize {
		mb.data[deviceID] = mb.data[deviceID][len(mb.data[deviceID])-mb.maxSize:]
	}
}

func (mb *MetricsBuffer) GetRollingAverage(deviceID string) float64 {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	values, exists := mb.data[deviceID]
	if !exists || len(values) == 0 {
		return 0
	}

	// Вычисляем скользящее среднее по последним N значениям
	start := 0
	if len(values) > mb.window {
		start = len(values) - mb.window
	}

	sum := 0.0
	count := 0
	for i := start; i < len(values); i++ {
		sum += values[i]
		count++
	}

	if count == 0 {
		return 0
	}

	return sum / float64(count)
}

func (mb *MetricsBuffer) GetZScore(deviceID string, currentValue float64) float64 {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	values, exists := mb.data[deviceID]
	if !exists || len(values) < 2 {
		return 0
	}

	// Вычисляем среднее и стандартное отклонение
	start := 0
	if len(values) > mb.window {
		start = len(values) - mb.window
	}

	var sum float64
	count := 0
	for i := start; i < len(values); i++ {
		sum += values[i]
		count++
	}

	if count == 0 {
		return 0
	}

	mean := sum / float64(count)

	// Стандартное отклонение
	var variance float64
	for i := start; i < len(values); i++ {
		diff := values[i] - mean
		variance += diff * diff
	}
	variance /= float64(count)
	stdDev := math.Sqrt(variance)

	if stdDev == 0 {
		return 0
	}

	// Z-score
	zScore := (currentValue - mean) / stdDev
	return zScore
}

// Service представляет основной сервис
type Service struct {
	redis          *redis.Client
	metricsBuffer  *MetricsBuffer
	ctx            context.Context
	anomalyChannel chan AnalyticsResult
}

// Prometheus метрики
var (
	requestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "highload_requests_total",
			Help: "Total number of requests",
		},
		[]string{"endpoint"},
	)

	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "highload_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint"},
	)

	anomaliesDetected = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "highload_anomalies_detected_total",
			Help: "Total number of anomalies detected",
		},
	)

	metricsProcessed = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "highload_metrics_processed_total",
			Help: "Total number of metrics processed",
		},
	)

	currentRPS = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "highload_current_rps",
			Help: "Current RPS value",
		},
	)
)

func NewService(redisAddr string) *Service {
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       0,
	})

	ctx := context.Background()

	// Проверка подключения к Redis
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Printf("Warning: Redis connection failed: %v. Continuing without Redis.", err)
	} else {
		log.Println("Successfully connected to Redis")
	}

	return &Service{
		redis:          rdb,
		metricsBuffer:  NewMetricsBuffer(50),
		ctx:            ctx,
		anomalyChannel: make(chan AnalyticsResult, 100),
	}
}

// MetricsHandler обрабатывает входящие метрики
func (s *Service) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(requestDuration.WithLabelValues("/metrics"))
	defer timer.ObserveDuration()
	requestsTotal.WithLabelValues("/metrics").Inc()

	var metric Metric
	if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Валидация
	if metric.DeviceID == "" {
		http.Error(w, "device_id is required", http.StatusBadRequest)
		return
	}

	// Добавляем метрику в буфер
	s.metricsBuffer.Add(metric.DeviceID, metric.CPU)

	// Обновляем Prometheus метрики
	metricsProcessed.Inc()
	currentRPS.Set(metric.RPS)

	// Кэшируем в Redis
	go s.cacheMetric(metric)

	// Анализируем в отдельной горутине
	go s.analyzeMetric(metric)

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "accepted",
		"message": "Metric received and queued for processing",
	})
}

func (s *Service) cacheMetric(metric Metric) {
	key := fmt.Sprintf("metric:%s:%d", metric.DeviceID, metric.Timestamp)
	data, _ := json.Marshal(metric)
	s.redis.Set(s.ctx, key, data, 10*time.Minute)
}

func (s *Service) analyzeMetric(metric Metric) {
	rollingAvg := s.metricsBuffer.GetRollingAverage(metric.DeviceID)
	zScore := s.metricsBuffer.GetZScore(metric.DeviceID, metric.CPU)

	// Порог для аномалий: |z-score| > 2
	isAnomaly := math.Abs(zScore) > 2.0

	if isAnomaly {
		anomaliesDetected.Inc()
		log.Printf("Anomaly detected! Device: %s, CPU: %.2f, Z-Score: %.2f",
			metric.DeviceID, metric.CPU, zScore)
	}

	result := AnalyticsResult{
		DeviceID:       metric.DeviceID,
		RollingAverage: rollingAvg,
		ZScore:         zScore,
		IsAnomaly:      isAnomaly,
		Timestamp:      metric.Timestamp,
		Value:          metric.CPU,
	}

	// Отправляем результат в канал
	select {
	case s.anomalyChannel <- result:
	default:
		// Канал заполнен, пропускаем
	}
}

// AnalyzeHandler возвращает результаты анализа для устройства
func (s *Service) AnalyzeHandler(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(requestDuration.WithLabelValues("/analyze"))
	defer timer.ObserveDuration()
	requestsTotal.WithLabelValues("/analyze").Inc()

	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		http.Error(w, "device_id parameter is required", http.StatusBadRequest)
		return
	}

	rollingAvg := s.metricsBuffer.GetRollingAverage(deviceID)

	response := map[string]interface{}{
		"device_id":       deviceID,
		"rolling_average": rollingAvg,
		"window_size":     50,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HealthHandler проверка здоровья сервиса
func (s *Service) HealthHandler(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status": "healthy",
		"time":   time.Now().Unix(),
	}

	// Проверяем Redis
	_, err := s.redis.Ping(s.ctx).Result()
	if err != nil {
		health["redis"] = "disconnected"
		health["status"] = "degraded"
	} else {
		health["redis"] = "connected"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// AnomaliesHandler возвращает список обнаруженных аномалий
func (s *Service) AnomaliesHandler(w http.ResponseWriter, r *http.Request) {
	requestsTotal.WithLabelValues("/anomalies").Inc()

	anomalies := make([]AnalyticsResult, 0)
	timeout := time.After(100 * time.Millisecond)

	// Собираем аномалии из канала
drainLoop:
	for {
		select {
		case anomaly := <-s.anomalyChannel:
			anomalies = append(anomalies, anomaly)
		case <-timeout:
			break drainLoop
		default:
			break drainLoop
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count":     len(anomalies),
		"anomalies": anomalies,
	})
}

func main() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	service := NewService(redisAddr)

	r := mux.NewRouter()

	// API endpoints
	r.HandleFunc("/api/metrics", service.MetricsHandler).Methods("POST")
	r.HandleFunc("/api/analyze", service.AnalyzeHandler).Methods("GET")
	r.HandleFunc("/api/anomalies", service.AnomaliesHandler).Methods("GET")
	r.HandleFunc("/health", service.HealthHandler).Methods("GET")

	// Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// Простая главная страница
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Highload Service with AI Analytics - Running"))
	}).Methods("GET")

	log.Printf("Starting server on port %s...", port)
	log.Printf("Endpoints: /api/metrics (POST), /api/analyze (GET), /api/anomalies (GET), /health (GET), /metrics (Prometheus)")

	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
