package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"tg-bot-orh3/MINI_APP/internal/appdb"
	"tg-bot-orh3/MINI_APP/internal/secrets"
)

type Bot struct {
	api     *tgbotapi.BotAPI
	db      *appdb.Store
	leaving sync.Map
}

func NewBot(db *appdb.Store) (*Bot, error) {
	if strings.TrimSpace(secrets.BotToken) == "" {
		return nil, fmt.Errorf("впишите BotToken в MINI_APP/internal/secrets/secrets.go")
	}
	api, err := tgbotapi.NewBotAPI(secrets.BotToken)
	if err != nil {
		return nil, err
	}
	log.Printf("miniapp: бот @%s", api.Self.UserName)
	b := &Bot{api: api, db: db}
	b.setup()
	return b, nil
}

func (b *Bot) setup() {
	_, _ = b.api.Request(tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "start", Description: "Открыть систему"},
		tgbotapi.BotCommand{Command: "done", Description: "Закончить приём файлов в заявку"},
		tgbotapi.BotCommand{Command: "undo", Description: "Удалить последнюю пересылку из заявки"},
		tgbotapi.BotCommand{Command: "clear", Description: "Удалить всё, что я отправил в заявку"},
		tgbotapi.BotCommand{Command: "id", Description: "Мой Telegram ID"},
	))
	_, _ = b.api.Request(tgbotapi.NewSetMyCommandsWithScope(
		tgbotapi.NewBotCommandScopeAllGroupChats(),
		tgbotapi.BotCommand{Command: "id", Description: "ID этой группы"},
		tgbotapi.BotCommand{Command: "chatid", Description: "ID этой группы"},
	))
	if u := strings.TrimSpace(secrets.WebAppURL); u != "" && !strings.Contains(u, "YOUR_DOMAIN") {
		raw, _ := json.Marshal(map[string]any{
			"type": "web_app", "text": "Worklet",
			"web_app": map[string]string{"url": u},
		})
		p := tgbotapi.Params{}
		p["menu_button"] = string(raw)
		if _, err := b.api.MakeRequest("setChatMenuButton", p); err != nil {
			log.Printf("miniapp menu button: %v", err)
		}
	}
}

func webAppURL(section string) string {
	u := strings.TrimSpace(secrets.WebAppURL)
	if section == "" || u == "" {
		return u
	}
	sep := "?"
	if strings.Contains(u, "?") {
		sep = "&"
	}
	return u + sep + "go=" + section
}

func webBtn(text, section string) map[string]any {
	u := webAppURL(section)
	if u == "" || strings.Contains(u, "YOUR_DOMAIN") {
		return map[string]any{"text": text}
	}
	return map[string]any{"text": text, "web_app": map[string]string{"url": u}}
}

func textBtn(text string) map[string]any {
	return map[string]any{"text": text}
}

func (b *Bot) mainKeyboard() map[string]any {
	return map[string]any{
		"keyboard": [][]map[string]any{
			{webBtn("Открыть Worklet", "")},
			{webBtn("Заявки", "requests"), webBtn("Канбан", "tasks")},
			{webBtn("Сальдо", "saldo"), webBtn("Согласование", "approvals")},
			{textBtn("Отменить последнее"), textBtn("Убрать отправленное")},
			{textBtn("Мой ID")},
		},
		"resize_keyboard": true,
		"is_persistent":   true,
	}
}

func (b *Bot) attachKeyboard() map[string]any {
	return map[string]any{
		"keyboard": [][]map[string]any{
			{webBtn("Открыть Worklet", "")},
			{textBtn("Убрать отправленное")},
			{textBtn("Отменить последнее")},
			{textBtn("Готово — хватит файлов")},
		},
		"resize_keyboard":         true,
		"is_persistent":           true,
		"input_field_placeholder": "Перешлите файл или сообщение",
	}
}

func mapBotButton(text, cmd string) string {
	t := strings.ToLower(strings.TrimSpace(text))
	t = strings.TrimPrefix(t, "↩ ")
	switch {
	case t == "убрать отправленное" || t == "удалить отправленное" || strings.Contains(t, "убрать отправлен") || strings.Contains(t, "удалить отправлен"):
		return "/clear"
	case t == "удалить всё моё" || t == "удалить все мое" || t == "удалить все моё" || strings.Contains(t, "удалить всё мо") || strings.Contains(t, "удалить все мо"):
		return "/clear"
	case t == "отменить последнее" || strings.Contains(t, "отменить послед"):
		return "/undo"
	case t == "готово" || strings.HasPrefix(t, "готово") || t == "хватит файлов" || t == "больше не ждать":
		return "/done"
	case t == "мой id" || t == "id":
		return "/id"
	default:
		return cmd
	}
}

func (b *Bot) Username() string {
	if b == nil || b.api == nil {
		return ""
	}
	return b.api.Self.UserName
}

func (b *Bot) Listen(ctx context.Context) {
	_, _ = b.api.MakeRequest("deleteWebhook", tgbotapi.Params{})
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	u.AllowedUpdates = []string{"message", "edited_message", "callback_query", "my_chat_member", "channel_post"}
	ch := b.api.GetUpdatesChan(u)
	for {
		select {
		case <-ctx.Done():
			return
		case upd := <-ch:
			if upd.Message != nil {
				b.noteMigrate(upd.Message)
			}
			if upd.MyChatMember != nil {
				b.onMyChatMember(upd.MyChatMember)
				continue
			}
			if upd.ChannelPost != nil {
				b.rejectForeignChat(upd.ChannelPost.Chat)
				continue
			}
			if upd.CallbackQuery != nil {
				if msg := upd.CallbackQuery.Message; msg != nil && msg.Chat != nil && !msg.Chat.IsPrivate() && b.rejectForeignChat(msg.Chat) {
					continue
				}
				b.handleCallback(upd.CallbackQuery)
				continue
			}
			if upd.Message == nil {
				continue
			}
			if b.addedToGroup(upd.Message) {
				continue
			}
			if !upd.Message.Chat.IsPrivate() && upd.Message.From == nil {
				b.rejectForeignChat(upd.Message.Chat)
				continue
			}
			if upd.Message.From == nil {
				continue
			}
			b.handle(ctx, upd.Message)
		}
	}
}

