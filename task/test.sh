#!/bin/bash
set -euo pipefail

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Конфигурация
API_URL="http://localhost:8085"
DB_CONTAINER=$(docker ps -qf "name=db" | head -n1)
if [ -z "$DB_CONTAINER" ]; then
    echo -e "${RED}Контейнер с БД не найден. Убедитесь, что docker-compose запущен.${NC}"
    exit 1
fi

# Функция для выполнения SQL через docker exec
db_exec() {
    docker exec -i "$DB_CONTAINER" psql -U postgres -d store -t -A "$@"
}

# Очистка базы данных перед тестами
cleanup_db() {
    echo -e "${YELLOW}Очистка базы данных...${NC}"
    db_exec -c "TRUNCATE order_items, orders, products, customers RESTART IDENTITY CASCADE;"
}

# Подготовка тестовых данных
prepare_data() {
    echo -e "${YELLOW}Подготовка тестовых данных...${NC}"
    db_exec <<EOF
INSERT INTO customers (first_name, last_name, email) VALUES
('Иван', 'Петров', 'ivan@example.com'),
('Мария', 'Сидорова', 'maria@example.com');
INSERT INTO products (name, price) VALUES
('Ноутбук', 1200.00),
('Мышь', 25.50),
('Клавиатура', 75.00);
EOF
}

# Проверка наличия jq
if ! command -v jq &> /dev/null; then
    echo -e "${RED}jq не установлен. Установите jq для парсинга JSON.${NC}"
    exit 1
fi

# Функция-обёртка для curl с проверкой статуса
curl_test() {
    local method="$1"
    local url="$2"
    local data="$3"
    local expected_code="$4"
    local description="$5"

    echo -n "▶ $description... "
    response=$(curl -s -X "$method" "$API_URL$url" \
        -H "Content-Type: application/json" \
        -d "$data" \
        -w "\n%{http_code}")
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    if [ "$http_code" -eq "$expected_code" ]; then
        echo -e "${GREEN}OK${NC} (код $http_code)"
        # Возвращаем тело ответа для дальнейшего использования
        echo "$body"
        return 0
    else
        echo -e "${RED}ОШИБКА${NC} ожидался код $expected_code, получен $http_code"
        echo "Ответ: $body"
        return 1
    fi
}

# Функция для проверки значения в JSON по пути
assert_json() {
    local json="$1"
    local path="$2"
    local expected="$3"
    local description="$4"

    actual=$(echo "$json" | jq -r "$path")
    if [ "$actual" = "$expected" ]; then
        echo -e "  ${GREEN}✓${NC} $description: $actual"
    else
        echo -e "  ${RED}✗${NC} $description: ожидалось '$expected', получено '$actual'"
        return 1
    fi
}

# ============================================
# НАЧАЛО ТЕСТОВ
# ============================================
echo -e "${YELLOW}=== ЗАПУСК ТЕСТОВ API ИНТЕРНЕТ-МАГАЗИНА ===${NC}"

# Подготовка окружения
cleanup_db
prepare_data

TESTS_FAILED=0

# --------------------------------------------
# СЦЕНАРИЙ 3: Добавление продукта
# --------------------------------------------
echo -e "\n${YELLOW}--- Сценарий 3: Добавление продукта ---${NC}"

# Успешное создание
body=$(curl_test "POST" "/products" '{"name": "Монитор", "price": 299.99}' 201 "Создание продукта 'Монитор'")
if [ $? -eq 0 ]; then
    assert_json "$body" ".name" "Монитор" "Имя продукта"
    assert_json "$body" ".price" "299.99" "Цена продукта"
    product_id=$(echo "$body" | jq -r ".id")
    # Проверяем, что запись появилась в БД
    db_count=$(db_exec -c "SELECT COUNT(*) FROM products WHERE id = $product_id;")
    if [ "$db_count" -eq 1 ]; then
        echo -e "  ${GREEN}✓${NC} Запись найдена в БД"
    else
        echo -e "  ${RED}✗${NC} Запись не найдена в БД"
        TESTS_FAILED=$((TESTS_FAILED+1))
    fi
else
    TESTS_FAILED=$((TESTS_FAILED+1))
fi

# Ошибка: отрицательная цена
curl_test "POST" "/products" '{"name": "Брак", "price": -10}' 400 "Отрицательная цена" > /dev/null
[ $? -ne 0 ] && TESTS_FAILED=$((TESTS_FAILED+1))

# Ошибка: пустое имя
curl_test "POST" "/products" '{"name": "", "price": 50}' 400 "Пустое имя" > /dev/null
[ $? -ne 0 ] && TESTS_FAILED=$((TESTS_FAILED+1))

# --------------------------------------------
# СЦЕНАРИЙ 2: Обновление email клиента
# --------------------------------------------
echo -e "\n${YELLOW}--- Сценарий 2: Обновление email клиента ---${NC}"

