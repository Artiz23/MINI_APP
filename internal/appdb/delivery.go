package appdb

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Delivery struct {
	ChatID         int64     `json:"chat_id,omitempty"`
	MessageID      int       `json:"message_id,omitempty"`
	DeliveryStatus string    `json:"delivery_status,omitempty"`
	DeliveryError  string    `json:"delivery_error,omitempty"`
	DeliveredAt    time.Time `json:"delivered_at,omitempty"`
	DeliveryParts  []int     `json:"delivery_parts,omitempty"`
	DeliveryText   string    `json:"delivery_text,omitempty"`
	DeliverySaldo  string    `json:"delivery_saldo,omitempty"`
	DeliveryCSV    string    `json:"delivery_csv,omitempty"`
}

func (s *Store) CreateRequestApproval(requestID, by int64, name string, admin bool) (Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requestByIDLocked(requestID)
	if !ok {
		return Approval{}, fmt.Errorf("заявка не найдена")
	}
	if !s.requestPermissionLocked(r, by, admin).CanEdit {
		return Approval{}, fmt.Errorf("нет права изменять заявку")
	}
	if err := s.requireStageLocked(requestID, StageApproval); err != nil {
		return Approval{}, err
	}
	if s.Settings.ApprovalChatID == 0 {
		return Approval{}, fmt.Errorf("настройте чат согласования")
	}
	for _, a := range s.Approvals {
		if a.RequestID == requestID && a.Status == "pending" {
			return a, nil
		}
	}
	id := s.nextLocked()
	a := Approval{ID: id, UID: uid("apr", id), RequestID: requestID, RequestUID: r.UID, Preview: s.requestContextLocked(r), Status: "pending", ManagerID: by, ManagerName: name, CreatedAt: time.Now()}
	manager, _ := s.managerLocked(r.ManagerID)
	managerName := manager.Name
	if managerName == "" {
		managerName = name
	}
	text := strings.TrimSpace(r.Comment)
	if text == "" {
		text = strings.TrimSpace(r.Title)
	}
	a.Delivery = Delivery{ChatID: s.Settings.ApprovalChatID, DeliveryStatus: "pending", DeliveryText: "🆕 Заявка " + r.UID + "\n👤 Менеджер: " + managerName + "\n\n" + text}
	a.DeliverySaldo, a.DeliveryCSV = s.counterpartyExportLocked(r.CounterpartyID)
	s.Approvals = append(s.Approvals, a)
	if err := s.saveLocked(); err != nil {
		s.Approvals = s.Approvals[:len(s.Approvals)-1]
		s.NextID = id
		return Approval{}, err
	}
	return a, nil
}
func (s *Store) counterpartyExportLocked(id int64) (string, string) {
	c, _ := s.counterpartyLocked(id)
	var text strings.Builder
	text.WriteString("Сальдо:\n")
	groups := map[string][]string{}
	order := []string{}
	for _, b := range s.SaldoBalances {
		if b.CounterpartyID == id || b.CounterpartyID == 0 && strings.EqualFold(b.CP, c.Name) {
			label := KindLabel(b.Kind)
			if _, exists := groups[label]; !exists {
				order = append(order, label)
			}
			groups[label] = append(groups[label], b.Amount+" "+b.Currency)
		}
	}
	for _, label := range order {
		fmt.Fprintf(&text, "%s: %s\n", label, strings.Join(groups[label], ", "))
	}
	if len(order) == 0 {
		text.WriteString("Нет операций\n")
	}
	rows := [][]string{}
	for _, op := range s.SaldoOps {
		if op.CounterpartyID == id || op.CounterpartyID == 0 && strings.EqualFold(op.CP, c.Name) {
			rows = append(rows, []string{op.CreatedAt.In(Moscow()).Format(time.RFC3339), op.UID, s.requestLabelLocked(op.RequestID), op.Kind, op.Currency, op.Action, strconv.FormatFloat(op.Amount, 'f', -1, 64), op.Manager})
		}
	}
	return text.String(), csvString([]string{"Дата", "UID", "Заявка", "Тип", "Валюта", "Действие", "Сумма", "Сотрудник"}, rows)
}
func (s *Store) DeliveryRecord(kind string, id int64) (Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var d Delivery
	found := false
	if kind == "approval" {
		for _, a := range s.Approvals {
			if a.ID == id {
				d = a.Delivery
				found = true
			}
		}
	} else {
		for _, p := range s.Payments {
			if p.ID == id {
				d = p.Delivery
				found = true
			}
		}
	}
	if !found {
		return d, fmt.Errorf("запись не найдена")
	}
	d.DeliveryParts = append([]int(nil), d.DeliveryParts...)
	return d, nil
}
func (s *Store) SaveDelivery(kind string, id int64, d Delivery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var target *Delivery
	if kind == "approval" {
		for i := range s.Approvals {
			if s.Approvals[i].ID == id {
				target = &s.Approvals[i].Delivery
				break
			}
		}
	} else if kind == "payment" {
		for i := range s.Payments {
			if s.Payments[i].ID == id {
				target = &s.Payments[i].Delivery
				break
			}
		}
	}
	if target == nil {
		return fmt.Errorf("запись не найдена")
	}
	old := *target
	d.DeliveryParts = append([]int(nil), d.DeliveryParts...)
	*target = d
	if err := s.saveLocked(); err != nil {
		*target = old
		return err
	}
	return nil
}
