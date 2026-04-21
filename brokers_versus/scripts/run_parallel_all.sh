#!/bin/bash

# Очистка очередей перед стартом (опционально, но рекомендуется)
docker exec rabbitmq rabbitmqctl delete_queue test_queue 2>/dev/null || true
docker exec redis redis-cli FLUSHDB 2>/dev/null || true

# Параллельный запуск всех тестов для RabbitMQ и Redis
set -e

BROKERS="rabbitmq redis"
SIZES="128 1024 10240 102400"
RATES="1000 5000 10000"
PRODUCER_DURATION="30s"
CONSUMER_DURATION="33s"

# Функция получения URI по имени брокера
get_uri() {
    case "$1" in
        rabbitmq) echo "amqp://guest:guest@localhost:5672/" ;;
        redis)    echo "redis://localhost:6379/0" ;;
        *)        echo "" ;;
    esac
}

# Функция получения числа воркеров для consumer
get_workers() {
    case "$1" in
        rabbitmq) echo "10" ;;    # RabbitMQ: один канал не потокобезопасен, используем 1 воркер
        redis)    echo "10" ;;   # Redis: можно много параллельных потребителей
        *)        echo "1" ;;
    esac
}

echo "=== Сборка бинарников ==="
go build -o bin/producer ./cmd/producer
go build -o bin/consumer ./cmd/consumer
go build -o bin/report ./cmd/report

rm -f results/*.json
mkdir -p results

pids=""

echo "=== Запуск всех тестов параллельно ==="
for BROKER in $BROKERS; do
    URI=$(get_uri "$BROKER")
    WORKERS=$(get_workers "$BROKER")
    for SIZE in $SIZES; do
        for RATE in $RATES; do
            QUEUE="test_queue_${BROKER}_${SIZE}_${RATE}"
            METRICS_PREFIX="${BROKER}_size${SIZE}_rate${RATE}"
            
            echo "[$BROKER] Размер=$SIZE B, Интенсивность=$RATE msg/s, Очередь=$QUEUE, Воркеры=$WORKERS"
            
            # Запуск consumer в фоне
            ./bin/consumer \
                --broker "$BROKER" \
                --uri "$URI" \
                --queue "$QUEUE" \
                --duration "$CONSUMER_DURATION" \
                --metrics-file "results/${METRICS_PREFIX}_consumer.json" \
                --log-level warn \
                --workers "$WORKERS" &
            consumer_pid=$!
            
            sleep 1
            
            # Запуск producer в фоне
            ./bin/producer \
                --broker "$BROKER" \
                --uri "$URI" \
                --queue "$QUEUE" \
                --size "$SIZE" \
                --rate "$RATE" \
                --duration "$PRODUCER_DURATION" \
                --metrics-file "results/${METRICS_PREFIX}_producer.json" \
                --log-level info &
            producer_pid=$!
            
            pids="$pids $consumer_pid $producer_pid"
            
            sleep 0.2
        done
    done
done

echo "=== Ожидание завершения всех тестов ==="
for pid in $pids; do
    wait $pid
done

echo "=== Генерация отчёта ==="
./bin/report --results results --output report.md

echo "=== Готово! Отчёт сохранён в report.md ==="