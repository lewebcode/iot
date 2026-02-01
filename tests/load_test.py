#!/usr/bin/env python3
"""
Скрипт нагрузочного тестирования для go-microservice
Использует локальные HTTP запросы для симуляции нагрузки
"""

import json
import time
import random
import requests
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime
import sys

# Конфигурация
BASE_URL = "http://localhost:8080"
DEVICE_IDS = [f"device_{i:03d}" for i in range(1, 21)]  # 20 устройств
DURATION_SECONDS = 300  # 5 минут
TARGET_RPS = 1000
NUM_THREADS = 50

# Статистика
stats = {
    "total_requests": 0,
    "successful_requests": 0,
    "failed_requests": 0,
    "total_latency": 0,
    "min_latency": float('inf'),
    "max_latency": 0,
    "start_time": None,
    "end_time": None,
}


def generate_metric():
    """Генерирует случайную метрику с возможностью аномалий"""
    device_id = random.choice(DEVICE_IDS)
    
    # С вероятностью 5% генерируем аномалию
    is_anomaly = random.random() < 0.05
    
    if is_anomaly:
        cpu = random.uniform(85, 99)  # Аномально высокая нагрузка
    else:
        cpu = random.uniform(20, 70)  # Нормальная нагрузка
    
    return {
        "timestamp": int(time.time()),
        "device_id": device_id,
        "cpu": cpu,
        "rps": random.uniform(100, 500),
        "memory": random.uniform(40, 80)
    }


def send_metric():
    """Отправляет одну метрику на сервер"""
    metric = generate_metric()
    
    try:
        start_time = time.time()
        response = requests.post(
            f"{BASE_URL}/api/metrics",
            json=metric,
            timeout=5
        )
        latency = (time.time() - start_time) * 1000  # в миллисекундах
        
        stats["total_requests"] += 1
        
        if response.status_code in [200, 202]:
            stats["successful_requests"] += 1
            stats["total_latency"] += latency
            stats["min_latency"] = min(stats["min_latency"], latency)
            stats["max_latency"] = max(stats["max_latency"], latency)
            return True, latency
        else:
            stats["failed_requests"] += 1
            return False, latency
            
    except Exception as e:
        stats["failed_requests"] += 1
        return False, 0


def check_health():
    """Проверяет здоровье сервиса"""
    try:
        response = requests.get(f"{BASE_URL}/health", timeout=5)
        if response.status_code == 200:
            print(f"✓ Сервис доступен: {response.json()}")
            return True
        else:
            print(f"✗ Сервис вернул код {response.status_code}")
            return False
    except Exception as e:
        print(f"✗ Не удалось подключиться к сервису: {e}")
        return False


def run_load_test():
    """Выполняет нагрузочное тестирование"""
    print("=" * 80)
    print("НАГРУЗОЧНОЕ ТЕСТИРОВАНИЕ go-microservice")
    print("=" * 80)
    print(f"URL: {BASE_URL}")
    print(f"Целевой RPS: {TARGET_RPS}")
    print(f"Длительность: {DURATION_SECONDS} секунд")
    print(f"Количество потоков: {NUM_THREADS}")
    print(f"Устройств: {len(DEVICE_IDS)}")
    print("=" * 80)
    
    # Проверка доступности
    if not check_health():
        print("\n✗ Сервис недоступен. Проверьте, что он запущен.")
        sys.exit(1)
    
    print("\nЗапуск теста...\n")
    
    stats["start_time"] = time.time()
    
    # Запускаем нагрузочное тестирование
    with ThreadPoolExecutor(max_workers=NUM_THREADS) as executor:
        futures = []
        request_count = 0
        target_requests = TARGET_RPS * DURATION_SECONDS
        
        interval = 1.0 / TARGET_RPS  # Интервал между запросами
        next_request_time = time.time()
        
        while time.time() - stats["start_time"] < DURATION_SECONDS:
            current_time = time.time()
            
            if current_time >= next_request_time:
                future = executor.submit(send_metric)
                futures.append(future)
                request_count += 1
                next_request_time += interval
                
                # Вывод прогресса каждые 100 запросов
                if request_count % 100 == 0:
                    elapsed = time.time() - stats["start_time"]
                    current_rps = stats["total_requests"] / elapsed if elapsed > 0 else 0
                    print(f"Прогресс: {request_count}/{target_requests} запросов, "
                          f"Текущий RPS: {current_rps:.1f}, "
                          f"Успешных: {stats['successful_requests']}, "
                          f"Ошибок: {stats['failed_requests']}")
            
            # Небольшая задержка для точности
            time.sleep(0.0001)
        
        # Ждем завершения всех запросов
        print("\nОжидание завершения всех запросов...")
        for future in as_completed(futures):
            pass
    
    stats["end_time"] = time.time()
    
    # Получаем статистику аномалий
    try:
        response = requests.get(f"{BASE_URL}/api/anomalies", timeout=5)
        anomalies_count = 0
        if response.status_code == 200:
            anomalies_data = response.json()
            anomalies_count = anomalies_data.get("count", 0)
    except:
        anomalies_count = 0
    
    # Выводим результаты
    print("\n" + "=" * 80)
    print("РЕЗУЛЬТАТЫ ТЕСТИРОВАНИЯ")
    print("=" * 80)
    
    total_time = stats["end_time"] - stats["start_time"]
    avg_latency = stats["total_latency"] / stats["successful_requests"] if stats["successful_requests"] > 0 else 0
    actual_rps = stats["total_requests"] / total_time if total_time > 0 else 0
    success_rate = (stats["successful_requests"] / stats["total_requests"] * 100) if stats["total_requests"] > 0 else 0
    
    print(f"Общее время: {total_time:.2f} секунд")
    print(f"Всего запросов: {stats['total_requests']}")
    print(f"Успешных запросов: {stats['successful_requests']}")
    print(f"Неудачных запросов: {stats['failed_requests']}")
    print(f"Процент успеха: {success_rate:.2f}%")
    print(f"Фактический RPS: {actual_rps:.2f}")
    print(f"\nЛатентность:")
    print(f"  Средняя: {avg_latency:.2f} мс")
    print(f"  Минимальная: {stats['min_latency']:.2f} мс")
    print(f"  Максимальная: {stats['max_latency']:.2f} мс")
    print(f"\nОбнаружено аномалий: {anomalies_count}")
    
    print("=" * 80)
    
    # Оценка результатов
    print("\nОЦЕНКА:")
    if actual_rps >= TARGET_RPS * 0.9 and success_rate >= 99:
        print("✓ ОТЛИЧНО: Сервис справился с нагрузкой!")
    elif actual_rps >= TARGET_RPS * 0.7 and success_rate >= 95:
        print("✓ ХОРОШО: Сервис работает стабильно")
    elif actual_rps >= TARGET_RPS * 0.5 and success_rate >= 90:
        print("~ УДОВЛЕТВОРИТЕЛЬНО: Есть проблемы с производительностью")
    else:
        print("✗ ПЛОХО: Сервис не справляется с нагрузкой")
    
    if avg_latency < 50:
        print("✓ Отличная латентность (< 50 мс)")
    elif avg_latency < 100:
        print("✓ Хорошая латентность (< 100 мс)")
    elif avg_latency < 200:
        print("~ Приемлемая латентность (< 200 мс)")
    else:
        print("✗ Высокая латентность (>= 200 мс)")
    
    print("=" * 80)


if __name__ == "__main__":
    try:
        run_load_test()
    except KeyboardInterrupt:
        print("\n\nТестирование прервано пользователем")
        sys.exit(0)