func (b *Bot) keepGroup(chatID int64) bool {
	if chatID == 0 {
		return false
	}
	if secrets.ForumChatID != 0 && chatID == secrets.ForumChatID {
		return true
	}
	if secrets.LawyerChatID != 0 && chatID == secrets.LawyerChatID {
		return true
	}
	if secrets.DocsChatID != 0 && chatID == secrets.DocsChatID {
		return true
	}
	lawyer, docs := b.db.AppealChats()
	if chatID == lawyer || chatID == docs {
		return true
	}
	for _, id := range b.db.KnownChatIDs() {
		if chatID == id {
			return true
		}
	}
	return false
}

func (b *Bot) leaveGroup(chatID int64, title string) {
	log.Printf("miniapp: выхожу из группы %d (%s)", chatID, title)
	var last error
	for i := 0; i < 3; i++ {
		p := tgbotapi.Params{}
		p["chat_id"] = strconv.FormatInt(chatID, 10)
		if _, err := b.api.MakeRequest("leaveChat", p); err != nil {
			last = err
			log.Printf("miniapp leave %d try %d: %v", chatID, i+1, err)
			time.Sleep(400 * time.Millisecond)
			continue
		}
		return
	}
	if last != nil {
		log.Printf("miniapp leave %d failed: %v", chatID, last)
	}
}

func (b *Bot) noteMigrate(m *tgbotapi.Message) {
	if m == nil || m.Chat == nil {
		return
	}
	if m.MigrateToChatID != 0 {
		b.onMigrate(m.Chat.ID, m.MigrateToChatID)
	}
	if m.MigrateFromChatID != 0 {
		b.onMigrate(m.MigrateFromChatID, m.Chat.ID)
	}
}

func (b *Bot) onMigrate(oldID, newID int64) {
	if oldID == 0 || newID == 0 || oldID == newID {
		return
	}
	if b.db.RemapChatID(oldID, newID) {
		log.Printf("miniapp: чат %d стал супергруппой %d — ID обновлён", oldID, newID)
	}
}

func (b *Bot) rejectForeignChat(chat *tgbotapi.Chat) bool {
	if chat == nil || chat.ID == 0 || chat.IsPrivate() || b.keepGroup(chat.ID) {
		return false
	}
	id := chat.ID
	title := strings.TrimSpace(chat.Title)
	if _, loaded := b.leaving.LoadOrStore(id, true); loaded {
		return true
	}
	go func() {
		time.Sleep(2500 * time.Millisecond)
		b.leaving.Delete(id)
		if b.keepGroup(id) {
			log.Printf("miniapp: чат %d оставил — после смены ID это сохранённая группа", id)
			return
		}
		log.Printf("miniapp: чужая группа %d %q — выхожу", id, title)
		html := fmt.Sprintf("Этот бот не остаётся в чужих группах.\nID чата: <code>%d</code>", id)
		msg := tgbotapi.NewMessage(id, html)
		msg.ParseMode = "HTML"
		_, _ = b.api.Send(msg)
		b.leaveGroup(id, title)
	}()
	return true
}

func (b *Bot) onMyChatMember(u *tgbotapi.ChatMemberUpdated) {
	if u == nil || u.Chat.IsPrivate() {
		return
	}
	st := strings.ToLower(u.NewChatMember.Status)
	if st == "left" || st == "kicked" {
		return
	}
	b.rejectForeignChat(&u.Chat)
}

func (b *Bot) addedToGroup(m *tgbotapi.Message) bool {
	if m == nil || m.Chat.IsPrivate() {
		return false
	}
	me := b.api.Self.ID
	for _, u := range m.NewChatMembers {
		if u.ID == me {
			b.rejectForeignChat(m.Chat)
			return true
		}
	}
	return false
}

func (b *Bot) handle(ctx context.Context, m *tgbotapi.Message) {
	uid := m.From.ID
	name := strings.TrimSpace(m.From.FirstName + " " + m.From.LastName)
	if isOwner(uid) {
		b.db.UpsertUser(appdb.User{ID: uid, Name: name, Username: m.From.UserName, Role: appdb.RoleOwner})
	}

	text := strings.TrimSpace(m.Text)
	if text == "" {
		text = strings.TrimSpace(m.Caption)
	}
	cmd := strings.ToLower(strings.TrimSpace(m.Command()))
	if cmd == "" {
		cmd = strings.ToLower(text)
		if i := strings.IndexAny(cmd, "@ "); i > 0 && strings.HasPrefix(cmd, "/") {
			cmd = cmd[:i]
		}
	} else {
		cmd = "/" + cmd
	}
	cmd = mapBotButton(text, cmd)

	if !m.Chat.IsPrivate() {
		if b.keepGroup(m.Chat.ID) {
			if cmd == "/id" || cmd == "/chatid" || cmd == "/chat_id" {
				b.replyChatIDs(m)
			}
			_ = b.handleAppealRelay(m)
			return
		}
		b.rejectForeignChat(m.Chat)
		return
	}

	if b.handleAppealRelay(m) {
		return
	}
	if b.handlePrivateAttach(m, uid, name, cmd) {
		return
	}

	switch cmd {
	case "/id", "/chatid", "/chat_id":
		b.replyChatIDs(m)
		return
	case "/start", "/help", "":
	default:
		if !strings.HasPrefix(cmd, "/") {
			if ok, _ := b.canOpen(uid); ok {
				u, _ := b.db.User(uid)
				b.sendHome(m.Chat.ID, isOwner(uid) || u.IsAdmin())
				return
			}
			b.reply(m.Chat.ID, "Откройте Mini App кнопкой ниже.")
			return
		}
	}

	if ok, msg := b.canOpen(uid); !ok {
		b.reply(m.Chat.ID, msg)
		return
	}
	u, _ := b.db.User(uid)
	b.sendHome(m.Chat.ID, isOwner(uid) || u.IsAdmin())
}

func (b *Bot) canOpen(uid int64) (bool, string) {
	if isOwner(uid) {
		return true, ""
	}
	u, ok := b.db.User(uid)
	if !ok {
		return false, fmt.Sprintf("Нет доступа. Передайте ID <code>%d</code> владельцу — сначала Доступы, потом Справочник → Сотрудники.", uid)
	}
	if u.IsAdmin() {
		return true, ""
	}
	if _, emp := b.db.EmployeeByTelegram(uid); !emp {
		return false, "Доступ есть, но Mini App откроется, когда вас добавят в Справочник → Сотрудники."
	}
	return true, ""
}

