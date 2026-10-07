package appdb

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type ManagedChat struct {
	ID        int64     `json:"id"`
	UID       string    `json:"uid"`
	Name      string    `json:"name"`
	ChatID    int64     `json:"chat_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

func (s *Store) seedManagedChatsLocked() bool {
	if len(s.Chats) > 0 {
		return false
	}
	now := time.Now()
	changed := false
	if s.LawyerChatID != 0 {
		id := s.nextLocked()
		s.Chats = append(s.Chats, ManagedChat{
			ID: id, UID: uid("chat", id), Name: "ВЭД ЮРИСТ", ChatID: s.LawyerChatID, CreatedAt: now,
		})
		changed = true
	}
	if s.DocsChatID != 0 {
		id := s.nextLocked()
		s.Chats = append(s.Chats, ManagedChat{
			ID: id, UID: uid("chat", id), Name: "Документалисты", ChatID: s.DocsChatID, CreatedAt: now,
		})
		changed = true
	}
	return changed
}

func (s *Store) ChatsCopy() []ManagedChat {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]ManagedChat{}, s.Chats...)
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func (s *Store) KnownChatIDs() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[int64]bool{}
	out := []int64{}
	add := func(id int64) {
		if id == 0 || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	for _, c := range s.Chats {
		add(c.ChatID)
	}
	add(s.LawyerChatID)
	add(s.DocsChatID)
	return out
}

func (s *Store) chatNameTakenLocked(name string, except int64) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	for _, c := range s.Chats {
		if c.ID == except {
			continue
		}
		if strings.ToLower(strings.TrimSpace(c.Name)) == name {
			return true
		}
	}
	return false
}

func (s *Store) chatTelegramTakenLocked(chatID, except int64) bool {
	if chatID == 0 {
		return false
	}
	for _, c := range s.Chats {
		if c.ID == except {
			continue
		}
		if c.ChatID == chatID {
			return true
		}
	}
	return false
}

func (s *Store) AddManagedChat(name string, chatID int64) (ManagedChat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" {
		return ManagedChat{}, fmt.Errorf("введите название чата")
	}
	if chatID == 0 {
		return ManagedChat{}, fmt.Errorf("введите ID чата Telegram")
	}
	if s.chatNameTakenLocked(name, 0) {
		return ManagedChat{}, fmt.Errorf("такое название уже есть")
	}
	if s.chatTelegramTakenLocked(chatID, 0) {
		return ManagedChat{}, fmt.Errorf("такой ID чата уже добавлен")
	}
	id := s.nextLocked()
	now := time.Now()
	c := ManagedChat{ID: id, UID: uid("chat", id), Name: name, ChatID: chatID, CreatedAt: now, UpdatedAt: now}
	s.Chats = append(s.Chats, c)
	return c, s.saveLocked()
}

func (s *Store) UpdateManagedChat(id int64, name string, chatID int64) (ManagedChat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" {
		return ManagedChat{}, fmt.Errorf("введите название чата")
	}
	if chatID == 0 {
		return ManagedChat{}, fmt.Errorf("введите ID чата Telegram")
	}
	if s.chatNameTakenLocked(name, id) {
		return ManagedChat{}, fmt.Errorf("такое название уже есть")
	}
	if s.chatTelegramTakenLocked(chatID, id) {
		return ManagedChat{}, fmt.Errorf("такой ID чата уже добавлен")
	}
	for i := range s.Chats {
		if s.Chats[i].ID != id {
			continue
		}
		old := s.Chats[i].ChatID
		s.Chats[i].Name = name
		s.Chats[i].ChatID = chatID
		s.Chats[i].UpdatedAt = time.Now()
		if old != 0 && old != chatID {
			if s.LawyerChatID == old {
				s.LawyerChatID = chatID
			}
			if s.DocsChatID == old {
				s.DocsChatID = chatID
			}
			for j := range s.Appeals {
				if s.Appeals[j].ChatID == old {
					s.Appeals[j].ChatID = chatID
				}
			}
		}
		return s.Chats[i], s.saveLocked()
	}
	return ManagedChat{}, fmt.Errorf("чат не найден")
}

func (s *Store) DeleteManagedChat(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	var gone ManagedChat
	for i := range s.Chats {
		if s.Chats[i].ID == id {
			idx = i
			gone = s.Chats[i]
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("чат не найден")
	}
	s.Chats = append(s.Chats[:idx], s.Chats[idx+1:]...)
	if gone.ChatID != 0 {
		if s.LawyerChatID == gone.ChatID {
			s.LawyerChatID = 0
		}
		if s.DocsChatID == gone.ChatID {
			s.DocsChatID = 0
		}
	}
	return s.saveLocked()
}

func (s *Store) ManagedChatByTelegram(chatID int64) (ManagedChat, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.Chats {
		if c.ChatID == chatID {
			return c, true
		}
	}
	return ManagedChat{}, false
}
