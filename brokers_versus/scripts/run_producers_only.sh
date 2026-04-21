#!/bin/bash

# Запуск producer для всех комбинаций размеров и интенсивностей
# Использование: ./run_producers_only.sh <rabbitmq|redis>

set -e

BROKER=$1
if [ -z "$BROKER" ]; then
    echo "Укажите брокер: rabbitmq или redis"
    exit 1
fi

# URI для брокеров
if [ "$BROKER" = "rabbitmq" ]; then
    URI="amqp://guest:guest@localhost:5672/"
elif [ "$BROKER" = "redis" ]; then
    URI="redis://localhost:6379/0"
else
    echo "Неизвестный брокер: $BROKER"
    exit 1
fi

SIZES=(128 1024 10240 102400)
RATES=(1000 5000 10000)
DURATION="30s"

mkdir -p results

for SIZE in "${SIZES[@]}"; do
    for RATE in "${RATES[@]}"; do
        QUEUE="test_queue_${SIZE}_${RATE}"
        METRICS_FILE="results/${BROKER}_size${SIZE}_rate${RATE}_producer.json"
        
        echo "=============================================="
        echo "Запуск producer: Брокер=$BROKER, Размер=$SIZE байт, Интенсивность=$RATE msg/s"
        echo "Очередь: $QUEUE"
        
        ./bin/producer \
            --broker "$BROKER" \
            --uri "$URI" \
            --queue "$QUEUE" \
            --size "$SIZE" \
            --rate "$RATE" \
            --duration "$DURATION" \
            --metrics-file "$METRICS_FILE" \
            --log-level info
        
        echo "Producer завершён. Метрики сохранены в $METRICS_FILE"
        echo ""
    done
done

echo "Все producer-тесты для $BROKER завершены."