func (b *Bot) handlePrivateAttach(m *tgbotapi.Message, uid int64, name, cmd string) bool {
	if ok, _ := b.canOpen(uid); !ok {
		return false
	}
	if cmd == "/done" || cmd == "/stop" || cmd == "/готово" {
		if w, _, ok := b.db.AttachWaitOf(uid); ok {
			b.db.ClearAttachWait(uid)
			where := "в заявку"
			if w.IsVault() {
				where = "в документы"
			}
			b.sendHTML(m.Chat.ID, "Больше не жду пересылку. Уже попавшее "+where+": кнопка «Отменить последнее» или «Удалить» в Mini App.", b.mainKeyboard())
			return true
		}
	}
	if cmd == "/undo" || cmd == "/cancel" || cmd == "/отмена" {
		n, err := b.db.UndoLastSent(uid)
		if err != nil {
			b.sendHTML(m.Chat.ID, tgEsc(err.Error()), b.mainKeyboard())
			return true
		}
		wait, _, waiting := b.db.AttachWaitOf(uid)
		kb := b.mainKeyboard()
		if waiting {
			kb = b.attachKeyboard()
		}
		where := "из заявки"
		if waiting && wait.IsVault() {
			where = "из документов"
		}
		b.sendHTML(m.Chat.ID, "Убрал "+where+": "+strconv.Itoa(n)+".", kb)
		return true
	}
	if cmd == "/clear" || cmd == "/wipe" || cmd == "/все" {
		reqID := int64(0)
		where := "из заявки"
		if w, _, ok := b.db.AttachWaitOf(uid); ok {
			reqID = w.RequestID
			if w.IsVault() {
				where = "из документов"
			}
		}
		n, err := b.db.DeleteOwnOnRequest(reqID, uid)
		if err != nil {
			kb := b.mainKeyboard()
			if _, _, waiting := b.db.AttachWaitOf(uid); waiting {
				kb = b.attachKeyboard()
			}
			b.sendHTML(m.Chat.ID, tgEsc(err.Error()), kb)
			return true
		}
		kb := b.mainKeyboard()
		if _, _, waiting := b.db.AttachWaitOf(uid); waiting {
			kb = b.attachKeyboard()
		}
		b.sendHTML(m.Chat.ID, "Убрал отправленное "+where+": "+strconv.Itoa(n)+".", kb)
		return true
	}
	if cmd == "/start" {
		arg := strings.TrimSpace(m.CommandArguments())
		if arg == "" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(m.Text)), "/start ") {
			parts := strings.Fields(m.Text)
			if len(parts) > 1 {
				arg = parts[1]
			}
		}
		if id := parseAttachStart(arg); id != 0 {
			rec, err := b.db.SetAttachWait(uid, id, 2*time.Hour)
			if err != nil {
				b.reply(m.Chat.ID, "Не нашёл заявку. Откройте её в Mini App и нажмите «Переслать из Telegram».")
				return true
			}
			b.sendHTML(m.Chat.ID, "Жду пересылку для заявки <b>"+tgEsc(rec.Title)+"</b> ("+tgEsc(rec.UID)+").\n\nПерешлите сюда файлы, фото или сообщения — положу в Mini App.\nКнопки внизу: «Готово», «Отменить последнее», «Убрать отправленное».", b.attachKeyboard())
			return true
		}
		if scope, owner, folder, ok := parseVaultStart(arg); ok {
			rec, err := b.db.SetVaultAttachWait(uid, scope, owner, folder, 2*time.Hour)
			if err != nil {
				b.reply(m.Chat.ID, "Не нашёл папку документов. Откройте Документы в Mini App и нажмите «Переслать из Telegram».")
				return true
			}
			b.sendHTML(m.Chat.ID, "Жду пересылку в документы <b>"+tgEsc(rec.Label)+"</b>.\n\nПерешлите сюда файлы, фото или видео — положу в Mini App.\nКнопки внизу: «Готово», «Отменить последнее», «Убрать отправленное».", b.attachKeyboard())
			return true
		}
	}

	wait, rec, ok := b.db.AttachWaitOf(uid)
	if !ok {
		if hasDealMedia(m) {
			b.sendHTML(m.Chat.ID, "Сначала в Mini App: заявка → <b>Файлы и сообщения</b> или Документы → Переслать из Telegram. Потом перешлите сюда файл.", b.mainKeyboard())
			return true
		}
		return false
	}
	if strings.HasPrefix(cmd, "/") && cmd != "/start" {
		return false
	}

	if wait.IsVault() {
		saved, err := b.ingestVaultMessage(m, wait, name, uid)
		if err != nil {
			b.replyTo(m, "Не получилось сохранить: "+tgEsc(err.Error()))
			return true
		}
		if len(saved) == 0 {
			if cmd == "/start" || strings.HasPrefix(cmd, "/") {
				return false
			}
			b.replyTo(m, "В документы нужен файл, фото или видео — текст без файла не кладу.")
			return true
		}
		ids := make([]int64, 0, len(saved))
		for _, f := range saved {
			ids = append(ids, f.ID)
		}
		b.db.RememberVaultSent(uid, ids)
		b.db.BumpInbox(appdb.SecDocuments)
		rows := [][]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Убрать отправленное", "delmine:0"),
			),
		}
		for _, f := range saved {
			label := "Удалить из документов"
			if strings.TrimSpace(f.Name) != "" {
				label = "Удалить «" + clipName(f.Name, 24) + "»"
			}
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(label, "delvf:"+strconv.FormatInt(f.ID, 10)),
			))
		}
		b.replyToMarkup(m, "В документы <b>"+tgEsc(rec.Title)+"</b> сохранил: "+strconv.Itoa(len(saved))+". Можно переслать ещё.\nОшиблись — «Убрать отправленное».", tgbotapi.NewInlineKeyboardMarkup(rows...))
		return true
	}

	saved, err := b.ingestDealMessage(m, wait.RequestID, name, uid)
	if err != nil {
		b.replyTo(m, "Не получилось сохранить: "+tgEsc(err.Error()))
		return true
	}
	if len(saved) == 0 {
		return false
	}
	ids := make([]int64, 0, len(saved))
	for _, f := range saved {
		ids = append(ids, f.ID)
	}
	b.db.RememberSent(uid, ids)
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Убрать отправленное", "delmine:"+strconv.FormatInt(wait.RequestID, 10)),
		),
	}
	for _, f := range saved {
		label := "Удалить из заявки"
		if f.Kind == "file" && strings.TrimSpace(f.Name) != "" {
			label = "Удалить «" + clipName(f.Name, 24) + "»"
		} else if f.Kind != "file" {
			label = "Удалить сообщение"
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, "deldoc:"+strconv.FormatInt(f.ID, 10)),
		))
	}
	b.replyToMarkup(m, "В заявку <b>"+tgEsc(rec.Title)+"</b> сохранил: "+strconv.Itoa(len(saved))+". Можно переслать ещё.\nОшиблись — «Убрать отправленное».", tgbotapi.NewInlineKeyboardMarkup(rows...))
	return true
}

