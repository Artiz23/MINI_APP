package hub

import "net/http"

func (a *API) appSettings(w http.ResponseWriter, r *http.Request, s session) {
	if !s.Admin && !s.Owner {
		http.Error(w, "только администратор", 403)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, a.db.AppSettings())
	case http.MethodPost:
		var in struct {
			Scope  string `json:"scope"`
			ChatID int64  `json:"chat_id"`
		}
		if readJSON(r, &in) != nil {
			http.Error(w, "json", 400)
			return
		}
		v, err := a.db.SetChat(in.Scope, in.ChatID, s.ID)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, v)
	default:
		http.Error(w, "метод", 405)
	}
}
