package appdb

import "time"

func (s *Store) ensureClientCommissionLocked(r Request, by int64) {
	c, ok := s.clientLocked(r.ClientID)
	if !ok || !c.CommissionEnabled {
		return
	}
	for _, p := range s.Payments {
		if p.RequestID == r.ID && p.Kind == "client_commission" {
			return
		}
	}
	id := s.nextLocked()
	text := "Оплата комиссия клиента\n" + s.requestContextLocked(r)
	p := Payment{ID: id, UID: uid("pay", id), RequestID: r.ID, Kind: "client_commission", Auto: true, Text: text, Status: "pending", CreatedBy: by, CreatedName: r.CreatedName, CreatedAt: time.Now(), EmployeeID: r.EmployeeID, ManagerID: r.ManagerID, ClientID: r.ClientID, CounterpartyID: r.CounterpartyID}
	p.Delivery = Delivery{ChatID: s.Settings.PaymentChatID, DeliveryStatus: "pending", DeliveryText: text}
	s.Payments = append(s.Payments, p)
}
