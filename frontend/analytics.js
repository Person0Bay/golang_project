// API URL
const API_URL = 'http://localhost:8080';

function ensureArray(value) {
    return Array.isArray(value) ? value : [];
}

// Загрузка аналитики при загрузке страницы
document.addEventListener('DOMContentLoaded', function() {
    loadAnalytics();
});

// Глобальная переменная для хранения имен кафе
let cafesMap = {};

// Загрузка всех аналитических данных
async function loadAnalytics() {
    try {
        // 1. Сначала загружаем словарь имен кафе
        await loadCafesMap();

        // 2. Затем параллельно загружаем данные, передавая туда словарь
        await Promise.all([
            loadTopToday(),
            loadTopAllTime(),
            loadStats()
        ]);

        createRatingsChart();
    } catch (error) {
        console.error('Ошибка при загрузке аналитики:', error);
        showNotification('Ошибка при загрузке данных', 'error');
    }
}

// Функция создания словаря имен кафе {id: "Название"}
async function loadCafesMap() {
    try {
        const response = await fetch(`${API_URL}/api/cafes`);
        if (response.ok) {
            const cafes = await response.json();
            const data = ensureArray(cafes);
            cafesMap = {}; // Очищаем
            data.forEach(cafe => {
                cafesMap[cafe.id] = cafe.name;
            });
        }
    } catch (error) {
        console.error("Не удалось загрузить список кафе:", error);
    }
}

// Загрузка топ блюд за сегодня
async function loadTopToday() {
    try {
        const response = await fetch(`${API_URL}/api/analytics/top-today`);

        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }

        const raw = await response.json();
        const data = ensureArray(raw);

        document.getElementById('top-today-loading').classList.add('hidden');
        document.getElementById('top-today-content').classList.remove('hidden');

        // Передаем label "Активность", так как тут счетчик действий
        displayTopList('top-today-list', data, 'Отзывов');
    } catch (error) {
        console.error('Ошибка при загрузке топа за сегодня:', error);
        document.getElementById('top-today-loading').innerHTML =
            '<p class="text-red-500">Нет данных для отображения</p>';
    }
}

// Загрузка топ блюд за все время
async function loadTopAllTime() {
    try {
        const response = await fetch(`${API_URL}/api/analytics/top-alltime`);

        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }

        const raw = await response.json();
        const data = ensureArray(raw);

        document.getElementById('top-alltime-loading').classList.add('hidden');
        document.getElementById('top-alltime-content').classList.remove('hidden');

        // Передаем label "Рейтинг", так как тут средняя оценка
        displayTopList('top-alltime-list', data, 'Рейтинг');
    } catch (error) {
        console.error('Ошибка при загрузке топа за все время:', error);
        document.getElementById('top-alltime-loading').innerHTML =
            '<p class="text-red-500">Нет данных для отображения</p>';
    }
}

// Отображение списка топ блюд
function displayTopList(containerId, data, scoreLabel) {
    const container = document.getElementById(containerId);
    container.innerHTML = '';

    if (!data || data.length === 0) {
        container.innerHTML = '<p class="text-gray-500 text-center py-4">Нет данных для отображения</p>';
        return;
    }

    data.forEach((item, index) => {
        const listItem = createTopListItem(item, index + 1, scoreLabel);
        container.appendChild(listItem);
    });
}

// Создание элемента списка топ блюд
// analytics.js -> createTopListItem
function createTopListItem(item, position, scoreLabel) {
    const div = document.createElement('div');
    div.className = 'flex items-center justify-between p-4 bg-gray-50 rounded-lg border border-gray-100 hover:shadow-sm transition';

    const medalColor = position === 1 ? 'text-yellow-500' :
                      position === 2 ? 'text-gray-400' :
                      position === 3 ? 'text-orange-600' : 'text-gray-300';

    const restId = String(item.restaurant_id);
    const cafeName = cafesMap[restId] || `Кафе #${item.restaurant_id}`;

    let displayScore = item.score || 0;
    if (scoreLabel === 'Рейтинг') {
        displayScore = parseFloat(displayScore).toFixed(1);
    } else {
        displayScore = Math.round(displayScore);
    }

    // Логика скрытия отзывов: показываем их только если это НЕ "Заказов"
    const showReviews = scoreLabel !== 'Заказов';

    div.innerHTML = `
        <div class="flex items-center space-x-4">
            <span class="text-2xl font-bold ${medalColor} w-8 text-center">${position}</span>
            <div>
                <h4 class="font-semibold text-gray-900">${item.dish_name || 'Неизвестное блюдо'}</h4>
                <div class="flex items-center text-sm text-gray-600 mt-1">
                    <i class="fas fa-store mr-1 text-gray-400"></i>
                    <span>${cafeName}</span>
                </div>
            </div>
        </div>
        <div class="text-right">
            <p class="font-bold text-orange-500 text-lg">${displayScore} <span class="text-xs font-normal text-gray-500 uppercase">${scoreLabel}</span></p>
            ${showReviews ? `
            <p class="text-xs text-gray-500 mt-1">
                Отзывов : <span class="font-semibold">${item.review_count || 0}</span>
            </p>` : ''}
        </div>
    `;

    return div;
}

