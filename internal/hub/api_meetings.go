package hub

import (
	"net/http"
	"strconv"
	"tg-bot-orh3/MINI_APP/internal/appdb"
	"time"
)

func (a *API) meetings(w http.ResponseWriter, r *http.Request, s session) {
	if !s.requireSection(w, appdb.SecMeetings) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		filter := r.URL.Query().Get("filter")
		if filter == "" {
			filter = "upcoming"
		}
		if filter != "upcoming" && filter != "past" && filter != "all" {
			http.Error(w, "неверный фильтр", 400)
			return
		}
		writeJSON(w, a.db.MeetingsList(filter, time.Now()))
	case http.MethodPost:
		var in appdb.Meeting
		if readJSON(r, &in) != nil {
			http.Error(w, "json: дата должна содержать часовой пояс", 400)
			return
		}
		if in.ID != 0 {
			for _, m := range a.db.MeetingsList("all", time.Now()) {
				if m.ID == in.ID && m.CreatedBy != s.ID && !s.Admin && !s.Owner {
					http.Error(w, "нет права изменять встречу", 403)
					return
				}
			}
		}
		v, err := a.db.SaveMeeting(in, s.ID, s.Name, s.Admin || s.Owner)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, v)
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		for _, m := range a.db.MeetingsList("all", time.Now()) {
			if m.ID == id && m.CreatedBy != s.ID && !s.Admin && !s.Owner {
				http.Error(w, "нет права удалять встречу", 403)
				return
			}
		}
		if err := a.db.DeleteMeeting(id, s.ID, s.Admin || s.Owner); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	default:
		http.Error(w, "метод", 405)
	}
}
