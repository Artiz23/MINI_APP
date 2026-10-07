package appdb

import (
	"encoding/json"
	"fmt"
	"time"
)

type RateSlot struct {
	Enabled bool   `json:"enabled"`
	Source  string `json:"source"`
	Symbol  string `json:"symbol"`
	Label   string `json:"label"`
}
type MonthlyArchiveSettings struct {
	Enabled      bool      `json:"enabled"`
	Day          int       `json:"day"`
	TimeHHMM     string    `json:"time_hhmm"`
	Sections     []string  `json:"sections"`
	IncludeFiles bool      `json:"include_files"`
	LastRunMonth string    `json:"last_run_month"`
	UpdatedBy    int64     `json:"updated_by"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type MiniAppSettings struct {
	ApprovalChatID  int64                  `json:"approval_chat_id"`
	PaymentChatID   int64                  `json:"payment_chat_id"`
	SaldoChatID     int64                  `json:"saldo_chat_id"`
	MonthlyArchive  MonthlyArchiveSettings `json:"monthly_archive"`
	CustomRateSlots [3]RateSlot            `json:"custom_rate_slots"`
}

func (s *Store) AppSettings() MiniAppSettings {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.Settings
	v.MonthlyArchive.Sections = append([]string(nil), v.MonthlyArchive.Sections...)
	return v
}

func (s *Store) SetRateSlots(slots [3]RateSlot, by int64) (MiniAppSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.Settings
	next.CustomRateSlots = slots
	return s.saveSettingsLocked(next, by, "rate_slots")
}
func (s *Store) SetChat(scope string, id, by int64) (MiniAppSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id > 0 {
		return MiniAppSettings{}, fmt.Errorf("укажите отрицательный ID группы Telegram")
	}
	if id != 0 {
		found := false
		for _, chat := range s.Chats {
			if chat.ChatID == id {
				found = true
				break
			}
		}
		if !found {
			return MiniAppSettings{}, fmt.Errorf("выберите группу, созданную в разделе «Чаты»")
		}
	}
	next := s.Settings
	switch scope {
	case "approval":
		next.ApprovalChatID = id
	case "payment":
		next.PaymentChatID = id
	case "saldo":
		next.SaldoChatID = id
	default:
		return next, fmt.Errorf("неизвестная настройка")
	}
	if next.ApprovalChatID != 0 && next.ApprovalChatID == next.PaymentChatID {
		return next, fmt.Errorf("чат оплаты должен отличаться от чата согласования")
	}
	return s.saveSettingsLocked(next, by, "chat_"+scope)
}
func (s *Store) saveSettingsLocked(next MiniAppSettings, by int64, action string) (MiniAppSettings, error) {
	old := s.Settings
	previousID := s.NextID
	before, _ := json.Marshal(old)
	after, _ := json.Marshal(next)
	s.Settings = next
	s.AuditEvents = append(s.AuditEvents, AuditEvent{ID: s.nextLocked(), ActorID: by, EntityType: "settings", Action: action, Before: string(before), After: string(after), At: time.Now()})
	if err := s.saveLocked(); err != nil {
		s.Settings = old
		s.NextID = previousID
		s.AuditEvents = s.AuditEvents[:len(s.AuditEvents)-1]
		return old, err
	}
	return next, nil
}