# Успешное обновление
curl_test "PUT" "/customers/1/email" '{"email": "ivan.new@example.com"}' 200 "Обновление email клиента 1" > /dev/null
if [ $? -eq 0 ]; then
    # Проверка в БД
    actual_email=$(db_exec -c "SELECT email FROM customers WHERE id = 1;")
    if [ "$actual_email" = "ivan.new@example.com" ]; then
        echo -e "  ${GREEN}✓${NC} Email в БД обновлён"
    else
        echo -e "  ${RED}✗${NC} Email в БД не обновлён (найдено: $actual_email)"
        TESTS_FAILED=$((TESTS_FAILED+1))
    fi
else
    TESTS_FAILED=$((TESTS_FAILED+1))
fi

# Ошибка: email уже занят
curl_test "PUT" "/customers/1/email" '{"email": "maria@example.com"}' 400 "Email уже занят (maria@example.com)" > /dev/null
[ $? -ne 0 ] && TESTS_FAILED=$((TESTS_FAILED+1))

# Ошибка: несуществующий клиент
curl_test "PUT" "/customers/999/email" '{"email": "ghost@example.com"}' 400 "Несуществующий клиент" > /dev/null
[ $? -ne 0 ] && TESTS_FAILED=$((TESTS_FAILED+1))

# --------------------------------------------
# СЦЕНАРИЙ 1: Размещение заказа
# --------------------------------------------
echo -e "\n${YELLOW}--- Сценарий 1: Размещение заказа ---${NC}"

# Успешное размещение
order_data='{
  "customerId": 1,
  "items": [
    {"productId": 1, "quantity": 1},
    {"productId": 2, "quantity": 3}
  ]
}'
body=$(curl_test "POST" "/orders" "$order_data" 201 "Размещение заказа с двумя позициями")
if [ $? -eq 0 ]; then
    order_id=$(echo "$body" | jq -r ".id")
    # Проверка totalAmount в ответе
    assert_json "$body" ".totalAmount" "1276.5" "TotalAmount в ответе"
    # Проверка в БД: заказ создан
    db_total=$(db_exec -c "SELECT total_amount FROM orders WHERE id = $order_id;")
    if [ "$db_total" = "1276.5" ]; then
        echo -e "  ${GREEN}✓${NC} TotalAmount в БД корректен"
    else
        echo -e "  ${RED}✗${NC} TotalAmount в БД: $db_total, ожидалось 1276.5"
        TESTS_FAILED=$((TESTS_FAILED+1))
    fi
    # Проверка количества позиций
    items_count=$(db_exec -c "SELECT COUNT(*) FROM order_items WHERE order_id = $order_id;")
    if [ "$items_count" -eq 2 ]; then
        echo -e "  ${GREEN}✓${NC} Создано 2 позиции заказа"
    else
        echo -e "  ${RED}✗${NC} Позиций создано: $items_count, ожидалось 2"
        TESTS_FAILED=$((TESTS_FAILED+1))
    fi
else
    TESTS_FAILED=$((TESTS_FAILED+1))
fi

# Ошибка: несуществующий товар
curl_test "POST" "/orders" '{"customerId": 1, "items": [{"productId": 999, "quantity": 1}]}' 400 "Несуществующий товар" > /dev/null
[ $? -ne 0 ] && TESTS_FAILED=$((TESTS_FAILED+1))

# Проверка, что при ошибке заказ не создался
orders_count=$(db_exec -c "SELECT COUNT(*) FROM orders;")
if [ "$orders_count" -eq 1 ]; then
    echo -e "  ${GREEN}✓${NC} Транзакция откатилась: заказов в БД по-прежнему 1"
else
    echo -e "  ${RED}✗${NC} Ошибка транзакции: заказов в БД стало $orders_count"
    TESTS_FAILED=$((TESTS_FAILED+1))
fi

# Ошибка: несуществующий клиент
curl_test "POST" "/orders" '{"customerId": 999, "items": [{"productId": 1, "quantity": 1}]}' 400 "Несуществующий клиент" > /dev/null
[ $? -ne 0 ] && TESTS_FAILED=$((TESTS_FAILED+1))

# Ошибка: отрицательное количество
curl_test "POST" "/orders" '{"customerId": 1, "items": [{"productId": 1, "quantity": 0}]}' 400 "Количество <= 0" > /dev/null
[ $? -ne 0 ] && TESTS_FAILED=$((TESTS_FAILED+1))

# --------------------------------------------
# ФИНАЛЬНАЯ ОЧИСТКА
# --------------------------------------------
cleanup_db

# Итог
echo -e "\n${YELLOW}=== РЕЗУЛЬТАТЫ ТЕСТИРОВАНИЯ ===${NC}"
if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}Все тесты пройдены успешно.${NC}"
    exit 0
else
    echo -e "${RED}Обнаружено ошибок: $TESTS_FAILED.${NC}"
    exit 1
fi