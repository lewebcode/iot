.PHONY: help build run test docker-build docker-run k8s-deploy k8s-delete clean

help: ## Показать это сообщение помощи
	@echo "Доступные команды:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Собрать Go приложение
	go build -o main .

run: ## Запустить приложение локально
	go run main.go

test: ## Запустить тесты
	go test -v ./...

docker-build: ## Собрать Docker образ
	docker build -t go-microservice:latest .

docker-run: ## Запустить через Docker Compose
	docker-compose up -d

docker-stop: ## Остановить Docker Compose
	docker-compose down

k8s-deploy: ## Развернуть в Kubernetes
	bash scripts/deploy.sh

k8s-delete: ## Удалить из Kubernetes
	kubectl delete -f k8s/ --ignore-not-found=true

load-test: ## Запустить нагрузочное тестирование
	python3 tests/load_test.py

simple-test: ## Запустить простой тест API
	bash tests/simple_test.sh

clean: ## Очистить сгенерированные файлы
	rm -f main
	docker-compose down -v 2>/dev/null || true

minikube-start: ## Запустить Minikube
	minikube start --cpus=2 --memory=4g --driver=docker

minikube-stop: ## Остановить Minikube
	minikube stop

minikube-dashboard: ## Открыть Kubernetes Dashboard
	minikube dashboard

