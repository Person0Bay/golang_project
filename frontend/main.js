// API URL
const API_URL = 'http://localhost:8080';

// Загрузка кафе при загрузке страницы
document.addEventListener('DOMContentLoaded', function() {
    loadCafes();
});

// Загрузка списка кафе
async function loadCafes() {
    try {
        const response = await fetch(`${API_URL}/api/cafes`);
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }

        const data = await response.json();
        const cafes = Array.isArray(data) ? data : [];

        const cafesGrid = document.getElementById('cafes-grid');
        cafesGrid.innerHTML = '';

        if (cafes.length === 0) {
            cafesGrid.innerHTML = '<p class="text-gray-500 text-center w-full col-span-full">Нет доступных кафе</p>';
        } else {
            cafes.forEach(cafe => {
                const cafeCard = createCafeCard(cafe);
                cafesGrid.appendChild(cafeCard);
            });
        }
    } catch (error) {
        console.error('Ошибка при загрузке кафе:', error);
        showNotification('Ошибка при загрузке данных', 'error');
    }
}


// Создание карточки кафе (Tailwind)
function createCafeCard(cafe) {
    const card = document.createElement('div');
    card.className = 'bg-white rounded-lg shadow-md overflow-hidden cursor-pointer transition transform hover:-translate-y-1 group';
    card.onclick = () => showMenu(cafe.id, cafe.name);

    // Логика отображения: Картинка или Плейсхолдер
    let imageContent;

    if (cafe.image_url && cafe.image_url.trim() !== "") {
        // Если есть картинка
        imageContent = `
            <div class="h-48 overflow-hidden relative">
                <img src="${cafe.image_url}" alt="${cafe.name}" class="w-full h-full object-cover transition duration-500 group-hover:scale-110">
                <div class="absolute inset-0 bg-black bg-opacity-20 group-hover:bg-opacity-10 transition"></div>
            </div>
        `;
    } else {
        // Если картинки нет (Плейсхолдер)
        imageContent = `
            <div class="bg-gradient-to-r from-orange-400 to-red-500 h-48 flex items-center justify-center">
                <i class="fas fa-store text-6xl text-white opacity-80"></i>
            </div>
        `;
    }

    card.innerHTML = `
        ${imageContent}
        <div class="p-6">
            <h4 class="text-xl font-bold text-gray-900 mb-2">${cafe.name}</h4>
            <p class="text-gray-600 mb-4 line-clamp-2">${cafe.description || 'Описание отсутствует'}</p>
            <div class="flex items-center justify-between">
                <span class="text-sm text-gray-500 bg-gray-100 px-2 py-1 rounded">ID: ${cafe.id}</span>
                <button class="text-orange-500 font-medium hover:text-orange-600 transition flex items-center">
                    Меню <i class="fas fa-arrow-right ml-2 text-sm"></i>
                </button>
            </div>
        </div>
    `;

    return card;
}

// Показ меню кафе
// ИСПРАВЛЕННАЯ ФУНКЦИЯ showMenu
async function showMenu(cafeId, cafeName) {
    try {
        const response = await fetch(`${API_URL}/api/cafe/${cafeId}/menu`);
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }

        const data = await response.json();
        const dishes = Array.isArray(data) ? data : [];

        // ✅ ПРОВЕРКА СУЩЕСТВОВАНИЯ ЭЛЕМЕНТА
        const titleEl = document.getElementById('cafe-name-title');
        if (titleEl) {
            titleEl.textContent = `Меню - ${cafeName}`;
        } else {
            console.error("Element #cafe-name-title not found in DOM");
        }

        document.getElementById('cafes-section').classList.add('hidden');
        document.getElementById('menu-section').classList.remove('hidden');

        const menuGrid = document.getElementById('menu-grid');
        menuGrid.innerHTML = '';

        if (dishes.length === 0) {
            menuGrid.innerHTML = '<p class="text-gray-500 text-center w-full col-span-full">Нет блюд для отображения</p>';
        } else {
            dishes.forEach(dish => {
                const dishCard = createDishCard(dish);
                menuGrid.appendChild(dishCard);
            });
        }
    } catch (error) {
        console.error('Ошибка при загрузке меню:', error);
        showNotification('Ошибка при загрузке меню', 'error');
    }
}