func parseAttachStart(arg string) int64 {
	arg = strings.TrimSpace(arg)
	if strings.HasPrefix(strings.ToLower(arg), "v_") {
		return 0
	}
	arg = strings.TrimPrefix(arg, "f")
	arg = strings.TrimPrefix(arg, "_")
	arg = strings.TrimPrefix(arg, "a")
	arg = strings.TrimPrefix(arg, "_")
	id, _ := strconv.ParseInt(arg, 10, 64)
	return id
}

func parseVaultStart(arg string) (scope string, owner int64, folder string, ok bool) {
	arg = strings.TrimSpace(arg)
	low := strings.ToLower(arg)
	if !strings.HasPrefix(low, "v_") {
		return
	}
	parts := strings.Split(low, "_")
	if len(parts) < 4 {
		return
	}
	switch parts[1] {
	case "c", "co", "company":
		scope = appdb.VaultCompany
	case "p", "cp", "counterparty":
		scope = appdb.VaultCounterparty
	case "m", "misc":
		scope = appdb.VaultMisc
	default:
		return
	}
	owner, _ = strconv.ParseInt(parts[2], 10, 64)
	switch parts[3] {
	case "i", "internal":
		folder = appdb.VaultInternal
	case "e", "external":
		folder = appdb.VaultExternal
	default:
		folder = appdb.VaultOther
	}
	return scope, owner, folder, true
}

func vaultStartArg(scope string, owner int64, folder string) string {
	s, f := "m", "o"
	switch scope {
	case appdb.VaultCompany:
		s = "c"
	case appdb.VaultCounterparty:
		s = "p"
	}
	switch folder {
	case appdb.VaultInternal:
		f = "i"
	case appdb.VaultExternal:
		f = "e"
	}
	return "v_" + s + "_" + strconv.FormatInt(owner, 10) + "_" + f
}

func hasDealMedia(m *tgbotapi.Message) bool {
	if m == nil {
		return false
	}
	return m.Document != nil || len(m.Photo) > 0 || m.Video != nil || m.Audio != nil || m.Voice != nil || m.Animation != nil || m.VideoNote != nil
}

func forwardWho(m *tgbotapi.Message) string {
	if m.ForwardFrom != nil {
		n := strings.TrimSpace(m.ForwardFrom.FirstName + " " + m.ForwardFrom.LastName)
		if n != "" {
			return n
		}
	}
	if m.ForwardFromChat != nil {
		if t := strings.TrimSpace(m.ForwardFromChat.Title); t != "" {
			return t
		}
	}
	if strings.TrimSpace(m.ForwardSenderName) != "" {
		return strings.TrimSpace(m.ForwardSenderName)
	}
	return ""
}

func (b *Bot) ingestDealMessage(m *tgbotapi.Message, requestID int64, byName string, by int64) ([]appdb.DealFile, error) {
	out := []appdb.DealFile{}
	caption := strings.TrimSpace(m.Caption)
	who := forwardWho(m)
	note := strings.TrimSpace(m.Text)
	if note == "" {
		note = caption
	}
	if who != "" && note != "" {
		note = who + ":\n" + note
	} else if who != "" && note == "" {
		note = "Переслано от " + who
	}

	addFile := func(fileID, name, mime, cap string) error {
		data, pathName, err := b.downloadTGFile(fileID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(name) == "" {
			name = pathName
		}
		if strings.TrimSpace(name) == "" {
			name = "file"
		}
		rec, err := b.db.AddDealFile(requestID, name, mime, data, byName, by, cap)
		if err != nil {
			return err
		}
		out = append(out, rec)
		return nil
	}

	switch {
	case m.Document != nil:
		cap := caption
		if who != "" && cap != "" {
			cap = who + ":\n" + cap
		} else if who != "" {
			cap = "Переслано от " + who
		}
		if err := addFile(m.Document.FileID, m.Document.FileName, m.Document.MimeType, cap); err != nil {
			return out, err
		}
	case len(m.Photo) > 0:
		ph := m.Photo[len(m.Photo)-1]
		cap := caption
		if who != "" && cap != "" {
			cap = who + ":\n" + cap
		} else if who != "" {
			cap = "Переслано от " + who
		}
		if err := addFile(ph.FileID, "photo.jpg", "image/jpeg", cap); err != nil {
			return out, err
		}
	case m.Video != nil:
		if err := addFile(m.Video.FileID, "video.mp4", m.Video.MimeType, note); err != nil {
			return out, err
		}
	case m.Audio != nil:
		name := "audio"
		if m.Audio.Title != "" {
			name = m.Audio.Title
		}
		if err := addFile(m.Audio.FileID, name, m.Audio.MimeType, note); err != nil {
			return out, err
		}
	case m.Voice != nil:
		if err := addFile(m.Voice.FileID, "voice.ogg", m.Voice.MimeType, note); err != nil {
			return out, err
		}
	case m.Animation != nil:
		if err := addFile(m.Animation.FileID, "animation.mp4", m.Animation.MimeType, note); err != nil {
			return out, err
		}
	case m.VideoNote != nil:
		if err := addFile(m.VideoNote.FileID, "videonote.mp4", "video/mp4", note); err != nil {
			return out, err
		}
	default:
		if strings.TrimSpace(note) == "" {
			return out, nil
		}
		rec, err := b.db.AddDealNote(requestID, "message", note, byName, by)
		if err != nil {
			return out, err
		}
		return append(out, rec), nil
	}
	return out, nil
}

func (b *Bot) ingestVaultMessage(m *tgbotapi.Message, wait appdb.AttachWait, byName string, by int64) ([]appdb.VaultFile, error) {
	out := []appdb.VaultFile{}
	addFile := func(fileID, name, mime string) error {
		data, pathName, err := b.downloadTGFile(fileID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(name) == "" {
			name = pathName
		}
		if strings.TrimSpace(name) == "" {
			name = "file"
		}
		rec, err := b.db.AddVaultFile(wait.Scope, wait.OwnerID, wait.Folder, name, mime, data, byName, by)
		if err != nil {
			return err
		}
		out = append(out, rec)
		return nil
	}
	switch {
	case m.Document != nil:
		if err := addFile(m.Document.FileID, m.Document.FileName, m.Document.MimeType); err != nil {
			return out, err
		}
	case len(m.Photo) > 0:
		ph := m.Photo[len(m.Photo)-1]
		if err := addFile(ph.FileID, "photo.jpg", "image/jpeg"); err != nil {
			return out, err
		}
	case m.Video != nil:
		if err := addFile(m.Video.FileID, "video.mp4", m.Video.MimeType); err != nil {
			return out, err
		}
	case m.Audio != nil:
		name := "audio"
		if m.Audio.Title != "" {
			name = m.Audio.Title
		}
		if err := addFile(m.Audio.FileID, name, m.Audio.MimeType); err != nil {
			return out, err
		}
	case m.Voice != nil:
		if err := addFile(m.Voice.FileID, "voice.ogg", m.Voice.MimeType); err != nil {
			return out, err
		}
	case m.Animation != nil:
		if err := addFile(m.Animation.FileID, "animation.mp4", m.Animation.MimeType); err != nil {
			return out, err
		}
	case m.VideoNote != nil:
		if err := addFile(m.VideoNote.FileID, "videonote.mp4", "video/mp4"); err != nil {
			return out, err
		}
	default:
		return out, nil
	}
	return out, nil
}

func (b *Bot) downloadTGFile(fileID string) ([]byte, string, error) {
	f, err := b.api.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return nil, "", err
	}
	link := f.Link(b.api.Token)
	if link == "" {
		return nil, "", fmt.Errorf("нет ссылки на файл")
	}
	cli := &http.Client{Timeout: 60 * time.Second}
	resp, err := cli.Get(link)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("telegram file %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 25<<20+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > 25<<20 {
		return nil, "", fmt.Errorf("файл больше 25 МБ")
	}
	name := filepath.Base(f.FilePath)
	name = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)
	return data, name, nil
}

