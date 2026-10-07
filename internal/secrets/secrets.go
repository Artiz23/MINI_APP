package secrets

import "os"

var BotToken = os.Getenv("MINIAPP_BOT_TOKEN")

// Все секреты главного бота Mini App. Токен впишите сами.
// Чаты курсов и согласования больше не нужны: всё смотрите в Mini App.
const (
	// Главный бот (BotFather). Пусто = задайте перед запуском.
	// Публичный HTTPS-адрес, который открывает Mini App (кнопка в боте).
	// Пример: https://your-domain.com
	WebAppURL = "https://maybeeidolons.site/?v=91"

	// HTTP-сервер Mini App + API.
	ListenAddr = ":8080"

	// Группа-форум, где бот создаёт тему по заявке. 0 = только запись в Mini App.
	ForumChatID int64 = 0

	// Чаты обращений из заявки Mini App. Бот должен быть участником группы.
	DocsChatID   int64 = 0
	LawyerChatID int64 = 0
)

// MainAdminIDs — вы (владельцы). Нельзя снять из Mini App.
// Дополнительно: сводка в 10:00, доступы, спорные случаи.
var MainAdminIDs = []int64{
	6592001752,
	664767965,
}

func IsMain(id int64) bool {
	for _, a := range MainAdminIDs {
		if a == id {
			return true
		}
	}
	return false
}