// Скрыть меню
function hideMenu() {
    document.getElementById('cafes-section').classList.remove('hidden');
    document.getElementById('menu-section').classList.add('hidden');
}

// Создание карточки блюда (Tailwind)
// main.js - Функция создания карточки блюда
function createDishCard(dish) {
    const card = document.createElement('div');
    card.className = 'bg-white rounded-lg shadow-md overflow-hidden hover:shadow-lg transition duration-300';

    // Логика картинки или плейсхолдера
  // main.js -> createDishCard
    let imageContent;

    // БЕЗОПАСНАЯ ПРОВЕРКА: Проверяем, что это строка И она не пустая
    if (dish.image_url && typeof dish.image_url === 'string' && dish.image_url.trim() !== "") {
        // Добавляем timestamp для сброса кэша
        const imageUrl = `${dish.image_url}?t=${new Date().getTime()}`;
        imageContent = `<img src="${imageUrl}" alt="${dish.name}" class="w-full h-48 object-cover">`;
    } else {
        imageContent = `
            <div class="bg-gradient-to-r from-gray-200 to-gray-300 h-48 flex items-center justify-center">
                <i class="fas fa-utensils text-4xl text-gray-400"></i>
            </div>
        `;
    }

    card.innerHTML = `
        <div class="relative">
            ${imageContent}
            <div class="absolute top-4 right-4 bg-white px-3 py-1 rounded-lg shadow">
                <span class="text-lg font-bold text-orange-500">${dish.price}₽</span>
            </div>
        </div>
        <div class="p-6">
            <h5 class="text-xl font-bold text-gray-900 mb-2">${dish.name}</h5>
            <p class="text-gray-600 mb-4 text-sm line-clamp-2">${dish.description || 'Описание отсутствует'}</p>
            <div class="flex items-center justify-between">
                <span class="text-xs text-gray-500 bg-gray-100 px-2 py-1 rounded">ID: ${dish.id}</span>
                <div class="flex items-center">
                    <i class="fas fa-star text-yellow-400 mr-1"></i>
                    <span class="text-xs text-gray-600">Оцените после заказа</span>
                </div>
            </div>
        </div>
    `;

    return card;
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

// Генерация и показ QR кода (для админа)
async function generateQRCode(checkId) {
    try {
        const response = await fetch(`${API_URL}/api/orders/${checkId}/qrcode`);
        if (response.ok) {
            const blob = await response.blob();
            const url = URL.createObjectURL(blob);
            showQRCode(url, checkId);
        }
    } catch (error) {
        console.error('Ошибка при генерации QR кода:', error);
        showNotification('Ошибка при генерации QR кода', 'error');
    }
}

// Показать QR код в модальном окне
function showQRCode(url, checkId) {
    const modal = document.createElement('div');
    modal.className = 'fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50';
    modal.innerHTML = `
        <div class="bg-white p-8 rounded-lg max-w-md">
            <h3 class="text-xl font-bold mb-4">QR код для чека #${checkId}</h3>
            <img src="${url}" alt="QR Code" class="w-64 h-64 mx-auto mb-4">
            <p class="text-gray-600 mb-4">Отсканируйте QR код для оставления отзыва</p>
            <button onclick="this.parentElement.parentElement.remove()" class="bg-orange-500 text-white px-4 py-2 rounded-lg hover:bg-orange-600">Закрыть</button>
        </div>
    `;
    document.body.appendChild(modal);
    modal.addEventListener('click', (e) => { if (e.target === modal) modal.remove(); });
}