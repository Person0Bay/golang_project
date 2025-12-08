// API URL
const API_URL = 'http://localhost:8080';

let currentCheck = null;

// Получаем ID чека из URL
function getCheckIdFromUrl() {
    const urlParams = new URLSearchParams(window.location.search);
    return urlParams.get('check_id');
}

// Загрузка данных чека при загрузке страницы
document.addEventListener('DOMContentLoaded', function() {
    const checkId = getCheckIdFromUrl();
    if (checkId) {
        loadCheck(checkId);
    } else {
        showError('ID чека не найден в URL');
    }
});

// Загрузка данных чека
async function loadCheck(checkId) {
    try {
        const response = await fetch(`${API_URL}/api/check/${checkId}`);
        if (!response.ok) throw new Error('Чек не найден');

        const check = await response.json();
        currentCheck = check;
        displayCheckInfo(check);
        displayDishes(check);
    } catch (error) {
        console.error('Ошибка при загрузке чека:', error);
        showError('Ошибка при загрузке данных чека');
    }
}

// Отображение информации о чеке
function displayCheckInfo(check) {
    document.getElementById('check-id').textContent = check.id;
    document.getElementById('check-total').textContent = check.total_amount || check.total || 0;
    document.getElementById('check-date').textContent = new Date(check.created_at).toLocaleDateString('ru-RU');
}

// Отображение блюд для оценки
function displayDishes(check) {
    const container = document.getElementById('dishes-container');
    container.innerHTML = '';

    check.items.forEach((item, index) => {
        const dishElement = createDishReviewElement(item, index, check.restaurant_id);
        container.appendChild(dishElement);
    });
}

// Создание элемента для оценки блюда
function createDishReviewElement(item, index, restaurantId) {
    const div = document.createElement('div');
    div.className = 'bg-gray-50 rounded-lg p-6 mb-6';
    div.dataset.dishId = item.dish_id;
    div.dataset.restaurantId = restaurantId;

    div.innerHTML = `
        <div class="flex items-start space-x-4">
            <div class="flex-1">
                <h4 class="text-lg font-semibold text-gray-900 mb-2">${item.dish_name}</h4>
                <p class="text-gray-600 mb-4">Количество: ${item.quantity} × ${item.price}₽</p>

                <div class="mb-4">
                    <label class="block text-sm font-medium text-gray-700 mb-2">Ваша оценка:</label>
                    <div class="flex space-x-1 rating-container" data-dish-id="${item.dish_id}">
                        ${[1,2,3,4,5].map(star =>
                            `<i class="fas fa-star rating-star inactive" data-rating="${star}"></i>`
                        ).join('')}
                    </div>
                </div>

                <div>
                    <label for="comment-${item.dish_id}" class="block text-sm font-medium text-gray-700 mb-2">
                        Комментарий (необязательно):
                    </label>
                    <textarea id="comment-${item.dish_id}" rows="3"
                              class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-orange-500"
                              placeholder="Расскажите, что понравилось или что можно улучшить..."></textarea>
                </div>
            </div>
        </div>
    `;

    // Обработчики для звезд рейтинга
    const stars = div.querySelectorAll('.rating-star');
    stars.forEach(star => {
        star.addEventListener('click', function() {
            setRating(stars, parseInt(this.dataset.rating));
        });
        star.addEventListener('mouseenter', function() {
            highlightStars(stars, parseInt(this.dataset.rating));
        });
    });

    const starContainer = div.querySelector('.rating-container');
    starContainer.addEventListener('mouseleave', () => resetStarHighlight(stars));

    return div;
}

// Установка рейтинга
function setRating(stars, rating) {
    stars.forEach((star, index) => {
        star.classList.toggle('active', index < rating);
        star.classList.toggle('inactive', index >= rating);
    });
    stars[0].parentElement.dataset.selectedRating = rating;
}

// Подсветка звезд при наведении
function highlightStars(stars, rating) {
    stars.forEach((star, index) => {
        star.style.color = index < rating ? '#fbbf24' : '#d1d5db';
    });
}

// Сброс подсветки звезд
function resetStarHighlight(stars) {
    const selectedRating = stars[0].parentElement.dataset.selectedRating;
    if (selectedRating) {
        setRating(stars, parseInt(selectedRating));
    } else {
        stars.forEach(star => star.style.color = '');
    }
}

// Обработка отправки формы
document.getElementById('review-form').addEventListener('submit', async function(e) {
    e.preventDefault();
    const checkId = getCheckIdFromUrl();

    const reviews = Array.from(document.querySelectorAll('.rating-container')).map(container => ({
        dish_id: parseInt(container.dataset.dishId),
        rating: parseInt(container.dataset.selectedRating) || 0,
        comment: container.closest('.bg-gray-50').querySelector('textarea').value
    })).filter(r => r.rating > 0);

    if (reviews.length === 0) {
        showNotification('Оцените хотя бы одно блюдо', 'error');
        return;
    }

    const payload = {
        check_id: parseInt(checkId),
        restaurant_id: currentCheck ? currentCheck.restaurant_id : null,
        reviews: reviews
    };

    try {
        const response = await fetch(`${API_URL}/api/reviews`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });

        if (response.ok) {
            showSuccess();
        } else {
            throw new Error('Failed to submit review');
        }
    } catch (error) {
        console.error('Ошибка при отправке:', error);
        showNotification('Ошибка при отправке отзыва', 'error');
    }
});

// Показать сообщение об успехе
function showSuccess() {
    const form = document.getElementById('review-form');
    form.innerHTML = `
        <div class="text-center py-12">
            <div class="mb-6">
                <i class="fas fa-check-circle text-6xl text-green-500"></i>
            </div>
            <h3 class="text-2xl font-bold text-gray-900 mb-4">Спасибо за ваш отзыв!</h3>
            <p class="text-gray-600 mb-6">Ваше мнение очень важно для нас и помогает улучшить качество обслуживания.</p>
            <button onclick="window.location.href='index.html'"
                    class="bg-orange-500 text-white px-6 py-3 rounded-lg hover:bg-orange-600 transition">
                <i class="fas fa-home mr-2"></i>На главную
            </button>
        </div>
    `;
}

// Показать ошибку
function showError(message) {
    document.querySelector('main').innerHTML = `
        <div class="bg-white rounded-lg shadow-lg p-8 text-center max-w-2xl mx-auto">
            <div class="mb-6">
                <i class="fas fa-exclamation-triangle text-6xl text-red-500"></i>
            </div>
            <h3 class="text-2xl font-bold text-gray-900 mb-4">Ошибка</h3>
            <p class="text-gray-600 mb-6">${message}</p>
            <button onclick="window.location.href='index.html'"
                    class="bg-orange-500 text-white px-6 py-3 rounded-lg hover:bg-orange-600 transition">
                <i class="fas fa-home mr-2"></i>На главную
            </button>
        </div>
    `;
}

// Уведомления
function showNotification(message, type = 'info') {
    const notification = document.createElement('div');
    notification.className = `fixed top-4 right-4 p-4 rounded-lg shadow-lg z-50 ${
        type === 'error' ? 'bg-red-500 text-white' :
        type === 'success' ? 'bg-green-500 text-white' :
        'bg-blue-500 text-white'
    }`;
    notification.textContent = message;
    document.body.appendChild(notification);
    setTimeout(() => notification.remove(), 3000);
}