func (b *Bot) sendHome(chatID int64, staff bool) {
	text := "Worklet — кнопки внизу открывают разделы Mini App.\n\n<b>Открыть Worklet</b> — весь кабинет.\nЗаявки, канбан, сальдо, согласование — сразу в нужный раздел.\n<b>Отменить последнее</b> — убрать то, что только что переслали в заявку."
	if staff {
		text += "\n\nДоступы коллегам — в Mini App → Доступы."
	}
	if err := b.sendHTML(chatID, text, b.mainKeyboard()); err != nil {
		log.Printf("miniapp send home: %v", err)
	}
}

func (b *Bot) sendHTML(chatID int64, html string, kb any) error {
	msg := tgbotapi.NewMessage(chatID, html)
	msg.ParseMode = "HTML"
	if kb != nil {
		msg.ReplyMarkup = kb
	}
	_, err := b.api.Send(msg)
	return err
}

func (b *Bot) replyChatIDs(m *tgbotapi.Message) {
	uid := m.From.ID
	if m.Chat.IsGroup() || m.Chat.IsSuperGroup() || m.Chat.Type == "channel" {
		title := strings.TrimSpace(m.Chat.Title)
		if title == "" {
			title = m.Chat.UserName
		}
		if title == "" {
			title = "—"
		}
		text := fmt.Sprintf(
			"Группа: <b>%s</b>\nТип: <code>%s</code>\n<b>ID чата:</b> <code>%d</code>",
			tgEsc(title), m.Chat.Type, m.Chat.ID,
		)
		text += fmt.Sprintf("\nВаш user ID: <code>%d</code>", uid)
		b.replyTo(m, text)
		return
	}
	b.replyTo(m, fmt.Sprintf("Ваш ID: <code>%d</code>", uid))
}

func (b *Bot) replyTo(m *tgbotapi.Message, html string) {
	b.replyToMarkup(m, html, tgbotapi.InlineKeyboardMarkup{})
}

func (b *Bot) replyToMarkup(m *tgbotapi.Message, html string, markup tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewMessage(m.Chat.ID, html)
	msg.ParseMode = "HTML"
	if m.MessageID != 0 {
		msg.ReplyToMessageID = m.MessageID
	}
	if len(markup.InlineKeyboard) > 0 {
		msg.ReplyMarkup = markup
	}
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("miniapp reply: %v", err)
	}
}

