#!/bin/bash

# Скрипт для сборки Docker образа

set -e

echo "=========================================="
echo "Сборка Docker образа go-microservice"
echo "=========================================="

# Проверка наличия Docker
if ! command -v docker &> /dev/null; then
    echo "Ошибка: Docker не установлен"
    exit 1
fi

# Сборка образа
echo "Сборка образа..."
docker build -t go-microservice:latest .

# Проверка размера образа
IMAGE_SIZE=$(docker images go-microservice:latest --format "{{.Size}}")
echo "Размер образа: $IMAGE_SIZE"

echo "=========================================="
echo "Образ успешно собран: go-microservice:latest"
echo "=========================================="

# Для Minikube - загрузка образа
if command -v minikube &> /dev/null; then
    echo "Обнаружен Minikube. Загрузка образа в Minikube..."
    minikube image load go-microservice:latest
    echo "Образ загружен в Minikube"
fi

