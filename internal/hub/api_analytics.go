package hub

import (
	"net/http"
	"strconv"
	"tg-bot-orh3/MINI_APP/internal/appdb"
	"time"
)

func (a *API) activityPing(w http.ResponseWriter, r *http.Request, s session) {
	if r.Method != http.MethodPost {
		http.Error(w, "метод", 405)
		return
	}
	if err := a.db.RecordActivity(s.ID, time.Now()); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
func (a *API) analytics(w http.ResponseWriter, r *http.Request, s session) {
	if !s.requireSection(w, appdb.SecAnalytics) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "метод", 405)
		return
	}
	from, err := time.ParseInLocation("2006-01-02", r.URL.Query().Get("from"), appdb.Moscow())
	if err != nil {
		http.Error(w, "начало периода: YYYY-MM-DD", 400)
		return
	}
	to, err := time.ParseInLocation("2006-01-02", r.URL.Query().Get("to"), appdb.Moscow())
	if err != nil || to.Before(from) {
		http.Error(w, "неверный конец периода", 400)
		return
	}
	to = to.AddDate(0, 0, 1)
	id, _ := strconv.ParseInt(r.URL.Query().Get("entity_id"), 10, 64)
	rows, err := a.db.Analytics(r.URL.Query().Get("dimension"), id, from, to)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, map[string]any{"rows": rows, "profit_available": true, "profit_note": "Прибыль указана сотрудниками вручную и суммируется отдельно по валютам", "period_basis": "заявки, созданные за период; активность за период", "currency_policy": "суммы отдельно по валютам"})
}
