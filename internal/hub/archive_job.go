package hub

import (
	"net/http"
	"tg-bot-orh3/MINI_APP/internal/appdb"
	"time"
)

func (a *API) archiveSettings(w http.ResponseWriter, r *http.Request, s session) {
	if !s.Admin && !s.Owner {
		http.Error(w, "только администратор", 403)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{"settings": a.db.AppSettings().MonthlyArchive, "sections": appdb.ArchiveSections})
	case http.MethodPost:
		var in appdb.MonthlyArchiveSettings
		if readJSON(r, &in) != nil {
			http.Error(w, "json", 400)
			return
		}
		out, err := a.db.SetArchiveSettings(in, s.ID)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, out.MonthlyArchive)
	default:
		http.Error(w, "метод", 405)
	}
}
func (a *API) archiveNow(w http.ResponseWriter, r *http.Request, s session) {
	if !s.Admin && !s.Owner {
		http.Error(w, "только администратор", 403)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "метод", 405)
		return
	}
	v, _, err := a.db.MonthlySnapshot(time.Now(), false, s.ID)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, v)
}
