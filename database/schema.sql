-- Таблица ресторанов (с image_url сразу)
CREATE TABLE IF NOT EXISTS restaurants (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    address TEXT,
    description TEXT,
    image_url TEXT,  -- Сразу добавили поле для фото
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Таблица блюд (с каскадным удалением сразу)
CREATE TABLE IF NOT EXISTS dishes (
    id SERIAL PRIMARY KEY,
    restaurant_id INTEGER REFERENCES restaurants(id) ON DELETE CASCADE,  -- Каскад сразу прописан
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10, 2),
    image_url TEXT,
    avg_rating DECIMAL(3,2) DEFAULT 0,
    review_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Таблица заказов
CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    restaurant_id INTEGER REFERENCES restaurants(id) ON DELETE CASCADE,
    total_amount DECIMAL(10, 2),
    status VARCHAR(50) DEFAULT 'pending',
    qr_code BYTEA,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Таблица позиций заказов
CREATE TABLE IF NOT EXISTS order_items (
    id SERIAL PRIMARY KEY,
    order_id INTEGER REFERENCES orders(id) ON DELETE CASCADE,
    dish_id INTEGER REFERENCES dishes(id) ON DELETE CASCADE,
    quantity INTEGER NOT NULL DEFAULT 1,
    price DECIMAL(10, 2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Таблица отзывов
CREATE TABLE IF NOT EXISTS reviews (
    id SERIAL PRIMARY KEY,
    dish_id INTEGER REFERENCES dishes(id) ON DELETE CASCADE,
    order_id INTEGER REFERENCES orders(id) ON DELETE CASCADE,
    restaurant_id INTEGER REFERENCES restaurants(id) ON DELETE CASCADE,
    rating INTEGER CHECK (rating >= 1 AND rating <= 5),
    comment TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_review_per_order UNIQUE (dish_id, order_id)
);

-- Тестовые данные: рестораны
INSERT INTO restaurants (name, address, description) VALUES
    ('Чайхана "Самарканд"', 'ул. Пушкина, д. 10', 'Традиционная узбекская кухня'),
    ('Суши-бар "Токио"', 'ул. Ленина, д. 25', 'Автентичная японская кухня'),
    ('Пиццерия "Наполетана"', 'ул. Гагарина, д. 5', 'Итальянская пицца по традиционным рецептам')
ON CONFLICT DO NOTHING;

-- Тестовые данные: блюда
INSERT INTO dishes (restaurant_id, name, description, price) VALUES
    (1, 'Плов', 'Традиционный узбекский плов', 350.00),
    (1, 'Шашлык', 'Свиной шашлык на углях', 280.00),
    (1, 'Лагман', 'Домашняя лапша с мясом', 320.00),
    (2, 'Ролл Филадельфия', 'Классический ролл с лососем', 420.00),
    (2, 'Суши с тунцом', 'Традиционные суши', 180.00),
    (2, 'Суп Мисо', 'Традиционный японский суп', 120.00),
    (3, 'Маргарита', 'Традиционная итальянская пицца', 450.00),
    (3, 'Пепперони', 'Пицца с пепперони и сыром', 520.00),
    (3, 'Четыре сезона', 'Пицца с 4 видами сыра', 580.00)
ON CONFLICT DO NOTHING;

-- ТЕСТОВЫЕ ЧЕКА (3 штуки)
INSERT INTO orders (restaurant_id, total_amount, status) VALUES
    (1, 950.00, 'completed'),
    (2, 720.00, 'completed'),
    (3, 1550.00, 'completed');

-- Элементы для чека #1 (Самарканд: 2xПлов + 1xШашлык = 980)
INSERT INTO order_items (order_id, dish_id, quantity, price) VALUES
    (1, 1, 2, 350.00),
    (1, 2, 1, 280.00);

-- Элементы для чека #2 (Токио: 1xФиладельфия + 2xМисо = 660)
INSERT INTO order_items (order_id, dish_id, quantity, price) VALUES
    (2, 4, 1, 420.00),
    (2, 6, 2, 120.00);

-- Элементы для чека #3 (Наполетана: 2xМаргарита + 1xПепперони = 1420)
INSERT INTO order_items (order_id, dish_id, quantity, price) VALUES
    (3, 7, 2, 450.00),
    (3, 8, 1, 520.00);