func clipName(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

func clipCaption(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if n <= 0 || len(r) <= n {
		return string(r)
	}
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

func (b *Bot) handleCallback(q *tgbotapi.CallbackQuery) {
	if q == nil || q.From == nil {
		return
	}
	data := strings.TrimSpace(q.Data)
	ack := func(text string) {
		cb := tgbotapi.NewCallback(q.ID, text)
		if _, err := b.api.Request(cb); err != nil {
			log.Printf("callback ack: %v", err)
		}
	}
	if strings.HasPrefix(data, "apr:") {
		parts := strings.Split(data, ":")
		if len(parts) != 3 {
			ack("Некорректная кнопка")
			return
		}
		id, _ := strconv.ParseInt(parts[2], 10, 64)
		admin, _, _ := b.appealRights(q.From.ID)
		if !admin {
			ack("Решение принимает администратор")
			return
		}
		var rec appdb.Approval
		for _, item := range b.db.ApprovalsCopy() {
			if item.ID == id {
				rec = item
				break
			}
		}
		if rec.ID == 0 || q.Message == nil || rec.ChatID != q.Message.Chat.ID || rec.MessageID != q.Message.MessageID {
			ack("Согласование не найдено")
			return
		}
		name := strings.TrimSpace(q.From.FirstName + " " + q.From.LastName)
		if parts[1] == "dispute" {
			_, err := b.db.AddDispute("approvals", rec.UID, "Спор открыт из группы Telegram", name, q.From.ID)
			if err != nil {
				ack(err.Error())
				return
			}
			ack("Спор открыт. Продолжите обсуждение в Mini App → Согласование → Споры")
			return
		}
		updated, err := b.db.DecideApproval(id, parts[1], name)
		if err != nil {
			ack(err.Error())
			return
		}
		label := "✅ Принято"
		if updated.Status == "rejected" {
			label = "❌ Отклонено"
		}
		edit := tgbotapi.NewEditMessageText(rec.ChatID, rec.MessageID, q.Message.Text+"\n\n"+label+": "+name)
		_, _ = b.api.Send(edit)
		ack(label)
		return
	}
	if strings.HasPrefix(data, "deldoc:") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "deldoc:"), 10, 64)
		if err := b.db.DeleteDealFile(id, q.From.ID, false); err != nil {
			ack(err.Error())
			return
		}
		ack("Удалил из заявки")
		if q.Message != nil {
			edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "Удалил из заявки.")
			if _, err := b.api.Send(edit); err != nil {
				log.Printf("edit after del: %v", err)
			}
		}
		return
	}
	if strings.HasPrefix(data, "delvf:") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "delvf:"), 10, 64)
		if err := b.db.DeleteVaultFile(id, q.From.ID, false); err != nil {
			ack(err.Error())
			return
		}
		ack("Удалил из документов")
		if q.Message != nil {
			edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "Удалил из документов.")
			if _, err := b.api.Send(edit); err != nil {
				log.Printf("edit after delvf: %v", err)
			}
		}
		return
	}
	if strings.HasPrefix(data, "delmine:") {
		reqID, _ := strconv.ParseInt(strings.TrimPrefix(data, "delmine:"), 10, 64)
		where := "из заявки"
		if w, _, ok := b.db.AttachWaitOf(q.From.ID); ok && w.IsVault() {
			where = "из документов"
		}
		n, err := b.db.DeleteOwnOnRequest(reqID, q.From.ID)
		if err != nil {
			ack(err.Error())
			return
		}
		ack("Убрал отправленное: " + strconv.Itoa(n))
		if q.Message != nil {
			edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "Убрал отправленное "+where+": "+strconv.Itoa(n)+".")
			if _, err := b.api.Send(edit); err != nil {
				log.Printf("edit after delmine: %v", err)
			}
		}
		return
	}
	if strings.HasPrefix(data, "apl:") {
		parts := strings.Split(data, ":")
		if len(parts) != 3 {
			ack("")
			return
		}
		id, _ := strconv.ParseInt(parts[2], 10, 64)
		name := strings.TrimSpace(q.From.FirstName + " " + q.From.LastName)
		admin, lawyer, docs := b.appealRights(q.From.ID)
		var rec appdb.Appeal
		var err error
		if parts[1] == "t" || parts[1] == "take" {
			rec, err = b.db.TakeAppeal(id, q.From.ID, name, admin, lawyer, docs)
			if err != nil {
				ack(err.Error())
				return
			}
			ack("В работе")
			b.AnnounceAppeal(rec, "taken")
			return
		}
		if parts[1] == "c" || parts[1] == "close" {
			rec, err = b.db.CloseAppeal(id, q.From.ID, name, admin, lawyer, docs)
			if err != nil {
				ack(err.Error())
				return
			}
			ack("Закрыто")
			b.AnnounceAppeal(rec, "closed")
			return
		}
		ack("")
		return
	}
	ack("")
}

func (b *Bot) reply(chatID int64, html string) {
	var kb any
	if chatID > 0 {
		if _, _, ok := b.db.AttachWaitOf(chatID); ok {
			kb = b.attachKeyboard()
		} else {
			kb = b.mainKeyboard()
		}
	}
	if err := b.sendHTML(chatID, html, kb); err != nil {
		log.Printf("miniapp reply: %v", err)
	}
}

func (b *Bot) appealRights(uid int64) (admin, lawyer, docs bool) {
	if isOwner(uid) {
		return true, true, true
	}
	u, ok := b.db.User(uid)
	if !ok {
		return false, false, false
	}
	if u.IsAdmin() {
		return true, true, true
	}
	acc := b.db.AccessFromEmployee(uid)
	return false, acc.Has(appdb.SecAppealLawyer), acc.Has(appdb.SecAppealDocs)
}

func (b *Bot) isAppealChat(id int64) bool {
	if id == 0 {
		return false
	}
	l, d := b.db.AppealChats()
	if l == 0 {
		l = secrets.LawyerChatID
	}
	if d == 0 {
		d = secrets.DocsChatID
	}
	alts := []int64{l, d, superGroupID(l), superGroupID(d)}
	for _, known := range b.db.KnownChatIDs() {
		alts = append(alts, known, superGroupID(known))
	}
	for _, x := range alts {
		if x != 0 && (x == id || superGroupID(id) == x) {
			return true
		}
	}
	return false
}

func mentionHTML(id int64, name, username string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "сотрудник"
	}
	if username = strings.TrimSpace(username); username != "" {
		return "@" + username
	}
	return fmt.Sprintf(`<a href="tg://user?id=%d">%s</a>`, id, tgEsc(name))
}

func appealTitle(kind string) string {
	if kind == "lawyer" {
		return "юристу"
	}
	return "документалисту"
}

func (b *Bot) appealHTML(rec appdb.Appeal, ctx, username, extra string) string {
	html := "<b>Обращение к " + appealTitle(rec.Kind) + "</b>"
	if rec.UID != "" {
		html += " · <code>" + tgEsc(rec.UID) + "</code>"
	}
	html += "\n\n" + tgEsc(ctx)
	html += "\n\nОт: " + mentionHTML(rec.CreatedBy, rec.CreatedName, username)
	html += "\n\n" + tgEsc(rec.Text)
	if extra != "" {
		html += "\n\n" + extra
	}
	html += "\n\nОтветьте реплаем на это сообщение — уйдёт автору в личку. Когда закончите — «Закрыть»."
	return html
}

func (b *Bot) appealButtons(rec appdb.Appeal) *tgbotapi.InlineKeyboardMarkup {
	if rec.Status == "closed" || rec.Status == "cancelled" {
		return nil
	}
	row := []tgbotapi.InlineKeyboardButton{}
	if rec.Status != "taken" {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData("Взять в работу", fmt.Sprintf("apl:t:%d", rec.ID)))
	}
	row = append(row, tgbotapi.NewInlineKeyboardButtonData("Закрыть заявку", fmt.Sprintf("apl:c:%d", rec.ID)))
	kb := tgbotapi.NewInlineKeyboardMarkup(row)
	return &kb
}

