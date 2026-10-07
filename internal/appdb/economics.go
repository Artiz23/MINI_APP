package appdb

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type CommissionTerm struct {
	Type     string `json:"type"`
	Value    string `json:"value"`
	Currency string `json:"currency"`
}
type SaleTerm struct {
	Value       string `json:"value"`
	Currency    string `json:"currency"`
	Rate        string `json:"rate"`
	Description string `json:"description"`
}
type MoneyTerm struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}
type RequestEconomics struct {
	CounterpartyCommission CommissionTerm `json:"counterparty_commission"`
	AgentCommission        CommissionTerm `json:"agent_commission"`
	ClientSale             SaleTerm       `json:"client_sale"`
	Profit                 MoneyTerm      `json:"profit"`
}
type Agent struct {
	ID          int64     `json:"id"`
	UID         string    `json:"uid"`
	Name        string    `json:"name"`
	WorkID      string    `json:"work_id"`
	CreatedBy   int64     `json:"created_by"`
	CreatedName string    `json:"created_name"`
	CreatedAt   time.Time `json:"created_at"`
}

var decimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)
var signedDecimalPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)
var currencyPattern = regexp.MustCompile(`^[A-Z]{3,8}$`)

func validateEconomics(v RequestEconomics) error {
	for _, c := range []CommissionTerm{v.CounterpartyCommission, v.AgentCommission} {
		if c.Value == "" {
			continue
		}
		if len(c.Value) > 40 || !decimalPattern.MatchString(c.Value) || (c.Type != "fixed" && c.Type != "percent") {
			return fmt.Errorf("комиссия: сумма или процент, значение — неотрицательное десятичное число")
		}
		if !currencyPattern.MatchString(c.Currency) {
			return fmt.Errorf("укажите валюту комиссии")
		}
	}
	if v.ClientSale.Value != "" && (!decimalPattern.MatchString(v.ClientSale.Value) || len(v.ClientSale.Value) > 40 || !currencyPattern.MatchString(v.ClientSale.Currency)) {
		return fmt.Errorf("укажите сумму продажи и валюту")
	}
	if v.ClientSale.Rate != "" && (!decimalPattern.MatchString(v.ClientSale.Rate) || len(v.ClientSale.Rate) > 40) {
		return fmt.Errorf("неверный курс продажи")
	}
	if v.Profit.Value != "" && (!signedDecimalPattern.MatchString(v.Profit.Value) || len(v.Profit.Value) > 40 || !currencyPattern.MatchString(v.Profit.Currency)) {
		return fmt.Errorf("укажите прибыль и валюту; убыток можно записать со знаком минус")
	}
	return nil
}
func (s *Store) SaveRequestEconomics(id, agentID, by int64, v RequestEconomics, admin bool) (Request, error) {
	if err := validateEconomics(v); err != nil {
		return Request{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if agentID != 0 {
		found := false
		for _, a := range s.Agents {
			if a.ID == agentID {
				found = true
			}
		}
		if !found {
			return Request{}, fmt.Errorf("агент не найден")
		}
	}
	for i, r := range s.Requests {
		if r.ID != id {
			continue
		}
		if !s.requestPermissionLocked(r, by, admin).CanEdit {
			return Request{}, fmt.Errorf("нет права изменять заявку")
		}
		if r.Status != "open" && r.Status != "in_progress" {
			return Request{}, fmt.Errorf("закрытая история не редактируется")
		}
		next := r
		next.AgentID = agentID
		next.Economics = v
		before, _ := json.Marshal(r.Economics)
		after, _ := json.Marshal(v)
		return s.commitRequestLocked(i, r, next, by, "economics", string(before), string(after))
	}
	return Request{}, fmt.Errorf("заявка не найдена")
}
func (s *Store) AgentsCopy() []Agent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Agent{}, s.Agents...)
}
func (s *Store) SaveAgent(in Agent, by int64, name string, admin bool) (Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return Agent{}, fmt.Errorf("укажите имя")
	}
	for _, a := range s.Agents {
		if a.ID != in.ID && strings.EqualFold(a.Name, in.Name) {
			return Agent{}, fmt.Errorf("агент уже существует")
		}
	}
	old := append([]Agent(nil), s.Agents...)
	oldID := s.NextID
	if in.ID == 0 {
		in.ID = s.nextLocked()
		in.UID = uid("agent", in.ID)
		in.CreatedBy = by
		in.CreatedName = name
		in.CreatedAt = time.Now()
		s.Agents = append(s.Agents, in)
	} else {
		if !admin {
			return Agent{}, fmt.Errorf("редактирует администратор")
		}
		found := false
		for i, a := range s.Agents {
			if a.ID == in.ID {
				a.Name = in.Name
				a.WorkID = in.WorkID
				s.Agents[i] = a
				in = a
				found = true
				break
			}
		}
		if !found {
			return Agent{}, fmt.Errorf("агент не найден")
		}
	}
	if err := s.saveLocked(); err != nil {
		s.Agents = old
		s.NextID = oldID
		return Agent{}, err
	}
	return in, nil
}
