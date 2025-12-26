// API URL
const API_URL = 'http://localhost:8080';

// Загрузка данных при загрузке страницы
document.addEventListener('DOMContentLoaded', function() {
    loadRecentChecks();
});

// Загрузка последних чеков
async function loadRecentChecks() {
    try {
        const response = await fetch(`${API_URL}/api/orders`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);

        const checks = await response.json();

        for (let check of checks) {
            if (!check.cafe_name && check.restaurant_id) {
                try {
                    const cafeResponse = await fetch(`${API_URL}/api/restaurants/${check.restaurant_id}`);
                    if (cafeResponse.ok) {
                        const cafe = await cafeResponse.json();
                        check.cafe_name = cafe.name || `Кафе #${check.restaurant_id}`;
                    }
                } catch {
                    check.cafe_name = `Кафе #${check.restaurant_id}`;
                }
            }
            if (!check.total_amount && check.total_amount !== 0) {
                check.total_amount = check.total || 0;
            }
        }

        displayChecks(checks);
    } catch (error) {
        console.error('Ошибка при загрузке чеков:', error);
        document.getElementById('checks-loading').innerHTML = '<p class="text-red-500">Ошибка при загрузке данных</p>';
    }
}

// Отображение чеков в таблице
function displayChecks(checks) {
    document.getElementById('checks-loading').classList.add('hidden');
    document.getElementById('checks-content').classList.remove('hidden');

    const tbody = document.getElementById('checks-table-body');
    tbody.innerHTML = '';

    if (!checks || checks.length === 0) {
        tbody.innerHTML = '<tr><td colspan="5" class="text-center text-gray-500 py-4">Чеки не найдены</td></tr>';
        return;
    }

    checks.forEach(check => tbody.appendChild(createCheckRow(check)));
}