func (b *Bot) PostAppeal(rec appdb.Appeal, ctx, username string) (int, error) {
	if rec.ChatID == 0 {
		return 0, nil
	}
	html := b.appealHTML(rec, ctx, username, "")
	mid, err := b.SendToGroupMarkup(rec.ChatID, html, b.appealButtons(rec))
	if err != nil {
		return 0, err
	}
	who := "юристу"
	if rec.Kind != "lawyer" {
		who = "документалисту"
	}
	note := "Ваше обращение ушло к " + who + ". Ответьте на это сообщение — уйдёт в их чат."
	if midNotify, nerr := b.sendChat(rec.CreatedBy, note); nerr == nil {
		b.db.SetAppealNotify(rec.ID, midNotify)
	}
	return mid, nil
}

func (b *Bot) EnsureInGroup(chatID int64) error {
	if b == nil || b.api == nil {
		return fmt.Errorf("бот не запущен")
	}
	if chatID == 0 {
		return fmt.Errorf("чат не задан")
	}
	p := tgbotapi.Params{}
	p["chat_id"] = strconv.FormatInt(chatID, 10)
	p["user_id"] = strconv.FormatInt(b.api.Self.ID, 10)
	resp, err := b.api.MakeRequest("getChatMember", p)
	if err != nil {
		return botChatErr(err)
	}
	var member struct {
		Status string `json:"status"`
	}
	if len(resp.Result) > 0 {
		_ = json.Unmarshal(resp.Result, &member)
	}
	st := strings.ToLower(strings.TrimSpace(member.Status))
	if st == "left" || st == "kicked" || st == "" {
		return fmt.Errorf("бота нет в этом чате")
	}
	return nil
}

func botChatErr(err error) error {
	if err == nil {
		return nil
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "not a member"),
		strings.Contains(s, "bot is not a member"),
		strings.Contains(s, "kicked"),
		strings.Contains(s, "chat not found"),
		strings.Contains(s, "forbidden"),
		strings.Contains(s, "bot was blocked"),
		strings.Contains(s, "have no rights"),
		strings.Contains(s, "not enough rights"),
		strings.Contains(s, "need administrator"),
		strings.Contains(s, "can't initiate"):
		return fmt.Errorf("бота нет в этом чате")
	default:
		return err
	}
}

func (b *Bot) AnnounceAppeal(rec appdb.Appeal, verb string) {
	if rec.ID == 0 {
		return
	}
	ctx := rec.RequestTitle
	if rec.RequestUID != "" {
		if ctx != "" {
			ctx = rec.RequestUID + " · " + ctx
		} else {
			ctx = rec.RequestUID
		}
	}
	if ctx == "" {
		ctx = "без заявки"
	}
	u, _ := b.db.User(rec.CreatedBy)
	extra := ""
	if verb == "taken" {
		extra = "В работе: <b>" + tgEsc(rec.TakenName) + "</b>"
		_ = b.Notify(rec.CreatedBy, "Ваше обращение к "+appealTitle(rec.Kind)+" взяли в работу: "+tgEsc(rec.TakenName)+".")
	}
	if verb == "closed" {
		extra = "Закрыта: <b>" + tgEsc(rec.ClosedName) + "</b>"
		_ = b.Notify(rec.CreatedBy, "Обращение к "+appealTitle(rec.Kind)+" закрыто.")
	}
	html := b.appealHTML(rec, ctx, u.Username, extra)
	if rec.ChatID != 0 && rec.MessageID != 0 {
		b.editGroupHTML(rec.ChatID, rec.MessageID, html, b.appealButtons(rec))
	} else if rec.ChatID != 0 {
		_, _ = b.SendToGroupMarkup(rec.ChatID, html, b.appealButtons(rec))
	}
}

func (b *Bot) editGroupHTML(chatID int64, mid int, html string, kb *tgbotapi.InlineKeyboardMarkup) {
	edit := tgbotapi.NewEditMessageText(chatID, mid, html)
	edit.ParseMode = "HTML"
	if kb != nil {
		edit.ReplyMarkup = kb
	}
	if _, err := b.api.Send(edit); err != nil {
		if alt := superGroupID(chatID); alt != 0 && alt != chatID {
			edit.ChatID = alt
			if _, err2 := b.api.Send(edit); err2 != nil {
				log.Printf("edit appeal: %v / %v", err, err2)
			}
			return
		}
		log.Printf("edit appeal: %v", err)
	}
}

func (b *Bot) handleAppealRelay(m *tgbotapi.Message) bool {
	if m == nil || m.From == nil || m.ReplyToMessage == nil {
		return false
	}
	reply := m.ReplyToMessage
	if m.Chat.IsPrivate() {
		rec, ok := b.db.AppealByNotify(m.From.ID, reply.MessageID)
		if !ok || rec.Status == "closed" || rec.Status == "cancelled" {
			return false
		}
		if rec.ChatID == 0 {
			b.reply(m.Chat.ID, "Чат ещё не задан. Напишите в Mini App.")
			return true
		}
		from := strings.TrimSpace(m.From.FirstName + " " + m.From.LastName)
		if strings.TrimSpace(m.Text) != "" {
			html := "<b>От " + tgEsc(from) + "</b>\n" + tgEsc(m.Text)
			msg := tgbotapi.NewMessage(rec.ChatID, html)
			msg.ParseMode = "HTML"
			if rec.MessageID != 0 {
				msg.ReplyToMessageID = rec.MessageID
			}
			if _, err := b.api.Send(msg); err != nil {
				b.reply(m.Chat.ID, "Не дошло в чат: "+err.Error())
				return true
			}
			b.reply(m.Chat.ID, "Ушло в чат.")
			return true
		}
		cm := tgbotapi.NewCopyMessage(rec.ChatID, m.Chat.ID, m.MessageID)
		if rec.MessageID != 0 {
			cm.ReplyToMessageID = rec.MessageID
		}
		if _, err := b.api.Send(cm); err != nil {
			b.reply(m.Chat.ID, "Не дошло в чат: "+err.Error())
			return true
		}
		b.reply(m.Chat.ID, "Ушло в чат.")
		return true
	}
	if !b.isAppealChat(m.Chat.ID) {
		return false
	}
	rec, ok := b.db.AppealByChatMsg(m.Chat.ID, reply.MessageID)
	if !ok {
		rec, ok = b.db.AppealByChatMsg(m.Chat.ID, 0)
	}
	if !ok || rec.CreatedBy == 0 || rec.CreatedBy == m.From.ID {
		return false
	}
	if rec.Status == "closed" || rec.Status == "cancelled" {
		return false
	}
	role := "Юрист"
	if rec.Kind != "lawyer" {
		role = "Документалист"
	}
	from := strings.TrimSpace(m.From.FirstName + " " + m.From.LastName)
	if strings.TrimSpace(m.Text) != "" {
		html := "<b>" + role + " · " + tgEsc(from) + "</b>\n" + tgEsc(m.Text) + "\n\nОтветьте на уведомление бота, чтобы написать в чат."
		if err := b.sendHTML(rec.CreatedBy, html, nil); err != nil {
			log.Printf("appeal relay to user: %v", err)
		}
		return true
	}
	cm := tgbotapi.NewCopyMessage(rec.CreatedBy, m.Chat.ID, m.MessageID)
	if rec.NotifyMID != 0 {
		cm.ReplyToMessageID = rec.NotifyMID
	}
	if _, err := b.api.Send(cm); err != nil {
		_ = b.Notify(rec.CreatedBy, role+" прислал файл по обращению "+rec.UID+".")
	}
	return true
}