// Загрузка статистики
async function loadStats() {
    try {
        const [cafesResponse, reviewsResponse] = await Promise.all([
            fetch(`${API_URL}/api/cafes`),
            fetch(`${API_URL}/api/analytics/top-alltime`),
        ]);

        let totalReviews = 0;
        let avgRating = 0;
        let reviewedDishes = 0;
        let activeCafes = 0;

        if (cafesResponse.ok) {
            const rawCafes = await cafesResponse.json();
            const cafes = ensureArray(rawCafes);
            activeCafes = cafes.length || 0;
        }

        if (reviewsResponse.ok) {
            const rawTop = await reviewsResponse.json();
            const topDishes = ensureArray(rawTop);
            reviewedDishes = topDishes.length || 0;
            if (reviewedDishes > 0) {
                totalReviews = topDishes.reduce((sum, dish) => sum + (dish.review_count || 0), 0);
                avgRating = topDishes.reduce((sum, dish) => sum + (dish.score || 0), 0) / reviewedDishes;
            }
        }

        // Обновляем UI
        const animateValue = (id, value, isFloat = false) => {
            const el = document.getElementById(id);
            if(el) el.textContent = isFloat ? value.toFixed(1) : value;
        };

        animateValue('active-cafes', activeCafes);
        animateValue('total-reviews', totalReviews);
        animateValue('reviewed-dishes', reviewedDishes);
        animateValue('avg-rating', avgRating, true);

    } catch (error) {
        console.error('Ошибка при загрузке статистики:', error);
    }
}

// Создание графика распределения оценок
async function createRatingsChart() {
    try {
        const response = await fetch(`${API_URL}/api/analytics/rating-distribution`);

        if (!response.ok) {
            renderChart([0, 0, 0, 0, 0]);
            return;
        }

        const data = await response.json();

        const chartData = [
            data["1"] || 0,
            data["2"] || 0,
            data["3"] || 0,
            data["4"] || 0,
            data["5"] || 0
        ];

        renderChart(chartData);
    } catch (error) {
        console.error('Ошибка при загрузке данных графика:', error);
        renderChart([0, 0, 0, 0, 0]);
    }
}

function renderChart(data) {
    const ctx = document.getElementById('ratings-chart').getContext('2d');

    if (window.ratingsChart) {
        window.ratingsChart.destroy();
    }

    window.ratingsChart = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: ['1 звезда', '2 звезды', '3 звезды', '4 звезды', '5 звезд'],
            datasets: [{
                label: 'Количество оценок',
                data: data,
                backgroundColor: ['#ef4444', '#f97316', '#eab308', '#22c55e', '#3b82f6'],
                borderRadius: 4,
                borderWidth: 0
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: { display: false },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            return `Оценок: ${context.parsed.y}`;
                        }
                    }
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    grid: { color: '#f3f4f6' },
                    ticks: { stepSize: 1 }
                },
                x: {
                    grid: { display: false }
                }
            }
        }
    });
}

// Уведомления
function showNotification(message, type = 'info') {
    const notification = document.createElement('div');
    notification.className = `fixed top-4 right-4 p-4 rounded-lg shadow-lg z-50 transition-opacity duration-300 ${
        type === 'error' ? 'bg-red-500 text-white' :
        type === 'success' ? 'bg-green-500 text-white' :
        'bg-blue-500 text-white'
    }`;
    notification.textContent = message;

    document.body.appendChild(notification);

    setTimeout(() => {
        notification.style.opacity = '0';
        setTimeout(() => notification.remove(), 300);
    }, 3000);
}

// Обновление данных каждые 5 минут
setInterval(() => {
    loadAnalytics();
}, 300000);