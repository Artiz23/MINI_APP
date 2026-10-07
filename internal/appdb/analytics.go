package appdb

import (
	"fmt"
	"math/big"
	"sort"
	"time"
)

type AnalyticsRow struct {
	ID                        int64             `json:"id"`
	Name                      string            `json:"name"`
	Requests                  int               `json:"requests"`
	InProgress                int               `json:"in_progress"`
	Successful                int               `json:"successful"`
	Failed                    int               `json:"failed"`
	Conversion                float64           `json:"conversion"`
	Sales                     map[string]string `json:"sales"`
	CounterpartyCommission    map[string]string `json:"counterparty_commission"`
	AgentCommission           map[string]string `json:"agent_commission"`
	Profit                    map[string]string `json:"profit"`
	PercentageTerms           int               `json:"percentage_terms"`
	ClientCommissionReminders int               `json:"client_commission_reminders"`
	ActivityHours             float64           `json:"activity_hours"`
	AverageCloseHours         *float64          `json:"average_close_hours"`
	closeHours                float64
	closeCount                int
}

func addDecimal(m map[string]string, currency, amount string) {
	if amount == "" {
		return
	}
	n, ok := new(big.Rat).SetString(amount)
	if !ok {
		return
	}
	old := new(big.Rat)
	if m[currency] != "" {
		old.SetString(m[currency])
	}
	m[currency] = decimalString(old.Add(old, n))
}
func (s *Store) Analytics(dimension string, entityID int64, from, to time.Time) ([]AnalyticsRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch dimension {
	case "employee", "client", "counterparty", "agent":
	default:
		return nil, fmt.Errorf("неверный объект аналитики")
	}
	rows := map[int64]*AnalyticsRow{}
	get := func(id int64) *AnalyticsRow {
		if rows[id] != nil {
			return rows[id]
		}
		name := "Без привязки"
		switch dimension {
		case "employee":
			if v, ok := s.employeeLocked(id); ok {
				name = v.Name
			}
		case "client":
			if v, ok := s.clientLocked(id); ok {
				name = v.Name
			}
		case "counterparty":
			if v, ok := s.counterpartyLocked(id); ok {
				name = v.Name
			}
		case "agent":
			for _, v := range s.Agents {
				if v.ID == id {
					name = v.Name
				}
			}
		}
		r := &AnalyticsRow{ID: id, Name: name, Sales: map[string]string{}, CounterpartyCommission: map[string]string{}, AgentCommission: map[string]string{}, Profit: map[string]string{}}
		rows[id] = r
		return r
	}
	for _, r := range s.Requests {
		if r.Status == "deleted" || r.CreatedAt.Before(from) || !r.CreatedAt.Before(to) {
			continue
		}
		id := r.EmployeeID
		switch dimension {
		case "client":
			id = r.ClientID
		case "counterparty":
			id = r.CounterpartyID
		case "agent":
			id = r.AgentID
		}
		if entityID != 0 && entityID != id {
			continue
		}
		v := get(id)
		v.Requests++
		switch r.Status {
		case "in_progress":
			v.InProgress++
		case "failed":
			v.Failed++
		case "success_closed":
			v.Successful++
			addDecimal(v.Sales, r.Economics.ClientSale.Currency, r.Economics.ClientSale.Value)
			addDecimal(v.Profit, r.Economics.Profit.Currency, r.Economics.Profit.Value)
			for i, c := range []CommissionTerm{r.Economics.CounterpartyCommission, r.Economics.AgentCommission} {
				if c.Value == "" {
					continue
				}
				if c.Type == "percent" {
					v.PercentageTerms++
					continue
				}
				target := v.CounterpartyCommission
				if i == 1 {
					target = v.AgentCommission
				}
				addDecimal(target, c.Currency, c.Value)
			}
		}
		if !r.ClosedAt.IsZero() && !r.ClosedAt.Before(r.CreatedAt) {
			v.closeHours += r.ClosedAt.Sub(r.CreatedAt).Hours()
			v.closeCount++
		}
	}
	if dimension == "employee" {
		for _, b := range s.ActivitySlices {
			at, err := time.Parse(time.RFC3339, b.Bucket)
			if err != nil || at.Before(from) || !at.Before(to) || entityID != 0 && entityID != b.EmployeeID {
				continue
			}
			get(b.EmployeeID).ActivityHours += 5.0 / 60
		}
	}
	for _, p := range s.Payments {
		if p.Kind != "client_commission" || p.CreatedAt.Before(from) || !p.CreatedAt.Before(to) {
			continue
		}
		id := p.EmployeeID
		switch dimension {
		case "client":
			id = p.ClientID
		case "counterparty":
			id = p.CounterpartyID
		case "agent":
			if r, ok := s.requestByIDLocked(p.RequestID); ok {
				id = r.AgentID
			}
		}
		if entityID == 0 || entityID == id {
			get(id).ClientCommissionReminders++
		}
	}
	out := []AnalyticsRow{}
	for _, v := range rows {
		if v.Successful+v.Failed > 0 {
			v.Conversion = 100 * float64(v.Successful) / float64(v.Successful+v.Failed)
		}
		if v.closeCount > 0 {
			n := v.closeHours / float64(v.closeCount)
			v.AverageCloseHours = &n
		}
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
