package hub

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"strings"
	"time"
)

type TelegramSender interface {
	SendToGroup(int64, string) (int, error)
	SendToGroupMarkup(int64, string, *tgbotapi.InlineKeyboardMarkup) (int, error)
	SendBytesToGroup(int64, string, []byte, string) (int, error)
}

func (a *API) deliver(kind string, id int64) error {
	a.deliveryMu.Lock()
	defer a.deliveryMu.Unlock()
	d, err := a.db.DeliveryRecord(kind, id)
	if err != nil {
		return err
	}
	if d.DeliveryStatus == "sent" {
		return nil
	}
	if d.ChatID == 0 {
		return fmt.Errorf("чат доставки не настроен")
	}
	if a.sender == nil {
		d.DeliveryStatus = "error"
		d.DeliveryError = "бот недоступен"
		if err := a.db.SaveDelivery(kind, id, d); err != nil {
			return err
		}
		return fmt.Errorf("бот недоступен")
	}
	parts := 1
	for len(d.DeliveryParts) < parts {
		var mid int
		switch len(d.DeliveryParts) {
		case 0:
			if kind == "approval" {
				kb := approvalButtons(id)
				lines := strings.SplitN(d.DeliveryText, "\n", 2)
				html := "<b>" + tgEsc(lines[0]) + "</b>"
				if len(lines) == 2 {
					html += "\n" + tgEsc(lines[1])
				}
				mid, err = a.sender.SendToGroupMarkup(d.ChatID, html+"\n\n"+tgEsc(strings.TrimSpace(d.DeliverySaldo)), &kb)
			} else {
				mid, err = a.sender.SendToGroup(d.ChatID, tgEsc(d.DeliveryText))
			}
		case 1:
			mid, err = a.sender.SendToGroup(d.ChatID, tgEsc(d.DeliverySaldo))
		case 2:
			mid, err = a.sender.SendBytesToGroup(d.ChatID, fmt.Sprintf("counterparty_%d.csv", id), []byte(d.DeliveryCSV), "Выгрузка на момент согласования")
		}
		if err != nil {
			d.DeliveryStatus = "error"
			d.DeliveryError = "Ошибка Telegram; проверьте доступ бота к группе перед повтором"
			if saveErr := a.db.SaveDelivery(kind, id, d); saveErr != nil {
				return saveErr
			}
			return fmt.Errorf("%s", d.DeliveryError)
		}
		d.DeliveryParts = append(d.DeliveryParts, mid)
		if d.MessageID == 0 {
			d.MessageID = mid
		}
		d.DeliveryStatus = "pending"
		d.DeliveryError = ""
		if len(d.DeliveryParts) == parts {
			d.DeliveryStatus = "sent"
			d.DeliveredAt = time.Now()
		}
		if err = a.db.SaveDelivery(kind, id, d); err != nil {
			return err
		}
	}
	if d.DeliveryStatus != "sent" {
		d.DeliveryStatus = "sent"
		d.DeliveryError = ""
		d.DeliveredAt = time.Now()
		return a.db.SaveDelivery(kind, id, d)
	}
	return nil
}

func approvalButtons(id int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✅ Принять", fmt.Sprintf("apr:approved:%d", id)),
		tgbotapi.NewInlineKeyboardButtonData("❌ Отклонить", fmt.Sprintf("apr:rejected:%d", id)),
		tgbotapi.NewInlineKeyboardButtonData("⚠️ Спор", fmt.Sprintf("apr:dispute:%d", id)),
	))
}