// Создание строки чека
function createCheckRow(check) {
    const tr = document.createElement('tr');
    const totalAmount = check.total_amount || check.total || 0;

    tr.innerHTML = `
        <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">${check.id}</td>
        <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">${check.cafe_name || `Кафе #${check.restaurant_id}`}</td>
        <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">${totalAmount}₽</td>
        <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">${new Date(check.created_at).toLocaleDateString('ru-RU')}</td>
        <td class="px-6 py-4 whitespace-nowrap text-sm font-medium">
            <button onclick="generateQRCode(${check.id})" class="text-blue-600 hover:text-blue-900 mr-3">
                <i class="fas fa-qrcode mr-1"></i>QR
            </button>
            <button onclick="viewCheck(${check.id})" class="text-green-600 hover:text-green-900">
                <i class="fas fa-eye mr-1"></i>Просмотр
            </button>
        </td>
    `;
    return tr;
}

// Показать модальное окно создания чека
async function showCreateCheckModal() {
    try {
        const response = await fetch(`${API_URL}/api/cafes`);
        if (!response.ok) throw new Error('Failed to load cafes');

        // ✅ Сохраняем результат в переменную
        const data = await response.json();
        const cafes = Array.isArray(data) ? data : [];

        const cafeSelect = document.getElementById('cafe-select');
        cafeSelect.innerHTML = '<option value="">Выберите кафе</option>';

        if (cafes.length === 0) {
            cafeSelect.innerHTML = '<option value="">Нет доступных кафе</option>';
        } else {
            cafes.forEach(cafe => {
                const option = document.createElement('option');
                option.value = cafe.id;
                option.textContent = cafe.name;
                cafeSelect.appendChild(option);
            });
        }

        cafeSelect.onchange = function() {
            if (this.value) loadCafeDishes(parseInt(this.value));
            else document.getElementById('dishes-selection').innerHTML = '';
        };

        document.getElementById('create-check-modal').classList.remove('hidden');
        document.getElementById('create-check-modal').classList.add('flex');
    } catch (error) {
        console.error('Ошибка при загрузке кафе:', error);
        showNotification('Ошибка при загрузке данных', 'error');
    }
}

// Скрыть модальное окно создания чека
function hideCreateCheckModal() {
    document.getElementById('create-check-modal').classList.add('hidden');
    document.getElementById('create-check-modal').classList.remove('flex');
    document.getElementById('create-check-form').reset();
    document.getElementById('dishes-selection').innerHTML = '';
}

// Загрузка блюд кафе
// ИСПРАВЛЕННАЯ ФУНКЦИЯ loadCafeDishes
async function loadCafeDishes(cafeId) {
    try {
        const response = await fetch(`${API_URL}/api/cafe/${cafeId}/menu`);
        if (!response.ok) throw new Error(`HTTP ${response.status}: ${response.statusText}`);

        // ✅ СОХРАНЯЕМ РЕЗУЛЬТАТ В ПЕРЕМЕННУЮ
        const data = await response.json();
        const dishes = Array.isArray(data) ? data : [];

        const container = document.getElementById('dishes-selection');
        container.innerHTML = '';

        if (dishes.length === 0) {
            container.innerHTML = '<p class="text-gray-500">Нет блюд в этом кафе</p>';
            return;
        }

        dishes.forEach(dish => container.appendChild(createDishSelectionElement(dish)));
        updateTotalAmount();
    } catch (error) {
        console.error('Ошибка при загрузке блюд:', error);
        document.getElementById('dishes-selection').innerHTML = '<p class="text-red-500">Ошибка при загрузке блюд</p>';
    }
}

// Создание элемента выбора блюда
function createDishSelectionElement(dish) {
    const div = document.createElement('div');
    div.className = 'flex items-center gap-3 p-3 bg-white border border-gray-300 rounded-lg';

    // ✅ ИСПРАВЛЕНО: Используем dish.dish_id
    const dishId = dish.dish_id || dish.id;

    div.innerHTML = `
        <input type="checkbox" class="dish-checkbox" id="dish-${dishId}" value="${dishId}" onchange="updateTotalAmount()">
        <label for="dish-${dishId}" class="flex-1 cursor-pointer">
            <span class="font-medium">${dish.name}</span>
            <span class="text-gray-600 ml-2">${dish.price}₽</span>
        </label>
        <input type="number" class="dish-quantity" min="1" max="99" value="1" id="quantity-${dishId}" style="width: 4rem;" onchange="updateTotalAmount()">
    `;
    return div;
}

function updateTotalAmount() {
    const totalAmountElement = document.getElementById('total-amount');
    if (!totalAmountElement) return;

    let total = 0;
    document.querySelectorAll('.dish-checkbox:checked').forEach(checkbox => {
        const dishId = checkbox.value;
        const quantity = parseInt(document.getElementById(`quantity-${dishId}`).value) || 1;
        const priceText = checkbox.nextElementSibling.querySelector('span:last-child').textContent;
        total += parseFloat(priceText) * quantity;
    });

    totalAmountElement.textContent = total.toFixed(2);
}

// Обработка создания чека
document.getElementById('create-check-form').addEventListener('submit', async function(e) {
    e.preventDefault();

    const cafeId = document.getElementById('cafe-select').value;
    const selectedDishes = document.querySelectorAll('.dish-checkbox:checked');

    if (!cafeId || selectedDishes.length === 0) {
        showNotification('Заполните все поля и выберите хотя бы одно блюдо', 'error');
        return;
    }

    const checkItems = Array.from(selectedDishes).map(checkbox => {
        const dishId = checkbox.value;
        const quantity = parseInt(document.getElementById(`quantity-${dishId}`).value);
        const priceText = checkbox.nextElementSibling.textContent;
        const price = parseFloat(priceText.match(/\d+\.?\d*/)[0]);
        return { dish_id: parseInt(dishId), quantity, price };
    });

    const totalAmount = checkItems.reduce((sum, item) => sum + (item.price * item.quantity), 0);

    try {
        const response = await fetch(`${API_URL}/api/orders`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ restaurant_id: parseInt(cafeId), items: checkItems, total_amount: totalAmount })
        });

        if (response.ok) {
            showNotification('Чек успешно создан!', 'success');
            hideCreateCheckModal();
            setTimeout(loadRecentChecks, 500);
        } else {
            throw new Error('Failed to create order');
        }
    } catch (error) {
        console.error('Ошибка при создании чека:', error);
        showNotification('Ошибка при создании чека', 'error');
    }
});

// Генерация QR кода
async function generateQRCode(checkId) {
    try {
        const response = await fetch(`${API_URL}/api/orders/${checkId}/qrcode`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);

        const blob = await response.blob();
        const url = URL.createObjectURL(blob);
        showQRCode(url, checkId);
    } catch (error) {
        console.error('Ошибка при генерации QR кода:', error);
        showNotification('Ошибка при генерации QR кода', 'error');
    }
}

// Показать QR код
function showQRCode(url, checkId) {
    const modal = document.createElement('div');
    modal.className = 'fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50';
    modal.innerHTML = `
        <div class="bg-white p-8 rounded-lg max-w-md">
            <h3 class="text-xl font-bold mb-4">QR код для чека #${checkId}</h3>
            <img src="${url}" alt="QR Code" class="w-64 h-64 mx-auto mb-4">
            <p class="text-gray-600 mb-4">Отсканируйте QR код для оставления отзыва</p>
            <button onclick="this.parentElement.parentElement.remove()" class="bg-blue-500 text-white px-4 py-2 rounded-lg hover:bg-blue-600">Закрыть</button>
        </div>
    `;
    document.body.appendChild(modal);
    modal.addEventListener('click', (e) => { if (e.target === modal) modal.remove(); });
}

// Просмотр чека
function viewCheck(checkId) {
    window.open(`review.html?check_id=${checkId}`, '_blank');
}

// === УПРАВЛЕНИЕ КАФЕ ===

// Показать список кафе
async function showCafeList() {
    document.getElementById('cafe-list-modal').classList.remove('hidden');
    document.getElementById('cafe-list-modal').classList.add('flex');
    await loadCafesList();
}

// Скрыть список кафе
function hideCafeListModal() {
    document.getElementById('cafe-list-modal').classList.add('hidden');
    document.getElementById('cafe-list-modal').classList.remove('flex');
}

// Показать форму кафе
function showCafeForm(cafe = null) {
    document.getElementById('cafe-form-modal').classList.remove('hidden');
    document.getElementById('cafe-form-modal').classList.add('flex');

    const form = document.getElementById('cafe-form');
    form.reset();

    if (cafe) {
        document.getElementById('cafe-form-title').textContent = 'Редактировать кафе';
        document.getElementById('cafe-id').value = cafe.id;
        document.getElementById('cafe-name').value = cafe.name;
        document.getElementById('cafe-description').value = cafe.description || '';
        const preview = document.getElementById('cafe-image-preview');
        if (cafe.image_url) {
            preview.src = cafe.image_url;
            preview.classList.remove('hidden');
        } else {
            preview.classList.add('hidden');
        }
    } else {
        document.getElementById('cafe-form-title').textContent = 'Добавить кафе';
        document.getElementById('cafe-id').value = '';
        document.getElementById('cafe-image-preview').classList.add('hidden');
    }
}

// Скрыть форму кафе
function hideCafeFormModal() {
    document.getElementById('cafe-form-modal').classList.add('hidden');
    document.getElementById('cafe-form-modal').classList.remove('flex');
}

// Загрузить список кафе
async function loadCafesList() {
    try {
        const response = await fetch(`${API_URL}/api/cafes`);
        const data = await response.json();
        const cafes = Array.isArray(data) ? data : [];
        const container = document.getElementById('cafes-list-container');
        container.innerHTML = cafes.map(cafe => `
            <div class="flex items-center justify-between p-4 bg-gray-50 rounded-lg border">
                <div class="flex-1">
                    <div class="font-semibold">${cafe.name || 'Без названия'}</div>
                    <div class="text-sm text-gray-500">ID: ${cafe.id}</div>
                </div>
                <div class="flex gap-2">
                    ${cafe.image_url ? `<img src="${cafe.image_url}" class="w-10 h-10 rounded-lg object-cover" alt="">` : ''}
                    <button onclick="editCafe(${cafe.id})" class="text-blue-600 hover:text-blue-800 p-2">
                        <i class="fas fa-edit"></i>
                    </button>
                    <button onclick="deleteCafe(${cafe.id})" class="text-red-600 hover:text-red-800 p-2">
                        <i class="fas fa-trash"></i>
                    </button>
                </div>
            </div>
        `).join('');
    } catch (error) {
        console.error('Ошибка загрузки кафе:', error);
        showNotification('Ошибка загрузки данных', 'error');
    }
}

// Редактировать кафе
async function editCafe(id) {
    try {
        const response = await fetch(`${API_URL}/api/restaurants/${id}`);
        if (!response.ok) throw new Error('Cafe not found');
        const cafe = await response.json();
        hideCafeListModal();
        showCafeForm(cafe);
    } catch (error) {
        console.error('Ошибка загрузки данных кафе:', error);
        showNotification('Ошибка загрузки данных', 'error');
    }
}

// Удалить кафе
async function deleteCafe(id) {
    if (!confirm('⚠️ Удалить кафе? Все блюда также будут удалены!')) return;

    try {
        const response = await fetch(`${API_URL}/api/restaurants/${id}`, { method: 'DELETE' });
        if (response.ok) {
            showNotification('✅ Кафе удалено', 'success');
            loadCafesList();
        } else {
            throw new Error(await response.text());
        }
    } catch (error) {
        console.error('Ошибка удаления:', error);
        showNotification(`❌ Ошибка: ${error.message}`, 'error');
    }
}

// Обработка формы кафе
document.getElementById('cafe-form')?.addEventListener('submit', async function(e) {
    e.preventDefault();

    const submitBtn = this.querySelector('button[type="submit"]');
    submitBtn.disabled = true;
    submitBtn.innerHTML = '<i class="fas fa-spinner fa-spin mr-2"></i>Сохранение...';

    try {
        const id = document.getElementById('cafe-id').value;
        const isNew = !id;
        const url = isNew ? `${API_URL}/api/restaurants` : `${API_URL}/api/restaurants/${id}`;
        const method = isNew ? 'POST' : 'PUT';

        const data = {
            name: document.getElementById('cafe-name').value.trim(),
            description: document.getElementById('cafe-description').value.trim()
        };

        if (!data.name) {
            showNotification('Название кафе обязательно!', 'error');
            return;
        }

        const response = await fetch(url, {
            method: method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });

        if (!response.ok) throw new Error(await response.text());

        const result = await response.json();
        const savedId = result.id || id;

        const fileInput = document.getElementById('cafe-image');
        if (fileInput.files[0]) {
            const formData = new FormData();
            formData.append('image', fileInput.files[0]);
            const imgResponse = await fetch(`${API_URL}/api/restaurants/${savedId}/image`, {
                method: 'POST',
                body: formData
            });
            if (!imgResponse.ok) throw new Error('Failed to upload image');
        }

        showNotification(isNew ? '✅ Кафе создано' : '✅ Кафе обновлено', 'success');
        hideCafeFormModal();
        loadCafesList();
    } catch (error) {
        console.error('Ошибка сохранения:', error);
        showNotification(`❌ Ошибка: ${error.message}`, 'error');
    } finally {
        submitBtn.disabled = false;
        submitBtn.innerHTML = 'Сохранить';
    }
});

// Управление блюдами
// === УПРАВЛЕНИЕ БЛЮДАМИ (НОВОЕ) ===

// 1. Открыть главное окно управления блюдами
async function showDishManagement() {
    const modal = document.getElementById('dish-manager-modal');
    modal.classList.remove('hidden');
    modal.classList.add('flex');

    // Загружаем список кафе в селект
    try {
        const response = await fetch(`${API_URL}/api/cafes`);
        const data = await response.json();
        const cafes = Array.isArray(data) ? data : [];

        const select = document.getElementById('dish-manager-cafe-select');
        select.innerHTML = '<option value="">Выберите кафе для редактирования меню</option>';

        cafes.forEach(cafe => {
            const option = document.createElement('option');
            option.value = cafe.id;
            option.textContent = cafe.name;
            select.appendChild(option);
        });

        // Сброс таблицы
        document.getElementById('dish-manager-table-body').innerHTML =
            '<tr><td colspan="4" class="text-center py-4 text-gray-500">Выберите кафе из списка выше</td></tr>';

        // Отключаем кнопку добавления пока не выбрано кафе
        document.getElementById('btn-add-dish').disabled = true;

    } catch (error) {
        console.error('Error loading cafes:', error);
        showNotification('Ошибка загрузки списка кафе', 'error');
    }
}

// 2. Скрыть главное окно
function hideDishManager() {
    const modal = document.getElementById('dish-manager-modal');
    modal.classList.add('hidden');
    modal.classList.remove('flex');
}

// 3. Обработчик выбора кафе (загрузка блюд)
document.getElementById('dish-manager-cafe-select').addEventListener('change', async function() {
    const cafeId = this.value;
    const tbody = document.getElementById('dish-manager-table-body');
    const btnAdd = document.getElementById('btn-add-dish');

    if (!cafeId) {
        tbody.innerHTML = '<tr><td colspan="4" class="text-center py-4 text-gray-500">Выберите кафе</td></tr>';
        btnAdd.disabled = true;
        return;
    }

    btnAdd.disabled = false;
    tbody.innerHTML = '<tr><td colspan="4" class="text-center py-4"><i class="fas fa-spinner fa-spin"></i> Загрузка...</td></tr>';

    try {
        const response = await fetch(`${API_URL}/api/restaurants/${cafeId}/dishes`);
        const dishes = await response.json(); // API возвращает массив

        tbody.innerHTML = '';
        if (!dishes || dishes.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" class="text-center py-4 text-gray-500">В меню пока пусто</td></tr>';
            return;
        }

        dishes.forEach(dish => {
            // Внутри dishes.forEach(dish => { ...
        const tr = document.createElement('tr');

        // Формируем HTML картинки для таблицы
        let thumbHtml;
        if (dish.image_url) {
            thumbHtml = `<img src="${dish.image_url}?t=${new Date().getTime()}" class="w-12 h-12 rounded object-cover border">`;
        } else {
            thumbHtml = `<div class="w-12 h-12 bg-gray-100 rounded flex items-center justify-center border"><i class="fas fa-utensils text-gray-300"></i></div>`;
        }

        tr.innerHTML = `
            <td class="px-4 py-2 whitespace-nowrap">${thumbHtml}</td>
            <td class="px-4 py-2 text-sm font-medium text-gray-900">${dish.name}</td>
            <td class="px-4 py-2 text-sm text-gray-600">${dish.price}₽</td>
            <td class="px-4 py-2 text-sm text-gray-500 truncate max-w-xs">${dish.description || '-'}</td>
            <td class="px-4 py-2 text-right text-sm font-medium">
                <button onclick='openEditDishModal(${JSON.stringify(dish).replace(/'/g, "&#39;")})' class="text-blue-600 hover:text-blue-900 mr-3">
                    <i class="fas fa-edit"></i>
                </button>
                <button onclick="deleteDish(${dish.id})" class="text-red-600 hover:text-red-900">
                    <i class="fas fa-trash"></i>
                </button>
            </td>
        `;
        tbody.appendChild(tr);
        });

    } catch (error) {
        console.error(error);
        tbody.innerHTML = '<tr><td colspan="4" class="text-center py-4 text-red-500">Ошибка загрузки меню</td></tr>';
    }
});

// 4. Открыть форму добавления блюда
function openAddDishModal() {
    document.getElementById('dish-form').reset();
    const modal = document.getElementById('dish-form-modal');
    modal.classList.remove('hidden');
    modal.classList.add('flex');
}

// 5. Скрыть форму добавления
function hideDishFormModal() {
    const modal = document.getElementById('dish-form-modal');
    modal.classList.add('hidden');
    modal.classList.remove('flex');
}

// 6. ОТПРАВКА ФОРМЫ (Создание блюда + Фото)
document.getElementById('dish-form').addEventListener('submit', async function(e) {
    e.preventDefault();

    const cafeId = document.getElementById('dish-manager-cafe-select').value;
    const dishId = document.getElementById('dish-id').value; // Проверяем, есть ли ID

    if (!cafeId) return;

    const submitBtn = this.querySelector('button[type="submit"]');
    submitBtn.disabled = true;
    submitBtn.innerHTML = '<i class="fas fa-spinner fa-spin"></i>';

    try {
        const dishData = {
            restaurant_id: parseInt(cafeId),
            name: document.getElementById('dish-name').value,
            price: parseFloat(document.getElementById('dish-price').value),
            description: document.getElementById('dish-description').value
        };

        let savedDishId;

        // Если есть ID -> делаем PUT (обновление)
        // Если нет ID -> делаем POST (создание)
        if (dishId) {
            const updateResp = await fetch(`${API_URL}/api/restaurants/${cafeId}/dishes/${dishId}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(dishData)
            });
            if (!updateResp.ok) throw new Error('Failed to update dish');
            savedDishId = dishId;
            showNotification('Блюдо обновлено!', 'success');
        } else {
            const createResp = await fetch(`${API_URL}/api/restaurants/${cafeId}/dishes`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(dishData)
            });
            if (!createResp.ok) throw new Error('Failed to create dish');
            const createdDish = await createResp.json();
            savedDishId = createdDish.id;
            showNotification('Блюдо создано!', 'success');
        }

        // Загрузка фото (работает одинаково для создания и обновления)
        const fileInput = document.getElementById('dish-image');
        if (fileInput.files[0]) {
            const formData = new FormData();
            formData.append('image', fileInput.files[0]);

            const imgResp = await fetch(`${API_URL}/api/restaurants/${cafeId}/dishes/${savedDishId}/image`, {
                method: 'POST',
                body: formData
            });
            if (!imgResp.ok) console.error("Ошибка загрузки фото");
        }

        hideDishFormModal();
        // Обновляем таблицу
        document.getElementById('dish-manager-cafe-select').dispatchEvent(new Event('change'));

    } catch (error) {
        console.error(error);
        showNotification('Ошибка при сохранении', 'error');
    } finally {
        submitBtn.disabled = false;
        submitBtn.innerHTML = 'Сохранить';
    }
});

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

