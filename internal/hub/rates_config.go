package hub

import (
	"net/http"
	"tg-bot-orh3/MINI_APP/internal/appdb"
	"tg-bot-orh3/internal/rates"
)

type rateInstrument struct {
	Source string `json:"source"`
	Symbol string `json:"symbol"`
	value  func(rates.Snapshot) (float64, string)
}

// This registry powers both the configuration choices and value resolution.
func rateInstruments() []rateInstrument {
	out := []rateInstrument{
		{"rapira", "USDT/RUB", func(s rates.Snapshot) (float64, string) { return s.Rapira, s.RapiraErr }},
		{"xe", "EUR/USD", func(s rates.Snapshot) (float64, string) { return s.XEEURUSD, s.XEErr }},
		{"investing", "USD/RUB", func(s rates.Snapshot) (float64, string) { return s.Investing, s.InvestingErr }},
	}
	for _, symbol := range []string{"USD/RUB", "EUR/RUB", "CNY/RUB", "EUR/USD"} {
		pair := symbol
		out = append(out, rateInstrument{"profinance", pair, func(s rates.Snapshot) (float64, string) {
			for _, q := range append(s.Rub, s.Forex...) {
				if q.Pair == pair {
					return q.Bid, s.PFErr
				}
			}
			return 0, "Котировка недоступна"
		}})
	}
	for _, code := range []string{"USD", "EUR", "CNY", "AED"} {
		currency := code
		out = append(out, rateInstrument{"cbr", currency + "/RUB", func(s rates.Snapshot) (float64, string) {
			for _, q := range s.CBR {
				if q.CharCode == currency {
					return q.PerUnit, s.CBRErr
				}
			}
			return 0, "Котировка недоступна"
		}})
	}
	return out
}
func (a *API) rateOptions(w http.ResponseWriter, r *http.Request, s session) {
	if !s.requireSection(w, appdb.SecRates) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "метод", 405)
		return
	}
	writeJSON(w, rateInstruments())
}
func (a *API) rateConfig(w http.ResponseWriter, r *http.Request, s session) {
	if !s.Admin && !s.Owner {
		http.Error(w, "только администратор", 403)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "метод", 405)
		return
	}
	var in struct {
		Slots []appdb.RateSlot `json:"slots"`
	}
	if readJSON(r, &in) != nil || len(in.Slots) != 3 {
		http.Error(w, "нужно ровно три слота", 400)
		return
	}
	var slots [3]appdb.RateSlot
	for i, v := range in.Slots {
		if v.Enabled {
			valid := false
			for _, opt := range rateInstruments() {
				if opt.Source == v.Source && opt.Symbol == v.Symbol {
					valid = true
				}
			}
			if !valid {
				http.Error(w, "инструмент не поддерживается", 400)
				return
			}
		}
		slots[i] = v
	}
	v, err := a.db.SetRateSlots(slots, s.ID)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, v.CustomRateSlots)
}
func customRates(snap rates.Snapshot, slots [3]appdb.RateSlot) []map[string]any {
	out := []map[string]any{}
	for _, slot := range slots {
		if !slot.Enabled {
			continue
		}
		v := 0.0
		err := "инструмент недоступен"
		for _, opt := range rateInstruments() {
			if opt.Source == slot.Source && opt.Symbol == slot.Symbol {
				v, err = opt.value(snap)
				break
			}
		}
		out = append(out, map[string]any{"label": slot.Label, "source": slot.Source, "symbol": slot.Symbol, "value": v, "error": err})
	}
	return out
}
