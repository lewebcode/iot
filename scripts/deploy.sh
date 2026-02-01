#!/bin/bash

# Скрипт для развертывания в Kubernetes

set -e

echo "=========================================="
echo "Развертывание go-microservice в Kubernetes"
echo "=========================================="

# Проверка наличия kubectl
if ! command -v kubectl &> /dev/null; then
    echo "Ошибка: kubectl не установлен"
    exit 1
fi

# Проверка подключения к кластеру
if ! kubectl cluster-info &> /dev/null; then
    echo "Ошибка: Нет подключения к Kubernetes кластеру"
    echo "Запустите Minikube: minikube start --cpus=2 --memory=4g"
    exit 1
fi

echo "1. Развертывание Redis..."
kubectl apply -f k8s/redis-deployment.yaml

echo "2. Развертывание основного сервиса..."
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml

echo "3. Настройка автомасштабирования..."
kubectl apply -f k8s/hpa.yaml

echo "4. Развертывание Prometheus..."
kubectl apply -f k8s/prometheus-config.yaml
kubectl apply -f k8s/prometheus-deployment.yaml

echo "5. Развертывание Grafana..."
kubectl apply -f k8s/grafana-dashboards.yaml
kubectl apply -f k8s/grafana-deployment.yaml

echo "6. Настройка Ingress..."
kubectl apply -f k8s/ingress.yaml

echo ""
echo "=========================================="
echo "Ожидание запуска подов..."
echo "=========================================="

kubectl wait --for=condition=ready pod -l app=redis --timeout=120s
kubectl wait --for=condition=ready pod -l app=go-microservice --timeout=120s
kubectl wait --for=condition=ready pod -l app=prometheus --timeout=120s
kubectl wait --for=condition=ready pod -l app=grafana --timeout=120s

echo ""
echo "=========================================="
echo "Развертывание завершено!"
echo "=========================================="
echo ""
echo "Статус подов:"
kubectl get pods

echo ""
echo "Сервисы:"
kubectl get svc

echo ""
echo "HPA:"
kubectl get hpa

echo ""
echo "=========================================="
echo "Доступ к сервисам:"
echo "=========================================="
echo "Highload Service: kubectl port-forward svc/go-microservice 8080:80"
echo "Prometheus: kubectl port-forward svc/prometheus 9090:9090"
echo "Grafana: kubectl port-forward svc/grafana 3000:3000"
echo ""
echo "Или используйте Minikube service:"
echo "minikube service go-microservice"
echo "=========================================="

