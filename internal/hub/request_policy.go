package hub

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"tg-bot-orh3/MINI_APP/internal/appdb"
)

// Authorize both existing and submitted references before mutating child records.
func (a *API) guardRequestMutation(w http.ResponseWriter, r *http.Request, s session) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	kind := strings.TrimPrefix(r.URL.Path, "/api/")
	switch kind {
	case "requests", "approvals", "payments", "tasks", "appeals", "compliance", "deal-files", "saldo", "disputes", "attach":
	default:
		return true
	}
	var in struct {
		ID         int64  `json:"id"`
		RequestID  int64  `json:"request_id"`
		CloseID    int64  `json:"close_id"`
		RequestUID string `json:"request_uid"`
		Ref        string `json:"ref"`
		RefUID     string `json:"ref_uid"`
		Action     string `json:"action"`
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		r.Body = http.MaxBytesReader(w, r.Body, 25<<20)
		if err := r.ParseMultipartForm(25 << 20); err != nil {
			http.Error(w, "неверная форма или файл больше 25 МБ", 400)
			return false
		}
		in.RequestID, _ = strconv.ParseInt(r.FormValue("request_id"), 10, 64)
	} else if r.Body != nil && r.Method != http.MethodDelete {
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
		if err != nil {
			http.Error(w, "слишком большой запрос", 413)
			return false
		}
		r.Body = io.NopCloser(bytes.NewReader(data))
		if len(data) > 0 && json.Unmarshal(data, &in) != nil {
			http.Error(w, "json", 400)
			return false
		}
	}
	if in.ID == 0 {
		in.ID, _ = strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	}
	if kind == "saldo" && in.CloseID != 0 {
		in.ID = in.CloseID
	}
	for _, ref := range []string{in.RequestUID, in.Ref, in.RefUID} {
		for _, id := range a.db.MutationRequestIDs(kind, in.ID, in.RequestID, ref, s.ID) {
			p, err := a.db.RequestPermissions(id, s.ID, s.Admin || s.Owner)
			if err != nil || !p.CanView {
				http.Error(w, "заявка не найдена", 404)
				return false
			}
			if !p.CanEdit {
				http.Error(w, "чужая заявка доступна только для просмотра", 403)
				return false
			}
			if in.ID == 0 && r.Method == http.MethodPost && in.Action != "retry_delivery" {
				stage := map[string]appdb.RequestStage{"approvals": appdb.StageApproval, "payments": appdb.StagePayment, "tasks": appdb.StageTask, "appeals": appdb.StageAppeal}[kind]
				if stage != "" {
					if err := a.db.RequireStage(id, stage); err != nil {
						http.Error(w, err.Error(), 409)
						return false
					}
				}
			}
		}
	}
	return true
}
