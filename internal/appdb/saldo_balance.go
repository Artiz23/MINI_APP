package appdb

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"
)

type SaldoBalance struct {
	CounterpartyID int64     `json:"counterparty_id"`
	CP             string    `json:"cp"`
	Kind           string    `json:"kind"`
	Currency       string    `json:"currency"`
	Amount         string    `json:"amount"`
	UpdatedAt      time.Time `json:"updated_at"`
	UpdatedBy      int64     `json:"updated_by"`
}

func balanceKey(id int64, cp, kind, cur string) string {
	identity := strings.ToLower(strings.Join(strings.Fields(cp), " "))
	if id != 0 {
		identity = "id:" + strconv.FormatInt(id, 10)
	}
	return identity + "|" + kind + "|" + cur
}
func decimalFloat(v float64) *big.Rat {
	r := new(big.Rat)
	r.SetString(strconv.FormatFloat(v, 'f', -1, 64))
	return r
}
func decimalString(v *big.Rat) string {
	// Float inputs are finite decimal strings. Preserve enough decimal places
	// for their denominator, without introducing a currency rounding policy.
	d := new(big.Int).Set(v.Denom())
	two, five := big.NewInt(2), big.NewInt(5)
	a, b := 0, 0
	for new(big.Int).Mod(d, two).Sign() == 0 {
		d.Div(d, two)
		a++
	}
	for new(big.Int).Mod(d, five).Sign() == 0 {
		d.Div(d, five)
		b++
	}
	if b > a {
		a = b
	}
	s := v.FloatString(a)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s
}
func (s *Store) rebuildBalancesLocked() []SaldoBalance {
	values := map[string]*big.Rat{}
	meta := map[string]SaldoBalance{}
	for _, op := range s.SaldoOps {
		id := op.CounterpartyID
		cp := op.CP
		if id == 0 {
			for _, c := range s.Counterparties {
				if strings.EqualFold(c.Name, cp) {
					id = c.ID
					break
				}
			}
		}
		if c, ok := s.counterpartyLocked(id); ok {
			cp = c.Name
		}
		key := balanceKey(id, cp, op.Kind, op.Currency)
		if values[key] == nil {
			values[key] = new(big.Rat)
		}
		amount := decimalFloat(op.Amount)
		if op.Action == "minus" {
			amount.Neg(amount)
		}
		values[key].Add(values[key], amount)
		meta[key] = SaldoBalance{CounterpartyID: id, CP: cp, Kind: op.Kind, Currency: op.Currency, UpdatedAt: op.CreatedAt, UpdatedBy: op.ManagerID}
	}
	keys := []string{}
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []SaldoBalance{}
	for _, k := range keys {
		v := meta[k]
		v.Amount = decimalString(values[k])
		out = append(out, v)
	}
	return out
}
func (s *Store) applySaldoDeltaLocked(op SaldoOp) (SaldoOp, error) {
	if op.Action != "plus" && op.Action != "minus" {
		s.NextID = op.ID
		return SaldoOp{}, fmt.Errorf("действие: plus или minus")
	}
	if math.IsNaN(op.Amount) || math.IsInf(op.Amount, 0) || op.Amount <= 0 {
		s.NextID = op.ID
		return SaldoOp{}, fmt.Errorf("сумма должна быть положительной")
	}
	previous := s.SaldoBalances
	previousAudit := len(s.AuditEvents)
	s.SaldoOps = append(s.SaldoOps, op)
	s.SaldoBalances = s.rebuildBalancesLocked()
	s.AuditEvents = append(s.AuditEvents, AuditEvent{ID: s.nextLocked(), ActorID: op.ManagerID, EntityType: "saldo", EntityID: op.ID, Action: "delta", After: op.Action + " " + strconv.FormatFloat(op.Amount, 'f', -1, 64) + " " + op.Currency, At: time.Now()})
	if err := s.saveLocked(); err != nil {
		s.SaldoOps = s.SaldoOps[:len(s.SaldoOps)-1]
		s.SaldoBalances = previous
		s.AuditEvents = s.AuditEvents[:previousAudit]
		s.NextID = op.ID
		return SaldoOp{}, err
	}
	return op, nil
}
func (s *Store) RebuildSaldoBalances(by int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.SaldoBalances
	previousID := s.NextID
	s.SaldoBalances = s.rebuildBalancesLocked()
	s.AuditEvents = append(s.AuditEvents, AuditEvent{ID: s.nextLocked(), ActorID: by, EntityType: "saldo", Action: "rebuild", At: time.Now()})
	if err := s.saveLocked(); err != nil {
		s.SaldoBalances = old
		s.NextID = previousID
		s.AuditEvents = s.AuditEvents[:len(s.AuditEvents)-1]
		return err
	}
	return nil
}
func (s *Store) SaldoBalancesCopy() []SaldoBalance {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]SaldoBalance{}, s.SaldoBalances...)
	for i := range out {
		if c, ok := s.counterpartyLocked(out[i].CounterpartyID); ok {
			out[i].CP = c.Name
		}
	}
	return out
}

func (s *Store) saldoSnapshot() *Store {
	s.mu.Lock()
	defer s.mu.Unlock()
	copyStore := &Store{Requests: append([]Request(nil), s.Requests...), SaldoOps: append([]SaldoOp(nil), s.SaldoOps...), SaldoBalances: append([]SaldoBalance(nil), s.SaldoBalances...), Counterparties: append([]Counterparty(nil), s.Counterparties...)}
	for i := range copyStore.SaldoOps {
		if c, ok := copyStore.counterpartyLocked(copyStore.SaldoOps[i].CounterpartyID); ok {
			copyStore.SaldoOps[i].CP = c.Name
		}
	}
	for i := range copyStore.SaldoBalances {
		if c, ok := copyStore.counterpartyLocked(copyStore.SaldoBalances[i].CounterpartyID); ok {
			copyStore.SaldoBalances[i].CP = c.Name
		}
	}
	return copyStore
}
func (s *Store) saldoReconcilesLocked() bool {
	expected := s.rebuildBalancesLocked()
	if len(expected) != len(s.SaldoBalances) {
		return false
	}
	stored := map[string]string{}
	for _, b := range s.SaldoBalances {
		stored[balanceKey(b.CounterpartyID, b.CP, b.Kind, b.Currency)] = b.Amount
	}
	for _, b := range expected {
		if stored[balanceKey(b.CounterpartyID, b.CP, b.Kind, b.Currency)] != b.Amount {
			return false
		}
	}
	return true
}