// Открытие модалки для НОВОГО блюда
function openEditDishModal(dish) {
    document.getElementById('dish-form').reset();

    // ✅ ИСПРАВЛЕНО: Используем dish.dish_id вместо dish.id
    document.getElementById('dish-id').value = dish.dish_id || dish.id || '';
    document.getElementById('dish-name').value = dish.name;
    document.getElementById('dish-price').value = dish.price;
    document.getElementById('dish-description').value = dish.description || '';

    const previewContainer = document.getElementById('dish-preview-container');
    const previewImg = document.getElementById('dish-preview-img');

    // ✅ ИСПРАВЛЕНО: Проверяем правильное поле
    if (dish.image_url) {
        previewImg.src = `${dish.image_url}?t=${new Date().getTime()}`;
        previewContainer.classList.remove('hidden');
    } else {
        previewContainer.classList.add('hidden');
    }

    document.querySelector('#dish-form-modal h3').textContent = 'Редактировать блюдо';

    const modal = document.getElementById('dish-form-modal');
    modal.classList.remove('hidden');
    modal.classList.add('flex');
}

// Также обнови openAddDishModal, чтобы скрывать превью
function openAddDishModal() {
    document.getElementById('dish-form').reset();
    document.getElementById('dish-id').value = '';
    document.getElementById('dish-preview-container').classList.add('hidden'); // Скрываем превью

    document.querySelector('#dish-form-modal h3').textContent = 'Новое блюдо';

    const modal = document.getElementById('dish-form-modal');
    modal.classList.remove('hidden');
    modal.classList.add('flex');
}

// Удаление блюда
async function deleteDish(dishId) {
    if (!confirm('Вы уверены, что хотите удалить это блюдо?')) return;

    const cafeId = document.getElementById('dish-manager-cafe-select').value;

    try {
        const response = await fetch(`${API_URL}/api/restaurants/${cafeId}/dishes/${dishId}`, {
            method: 'DELETE'
        });

        if (response.ok) {
            showNotification('Блюдо удалено', 'success');
            // Обновляем таблицу
            document.getElementById('dish-manager-cafe-select').dispatchEvent(new Event('change'));
        } else {
            throw new Error('Failed to delete');
        }
    } catch (error) {
        console.error(error);
        showNotification('Ошибка удаления', 'error');
    }
}

// Закрытие модальных окон по клику вне их
document.addEventListener('click', function(e) {
    if (e.target.id === 'create-check-modal') hideCreateCheckModal();
    if (e.target.id === 'cafe-list-modal') hideCafeListModal();
    if (e.target.id === 'cafe-form-modal') hideCafeFormModal();
    if (e.target.id === 'dish-manager-modal') hideDishManager();
    if (e.target.id === 'dish-form-modal') hideDishFormModal();
});