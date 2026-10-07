package hub

import (
	"net/http"
	"tg-bot-orh3/MINI_APP/internal/appdb"
)

func (a *API) agents(w http.ResponseWriter, r *http.Request, s session) {
	if !s.canSection(appdb.SecDirectory) && !s.canSection(appdb.SecRequests) {
		http.Error(w, "нет доступа", 403)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, a.db.AgentsCopy())
	case http.MethodPost:
		if !s.requireSection(w, appdb.SecDirectory) {
			return
		}
		var in appdb.Agent
		if readJSON(r, &in) != nil {
			http.Error(w, "json", 400)
			return
		}
		if in.ID != 0 && !s.Admin && !s.Owner {
			http.Error(w, "редактирует администратор", 403)
			return
		}
		v, err := a.db.SaveAgent(in, s.ID, s.Name, s.Admin || s.Owner)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, v)
	default:
		http.Error(w, "метод", 405)
	}
}
