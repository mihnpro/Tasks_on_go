BINARY := cache-comparison
PKG    := ./cmd/...

.PHONY: run down build tidy

## run: собрать образ и запустить бенчмарк (отчёт появится в ./reports/)
run:
	docker compose up --build --abort-on-container-exit --exit-code-from app
	docker compose down

## down: остановить и удалить контейнеры и тома
down:
	docker compose down -v

## build: собрать бинарник локально
build:
	go build -o $(BINARY) $(PKG)

## tidy: подтянуть зависимости
tidy:
	go mod tidy