func (b *Bot) SendToGroupMarkup(chatID int64, html string, kb *tgbotapi.InlineKeyboardMarkup) (int, error) {
	if chatID == 0 {
		return 0, fmt.Errorf("нет чата")
	}
	mid, err := b.sendChatMarkup(chatID, html, kb)
	if err == nil {
		return mid, nil
	}
	if alt := superGroupID(chatID); alt != 0 && alt != chatID {
		if mid2, err2 := b.sendChatMarkup(alt, html, kb); err2 == nil {
			return mid2, nil
		}
	}
	return 0, err
}

func (b *Bot) sendChatMarkup(chatID int64, html string, kb *tgbotapi.InlineKeyboardMarkup) (int, error) {
	msg := tgbotapi.NewMessage(chatID, html)
	msg.ParseMode = "HTML"
	if kb != nil {
		msg.ReplyMarkup = kb
	}
	sent, err := b.api.Send(msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

func (b *Bot) SendToGroup(chatID int64, html string) (int, error) {
	if chatID == 0 {
		return 0, fmt.Errorf("нет чата")
	}
	mid, err := b.sendChat(chatID, html)
	if err == nil {
		return mid, nil
	}
	if alt := superGroupID(chatID); alt != 0 && alt != chatID {
		if mid2, err2 := b.sendChat(alt, html); err2 == nil {
			return mid2, nil
		}
	}
	return 0, err
}

func (b *Bot) sendChat(chatID int64, html string) (int, error) {
	msg := tgbotapi.NewMessage(chatID, html)
	msg.ParseMode = "HTML"
	sent, err := b.api.Send(msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

func superGroupID(id int64) int64 {
	if id >= 0 {
		return 0
	}
	abs := -id
	if abs >= 1000000000000 {
		return 0
	}
	return -(1000000000000 + abs)
}

func (b *Bot) Notify(userID int64, html string) error {
	if userID == 0 {
		return nil
	}
	return b.sendHTML(userID, html, b.mainKeyboard())
}

func (b *Bot) NotifyAttach(userID int64, html string) error {
	if userID == 0 {
		return nil
	}
	return b.sendHTML(userID, html, b.attachKeyboard())
}

func (b *Bot) NotifyPlain(userID int64, text string) error {
	if userID == 0 {
		return nil
	}
	msg := tgbotapi.NewMessage(userID, text)
	_, err := b.api.Send(msg)
	return err
}

func (b *Bot) SendBytes(userID int64, name string, data []byte, caption string) error {
	if userID == 0 {
		return fmt.Errorf("нет получателя")
	}
	doc := tgbotapi.NewDocument(userID, tgbotapi.FileBytes{Name: name, Bytes: data})
	if caption != "" {
		doc.Caption = caption
	}
	_, err := b.api.Send(doc)
	return err
}

func (b *Bot) SendBytesToGroup(chatID int64, name string, data []byte, caption string) (int, error) {
	if chatID == 0 {
		return 0, fmt.Errorf("нет группы")
	}
	doc := tgbotapi.NewDocument(chatID, tgbotapi.FileBytes{Name: name, Bytes: data})
	doc.Caption = caption
	m, err := b.api.Send(doc)
	return m.MessageID, err
}

func (b *Bot) SendDealToUser(userID int64, name, mime string, data []byte, caption string) error {
	if userID == 0 {
		return fmt.Errorf("нет получателя")
	}
	if strings.TrimSpace(name) == "" {
		name = "file"
	}
	caption = clipCaption(caption, 1024)
	mime = strings.ToLower(strings.TrimSpace(mime))
	asPhoto := strings.HasPrefix(mime, "image/") && mime != "image/svg+xml" && len(data) <= 10<<20
	if asPhoto {
		ph := tgbotapi.NewPhoto(userID, tgbotapi.FileBytes{Name: name, Bytes: data})
		ph.Caption = caption
		if _, err := b.api.Send(ph); err == nil {
			return nil
		}
	}
	return b.SendBytes(userID, name, data, caption)
}

func tgEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func (b *Bot) NotifyPeers(ids []int64, except int64, html string) {
	for _, id := range ids {
		if id == 0 || id == except {
			continue
		}
		if err := b.Notify(id, html); err != nil {
			log.Printf("miniapp notify %d: %v", id, err)
		}
	}
}

func (b *Bot) CreateRequestTopic(title string) (threadID int64, link string, err error) {
	if secrets.ForumChatID == 0 {
		return 0, "", nil
	}
	params := tgbotapi.Params{}
	params.AddNonZero64("chat_id", secrets.ForumChatID)
	params.AddNonEmpty("name", title)
	resp, err := b.api.MakeRequest("createForumTopic", params)
	if err != nil {
		return 0, "", err
	}
	var out struct {
		MessageThreadID int64 `json:"message_thread_id"`
	}
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		return 0, "", err
	}
	link = fmt.Sprintf("https://t.me/c/%s/%d", trimChat(secrets.ForumChatID), out.MessageThreadID)
	return out.MessageThreadID, link, nil
}

func trimChat(id int64) string {
	s := strconv.FormatInt(id, 10)
	s = strings.TrimPrefix(s, "-100")
	return s
}
