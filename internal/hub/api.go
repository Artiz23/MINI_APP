package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"tg-bot-orh3/MINI_APP/internal/appdb"
	"tg-bot-orh3/MINI_APP/internal/secrets"
	"tg-bot-orh3/internal/config"
	"tg-bot-orh3/internal/holidays"
	"tg-bot-orh3/internal/rates"
	"tg-bot-orh3/internal/sources"
)

type API struct {
	deliveryMu sync.Mutex
	sender     TelegramSender
	db         *appdb.Store
	bot        *Bot
	webDir     string
	holidays   *holidays.Service
	pf         *sources.ProfinanceClient
	cbr        *sources.CBRClient
	xe         *sources.XEClient
	investing  *sources.InvestingClient
	rapira     *sources.RapiraClient
}

func NewAPI(db *appdb.Store, bot *Bot, webDir string) *API {
	httpClient := sources.NewHTTPClient()
	var hs *holidays.Service
	loc, _ := time.LoadLocation("Europe/Moscow")
	if loc == nil {
		loc = time.FixedZone("MSK", 3*3600)
	}
	if s, err := holidays.NewService(config.HolidaysConfig{WarnDays: []int{5}}, loc, httpClient); err == nil {
		hs = s
	} else {
		log.Printf("miniapp holidays: %v", err)
	}
	var sender TelegramSender
	if bot != nil {
		sender = bot
	}
	return &API{
		sender: sender,
		db:     db, bot: bot, webDir: webDir, holidays: hs,
		pf:        sources.NewProfinance(httpClient),
		cbr:       sources.NewCBR(httpClient),
		xe:        sources.NewXE(httpClient),
		investing: sources.NewInvesting(httpClient),
		rapira:    sources.NewRapira(httpClient),
	}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/app-settings", a.withUser(a.appSettings))
	mux.HandleFunc("/api/meetings", a.withUser(a.meetings))
	mux.HandleFunc("/api/agents", a.withUser(a.agents))
	mux.HandleFunc("/api/activity/ping", a.withUser(a.activityPing))
	mux.HandleFunc("/api/analytics", a.withUser(a.analytics))
	mux.HandleFunc("/api/archive-settings", a.withUser(a.archiveSettings))
	mux.HandleFunc("/api/archive-now", a.withUser(a.archiveNow))
	mux.HandleFunc("/api/home", a.withUser(a.home))
	mux.HandleFunc("/api/me", a.withUser(a.me))
	mux.HandleFunc("/api/summary", a.withUser(a.summary))
	mux.HandleFunc("/api/users", a.withUser(a.users))
	mux.HandleFunc("/api/requests", a.withUser(a.requests))
	mux.HandleFunc("/api/requests/archive", a.withUser(a.archiveRequest))
	mux.HandleFunc("/api/approvals", a.withUser(a.approvals))
	mux.HandleFunc("/api/saldo", a.withUser(a.saldo))
	mux.HandleFunc("/api/rates", a.withUser(a.getRates))
	mux.HandleFunc("/api/rates/options", a.withUser(a.rateOptions))
	mux.HandleFunc("/api/rates/config", a.withUser(a.rateConfig))
	mux.HandleFunc("/api/holidays", a.withUser(a.getHolidays))
	mux.HandleFunc("/api/balance", a.withUser(a.balance))
	mux.HandleFunc("/api/compliance", a.withUser(a.compliance))
	mux.HandleFunc("/api/disputes", a.withUser(a.disputes))
	mux.HandleFunc("/api/inbox", a.withUser(a.inbox))
	mux.HandleFunc("/api/notify", a.withUser(a.notifyPrefs))
	mux.HandleFunc("/api/directory", a.withUser(a.directory))
	mux.HandleFunc("/api/payments", a.withUser(a.payments))
	mux.HandleFunc("/api/deal-files", a.withUser(a.dealFiles))
	mux.HandleFunc("/api/appeals", a.withUser(a.appeals))
	mux.HandleFunc("/api/tasks", a.withUser(a.tasks))
	mux.HandleFunc("/api/attach", a.withUser(a.attach))
	mux.HandleFunc("/api/documents", a.withUser(a.documents))
	mux.HandleFunc("/api/chats", a.withUser(a.chats))
	mux.Handle("/", a.static())
	return mux
}

func (a *API) static() http.Handler {
	fs := http.FileServer(http.Dir(a.webDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			http.ServeFile(w, r, filepath.Join(a.webDir, "index.html"))
			return
		}
		if _, err := os.Stat(filepath.Join(a.webDir, r.URL.Path)); err == nil {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(a.webDir, "index.html"))
	})
}

type ctxKey int

const userKey ctxKey = 1

type session struct {
	ID       int64
	Name     string
	Username string
	Owner    bool
	Admin    bool
	Role     appdb.Role
	Access   appdb.Access
	User     appdb.User
}

func (s session) canSection(sec string) bool {
	if s.Owner || s.Admin {
		return true
	}
	return s.Access.Has(sec)
}

func (s session) canManageUsers() bool {
	return s.Owner || s.Admin
}

func (s session) canPayments() bool {
	return s.Owner || s.Admin || s.canSection(appdb.SecPayments)
}

func (s session) canAppeals() bool {
	return s.canSection(appdb.SecAppeals) || s.canSection(appdb.SecAppealLawyer) || s.canSection(appdb.SecAppealDocs)
}

func (s session) allowedSections() []string {
	out := make([]string, 0, len(appdb.SectionKeys))
	for _, k := range appdb.SectionKeys {
		if k == appdb.SecPayments {
			if s.canPayments() {
				out = append(out, k)
			}
			continue
		}
		if s.canSection(k) {
			out = append(out, k)
		}
	}
	return out
}

func deny(w http.ResponseWriter, msg string, code int) {
	http.Error(w, msg, code)
}

func (a *API) withUser(fn func(http.ResponseWriter, *http.Request, session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initData := r.Header.Get("X-Telegram-Init-Data")
		if initData == "" {
			initData = r.URL.Query().Get("initData")
		}
		u, err := checkInitData(initData, secrets.BotToken)
		if err != nil {
			http.Error(w, "нужно открыть из Telegram Mini App", http.StatusUnauthorized)
			return
		}
		if isOwner(u.ID) {
			a.db.UpsertUser(appdb.User{ID: u.ID, Name: u.Display(), Username: u.Username, Role: appdb.RoleOwner, Access: appdb.AllAccess()})
		}
		rec, ok := a.db.User(u.ID)
		if !ok && !isOwner(u.ID) {
			http.Error(w, "нет доступа", http.StatusForbidden)
			return
		}
		role := strings.ToLower(strings.TrimSpace(string(rec.Role)))
		if isOwner(u.ID) || role == "owner" {
			rec.Role = appdb.RoleOwner
			rec.Access = appdb.AllAccess()
		} else if role == "admin" {
			rec.Role = appdb.RoleAdmin
			rec.Access = appdb.AllAccess()
		} else {
			rec.Access = a.db.AccessFromEmployee(u.ID)
		}
		owner := isOwner(u.ID) || rec.Role == appdb.RoleOwner
		admin := owner || rec.Role == appdb.RoleAdmin
		if !owner && !admin {
			if _, emp := a.db.EmployeeByTelegram(u.ID); !emp {
				http.Error(w, "Mini App только для сотрудников. Админ добавит вас в Справочник → Сотрудники.", http.StatusForbidden)
				return
			}
		}
		s := session{
			ID: u.ID, Name: u.Display(), Username: u.Username,
			Owner: owner, Admin: admin, Role: rec.Role, Access: rec.Access.Copy(), User: rec,
		}
		if s.Name == "" {
			s.Name = rec.Name
		}
		if !a.guardRequestMutation(w, r, s) {
			return
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		fn(w, r.WithContext(context.WithValue(r.Context(), userKey, s)), s)
	}
}

func (s session) requireSection(w http.ResponseWriter, sec string) bool {
	if s.canSection(sec) {
		return true
	}
	deny(w, "нет доступа к разделу", http.StatusForbidden)
	return false
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, v)
}

func (a *API) home(w http.ResponseWriter, _ *http.Request, _ session) {
	writeJSON(w, a.db.HomeDash())
}

func (a *API) me(w http.ResponseWriter, _ *http.Request, s session) {
	unread := a.db.Unread(s.ID, s.Admin)
	var empJSON any
	if e, ok := a.db.EmployeeByTelegram(s.ID); ok {
		empJSON = e
	}
	writeJSON(w, map[string]any{
		"id":                  s.ID,
		"name":                s.Name,
		"owner":               s.Owner,
		"admin":               s.Admin,
		"staff":               s.Owner || s.Admin,
		"see_all":             s.Owner || s.Admin,
		"limited":             !s.Owner && !s.Admin,
		"role":                s.Role,
		"access":              s.Access,
		"sections":            s.allowedSections(),
		"can_manage_users":    s.canManageUsers(),
		"can_payments":        s.canPayments(),
		"balance_responsible": s.User.BalanceResponsible,
		"web_app_url":         secrets.WebAppURL,
		"forum_chat_id":       secrets.ForumChatID,
		"unread":              unread,
		"notify":              appdb.NormalizeNotify(s.User.Notify),
		"employee":            empJSON,
		"bot_username":        a.botName(),
		"dir_warn":            a.db.OrphanPositionWarn(),
	})
}

func (a *API) summary(w http.ResponseWriter, _ *http.Request, s session) {
	if !s.Owner {
		http.Error(w, "только владелец", http.StatusForbidden)
		return
	}
	writeJSON(w, a.db.Summary())
}

func (a *API) users(w http.ResponseWriter, r *http.Request, s session) {
	if !s.canManageUsers() {
		http.Error(w, "только админ", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, a.db.UsersCopy())
	case http.MethodPost:
		var in struct {
			ID                 int64        `json:"id"`
			Name               string       `json:"name"`
			Role               string       `json:"role"`
			Access             appdb.Access `json:"access"`
			BalanceResponsible bool         `json:"balance_responsible"`
			Dirs               []string     `json:"dirs"`
		}
		if err := readJSON(r, &in); err != nil || in.ID == 0 {
			http.Error(w, "нужен id", http.StatusBadRequest)
			return
		}
		if isOwner(in.ID) || (in.Role == "owner") {
			if !s.Owner {
				http.Error(w, "владельца нельзя менять", http.StatusForbidden)
				return
			}
			u, err := a.db.SaveUser(appdb.User{
				ID: in.ID, Name: in.Name, Role: appdb.RoleOwner, Access: appdb.AllAccess(),
				BalanceResponsible: in.BalanceResponsible, Dirs: in.Dirs,
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, u)
			return
		}
		target, ok := a.db.User(in.ID)
		newUser := !ok
		if ok && target.Role == appdb.RoleOwner {
			http.Error(w, "у владельца нельзя менять доступ", http.StatusForbidden)
			return
		}
		role := appdb.RoleOperator
		if in.Role == "admin" {
			role = appdb.RoleAdmin
		}
		acc := in.Access
		if len(acc) == 0 {
			acc = appdb.DefaultAccess()
		}
		u, err := a.db.SaveUser(appdb.User{
			ID: in.ID, Name: in.Name, Role: role, Access: acc,
			BalanceResponsible: in.BalanceResponsible, Dirs: in.Dirs,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if newUser {
			a.db.BumpInbox("users")
		}
		writeJSON(w, u)
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if isOwner(id) {
			http.Error(w, "у владельца нельзя забрать доступ", http.StatusForbidden)
			return
		}
		target, ok := a.db.User(id)
		if ok && target.Role == appdb.RoleOwner {
			http.Error(w, "у владельца нельзя забрать доступ", http.StatusForbidden)
			return
		}
		if err := a.db.RemoveUser(id); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		writeJSON(w, a.db.UsersCopy())
	default:
		http.Error(w, "метод", http.StatusMethodNotAllowed)
	}
}

func (a *API) directory(w http.ResponseWriter, r *http.Request, s session) {
	canRead := s.canSection(appdb.SecDirectory) || s.canSection(appdb.SecRequests) || s.canSection(appdb.SecSaldo) || s.canSection(appdb.SecCompliance)
	if !canRead && !s.Owner {
		deny(w, "нет доступа", http.StatusForbidden)
		return
	}
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	switch r.Method {
	case http.MethodGet:
		if kind == "" {
			writeJSON(w, map[string]any{
				"employees":      a.db.EmployeesCopy(),
				"managers":       a.db.ManagersCopy(),
				"clients":        a.db.ClientsCopy(),
				"counterparties": a.db.CounterpartiesCopy(),
				"positions":      a.db.PositionsCopy(),
				"access_pool":    a.db.AccessPool(),
				"dir_warn":       a.db.OrphanPositionWarn(),
			})
			return
		}
		list, err := a.db.CatalogGet(kind)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, list)
	case http.MethodPost, http.MethodDelete:
		isAdmin := s.Owner || s.Admin
		hasDir := s.canSection(appdb.SecDirectory)
		relKind := func(k string) bool {
			return k == "managers" || k == "clients" || k == "counterparties"
		}
		if r.Method == http.MethodDelete {
			if !isAdmin && !hasDir {
				http.Error(w, "нет доступа", http.StatusForbidden)
				return
			}
			id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
			if err := a.db.CatalogDelete(kind, id, s.ID, isAdmin); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		var in appdb.CatalogIn
		if err := readJSON(r, &in); err != nil {
			http.Error(w, "json", http.StatusBadRequest)
			return
		}
		if in.CommissionEnabled != nil && !s.Admin && !s.Owner {
			http.Error(w, "комиссию клиента меняет администратор", 403)
			return
		}
		if in.Kind == "" {
			in.Kind = kind
		}
		if in.RevokeAccess {
			if !isAdmin {
				http.Error(w, "доступ снимает администратор", http.StatusForbidden)
				return
			}
			rec, err := a.db.RevokeEmployeeAccess(in.ID, isOwner)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, rec)
			return
		}
		in.CreatedBy = s.ID
		in.CreatedName = s.Name
		if relKind(in.Kind) {
			if !isAdmin && !hasDir {
				http.Error(w, "нет доступа к справочнику", http.StatusForbidden)
				return
			}
			if in.ID != 0 && !isAdmin {
				http.Error(w, "править карточку может администратор", http.StatusForbidden)
				return
			}
		} else if !isAdmin {
			http.Error(w, "это меняет администратор", http.StatusForbidden)
			return
		}
		rec, err := a.db.CatalogAdd(in)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if in.ID == 0 {
			a.db.BumpInbox(appdb.SecDirectory)
		}
		writeJSON(w, rec)
	default:
		http.Error(w, "метод", http.StatusMethodNotAllowed)
	}
}

// Kept as a tombstone for old cached clients. Existing document copies remain readable.
func (a *API) archiveRequest(w http.ResponseWriter, r *http.Request, s session) {
	if !s.requireSection(w, appdb.SecRequests) || !s.requireSection(w, appdb.SecDocuments) {
		return
	}
	http.Error(w, "Сохранение заявок в документы отключено", http.StatusGone)
}

func (a *API) requests(w http.ResponseWriter, r *http.Request, s session) {
	if !s.requireSection(w, appdb.SecRequests) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		if id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64); id != 0 {
			b, err := a.db.RequestBundle(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			for i := range b.Files {
				b.Files[i].Mine = b.Files[i].CreatedBy != 0 && b.Files[i].CreatedBy == s.ID
			}
			b.RequestPermission, _ = a.db.RequestPermissions(id, s.ID, s.Admin || s.Owner)
			if !b.CanView {
				http.Error(w, "заявка не найдена", http.StatusNotFound)
				return
			}
			writeJSON(w, b)
			return
		}
		views := a.db.RequestViews()
		out := make([]appdb.RequestView, 0, len(views))
		scope := r.URL.Query().Get("scope")
		if scope == "" {
			scope = "mine"
		}
		if scope != "mine" && scope != "all" {
			http.Error(w, "неверный scope", 400)
			return
		}
		for _, v := range views {
			v.RequestPermission, _ = a.db.RequestPermissions(v.ID, s.ID, s.Admin || s.Owner)
			if !v.CanView || (scope == "mine" && !v.IsMine) {
				continue
			}
			out = append(out, v)
		}
		writeJSON(w, out)
	case http.MethodPost:
		var in struct {
			Title          string                 `json:"title"`
			Action         string                 `json:"action"`
			AgentID        int64                  `json:"agent_id"`
			Economics      appdb.RequestEconomics `json:"economics"`
			Comment        string                 `json:"comment"`
			CloseReason    string                 `json:"close_reason"`
			UID            string                 `json:"uid"`
			Status         string                 `json:"status"`
			ID             int64                  `json:"id"`
			TableRef       string                 `json:"table_ref"`
			Notes          string                 `json:"notes"`
			ManagerID      int64                  `json:"manager_id"`
			ClientID       int64                  `json:"client_id"`
			CounterpartyID int64                  `json:"counterparty_id"`
		}
		if err := readJSON(r, &in); err != nil {
			http.Error(w, "json", http.StatusBadRequest)
			return
		}
		if in.Action == "edit_comment" {
			rec, err := a.db.EditRequestComment(in.ID, in.Comment, s.ID, s.Admin || s.Owner)
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			writeJSON(w, rec)
			return
		}
		if in.Action == "economics" {
			rec, err := a.db.SaveRequestEconomics(in.ID, in.AgentID, s.ID, in.Economics, s.Admin || s.Owner)
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			writeJSON(w, rec)
			return
		}
		if in.Action == "skip_stage" {
			if !s.Admin && !s.Owner {
				http.Error(w, "только администратор", 403)
				return
			}
			rec, err := a.db.SkipRequestStage(in.ID, s.ID, in.CloseReason, true)
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			writeJSON(w, rec)
			return
		}
		if in.Action == "reopen" {
			in.Status = "reopen"
		}
		if in.ID != 0 && in.Status != "" {
			rec, err := a.db.UpdateRequestState(in.ID, in.Status, in.CloseReason, s.ID, s.Admin || s.Owner)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if rec.Status == "success_closed" {
				for _, p := range a.db.PaymentsViews(true, s.ID) {
					if p.RequestID == rec.ID && p.Kind == "client_commission" {
						_ = a.deliver("payment", p.ID)
					}
				}
			}
			a.notifyUsers(appdb.SecRequests, appdb.NtfRequests, s.ID, requestNotifyHTML("Заявка", rec.UID, rec.Title, rec.Status))
			writeJSON(w, rec)
			return
		}
		title := strings.TrimSpace(in.Title)
		var threadID int64
		var link string
		if a.bot != nil {
			tid, lnk, err := a.bot.CreateRequestTopic(title)
			if err != nil {
				log.Printf("create topic: %v", err)
			} else {
				threadID, link = tid, lnk
			}
		}
		rec, err := a.db.AddRequest(title, s.Name, s.ID, threadID, link, in.ManagerID, in.ClientID, in.CounterpartyID, s.Admin || s.Owner, in.UID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.notifyUsers(appdb.SecRequests, appdb.NtfRequests, s.ID, requestNotifyHTML("Новая заявка", rec.UID, rec.Title, s.Name))
		writeJSON(w, rec)
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err := a.db.DeleteRequest(id, s.ID, s.Admin || s.Owner); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "метод", http.StatusMethodNotAllowed)
	}
}

func (a *API) approvals(w http.ResponseWriter, r *http.Request, s session) {
	if !s.requireSection(w, appdb.SecApprovals) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, a.db.ApprovalsFor(s.ID))
	case http.MethodPost:
		var in struct {
			Action     string `json:"action"`
			ID         int64  `json:"id"`
			Preview    string `json:"preview"`
			RequestUID string `json:"request_uid"`
			RequestID  int64  `json:"request_id"`
			Status     string `json:"status"`
		}
		if err := readJSON(r, &in); err != nil {
			http.Error(w, "json", http.StatusBadRequest)
			return
		}
		if in.Action == "retry_delivery" {
			if !s.Admin && !s.Owner {
				http.Error(w, "только администратор", 403)
				return
			}
			if err := a.deliver("approval", in.ID); err != nil {
				http.Error(w, err.Error(), 502)
				return
			}
			d, _ := a.db.DeliveryRecord("approval", in.ID)
			writeJSON(w, d)
			return
		}
		if in.ID != 0 && in.Status != "" {
			if in.Status == "withdrawn" || in.Status == "pending" {
				rec, err := a.db.RecallApproval(in.ID, s.ID, s.Admin || s.Owner, in.Status)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				writeJSON(w, rec)
				return
			}
			if !s.Admin {
				http.Error(w, "принять или отклонить может только администратор", http.StatusForbidden)
				return
			}
			rec, err := a.db.DecideApproval(in.ID, in.Status, s.Name)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			label := "принята"
			if in.Status == "rejected" {
				label = "отклонена"
			}
			a.db.BumpInboxIDs(appdb.SecApprovals, []int64{rec.ManagerID})
			if a.db.Wants(rec.ManagerID, appdb.SecApprovals, appdb.NtfApprovals) {
				_ = a.bot.Notify(rec.ManagerID, "Согласование <b>"+tgEsc(rec.UID)+"</b>: "+label+". Смотрите Mini App → Согласование.")
			}
			writeJSON(w, rec)
			return
		}
		if strings.TrimSpace(in.Preview) == "" && in.RequestID == 0 {
			http.Error(w, "preview", http.StatusBadRequest)
			return
		}
		if in.RequestID != 0 {
			rec, err := a.db.CreateRequestApproval(in.RequestID, s.ID, s.Name, s.Admin || s.Owner)
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			_ = a.deliver("approval", rec.ID)
			rec.Delivery, _ = a.db.DeliveryRecord("approval", rec.ID)
			writeJSON(w, rec)
			return
		}
		writeJSON(w, a.db.AddApproval(in.Preview, s.Name, s.ID, in.RequestUID, in.RequestID))
		label := in.Preview
		if strings.TrimSpace(label) == "" {
			label = "заявка"
		}
		a.notifyUsers(appdb.SecApprovals, appdb.NtfApprovals, s.ID, "На согласование: <b>"+tgEsc(label)+"</b> · "+tgEsc(s.Name))
	default:
		http.Error(w, "метод", http.StatusMethodNotAllowed)
	}
}

func (a *API) saldo(w http.ResponseWriter, r *http.Request, s session) {
	if !s.requireSection(w, appdb.SecSaldo) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		cps, curs := a.db.SaldoCatalog()
		view := r.URL.Query().Get("view")
		switch view {
		case "report":
			writeJSON(w, map[string]any{"report": a.db.SaldoReport()})
			return
		case "detail":
			writeJSON(w, map[string]any{"detail": a.db.SaldoDetail(r.URL.Query().Get("cp"))})
			return
		case "files":
			txt, bal, ops := a.db.SaldoFiles()
			writeJSON(w, map[string]any{"txt": txt, "balances_csv": bal, "operations_csv": ops})
			return
		}
		writeJSON(w, map[string]any{
			"counterparties": cps,
			"currencies":     curs,
			"ops":            a.db.SaldoCopy(),
			"totals":         a.db.SaldoTotals(),
			"balances":       a.db.SaldoBalancesCopy(),
			"report_lines":   a.db.SaldoReportLines(),
			"lots":           a.db.OpenLotsByManager(s.ID),
			"report":         a.db.SaldoReport(),
		})
	case http.MethodPost:
		var in struct {
			CP             string  `json:"cp"`
			Kind           string  `json:"kind"`
			Currency       string  `json:"currency"`
			Action         string  `json:"action"`
			Amount         float64 `json:"amount"`
			CloseID        int64   `json:"close_id"`
			AddCP          string  `json:"add_cp"`
			DelCP          string  `json:"del_cp"`
			RenameFrom     string  `json:"rename_from"`
			RenameTo       string  `json:"rename_to"`
			SendFiles      bool    `json:"send_files"`
			RequestID      int64   `json:"request_id"`
			CounterpartyID int64   `json:"counterparty_id"`
		}
		if err := readJSON(r, &in); err != nil {
			http.Error(w, "json", http.StatusBadRequest)
			return
		}
		if in.Action == "rebuild_balances" {
			if !s.Admin && !s.Owner {
				http.Error(w, "только администратор", 403)
				return
			}
			if err := a.db.RebuildSaldoBalances(s.ID); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			writeJSON(w, map[string]bool{"ok": true})
			return
		}
		if in.SendFiles {
			txt, bal, ops := a.db.SaldoFiles()
			chatID := a.db.AppSettings().SaldoChatID
			if chatID == 0 {
				http.Error(w, "настройте чат сальдо", http.StatusBadRequest)
				return
			}
			if a.sender == nil {
				http.Error(w, "бот недоступен", http.StatusBadGateway)
				return
			}
			if _, err := a.sender.SendBytesToGroup(chatID, "saldo_table.txt", []byte(txt), "Сальдо"); err != nil {
				http.Error(w, "не удалось отправить в чат сальдо", http.StatusBadGateway)
				return
			}
			if _, err := a.sender.SendBytesToGroup(chatID, "saldo_balances.csv", []byte(bal), "Текущие остатки"); err != nil {
				http.Error(w, "не удалось отправить остатки в чат сальдо", http.StatusBadGateway)
				return
			}
			if _, err := a.sender.SendBytesToGroup(chatID, "saldo_operations.csv", []byte(ops), "Журнал операций"); err != nil {
				http.Error(w, "не удалось отправить журнал в чат сальдо", http.StatusBadGateway)
				return
			}
			writeJSON(w, map[string]any{"ok": true, "sent": true})
			return
		}
		if strings.TrimSpace(in.AddCP) != "" || strings.TrimSpace(in.DelCP) != "" || strings.TrimSpace(in.RenameFrom) != "" {
			if !s.Admin && !s.Owner {
				http.Error(w, "контрагентов меняет админ", http.StatusForbidden)
				return
			}
			if strings.TrimSpace(in.AddCP) != "" {
				name, err := a.db.AddSaldoCP(in.AddCP)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				writeJSON(w, map[string]any{"cp": name})
				return
			}
			if strings.TrimSpace(in.DelCP) != "" {
				name, n, err := a.db.DeleteSaldoCP(in.DelCP)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				writeJSON(w, map[string]any{"deleted": name, "ops": n})
				return
			}
			old, neu, err := a.db.RenameSaldoCP(in.RenameFrom, in.RenameTo)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]any{"from": old, "to": neu})
			return
		}
		if in.CloseID != 0 {
			lot, err := a.db.CloseSaldoLot(in.CloseID, s.ID, s.Name)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, lot)
			return
		}
		if in.Amount == 0 || in.Kind == "" || in.Currency == "" {
			http.Error(w, "выберите тип, валюту и сумму", http.StatusBadRequest)
			return
		}
		if in.CP == "" && in.CounterpartyID == 0 && in.RequestID == 0 {
			http.Error(w, "выберите контрагента из списка", http.StatusBadRequest)
			return
		}
		if in.Action != "minus" {
			in.Action = "plus"
		}
		op, err := a.db.AddSaldo(in.CP, in.Kind, in.Currency, in.Action, s.Name, in.Amount, s.ID, in.RequestID, in.CounterpartyID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.db.BumpInbox(appdb.SecSaldo)
		writeJSON(w, op)
	default:
		http.Error(w, "метод", http.StatusMethodNotAllowed)
	}
}

func (a *API) getRates(w http.ResponseWriter, r *http.Request, s session) {
	if !s.requireSection(w, appdb.SecRates) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	snap := a.fetchRates(ctx)
	out := ratesJSON(snap)
	out["custom_slots"] = customRates(snap, a.db.AppSettings().CustomRateSlots)
	writeJSON(w, out)
}

func ratesJSON(s rates.Snapshot) map[string]any {
	rub := make([]map[string]any, 0, len(s.Rub))
	for _, q := range s.Rub {
		rub = append(rub, map[string]any{"pair": q.Pair, "bid": q.Bid})
	}
	fx := make([]map[string]any, 0, len(s.Forex))
	for _, q := range s.Forex {
		fx = append(fx, map[string]any{"pair": q.Pair, "bid": q.Bid})
	}
	cbr := make([]map[string]any, 0, len(s.CBR))
	for _, r := range s.CBR {
		cbr = append(cbr, map[string]any{"code": r.CharCode, "per_unit": r.PerUnit})
	}
	return map[string]any{
		"rapira": s.Rapira, "rapira_err": s.RapiraErr,
		"cbr": cbr, "cbr_date": s.CBRDate, "cbr_err": s.CBRErr,
		"rub": rub, "forex": fx, "pf_err": s.PFErr,
		"xe_eurusd": s.XEEURUSD, "xe_err": s.XEErr,
		"investing": s.Investing, "investing_err": s.InvestingErr,
	}
}

func (a *API) fetchRates(ctx context.Context) rates.Snapshot {
	var snap rates.Snapshot
	if all, err := a.pf.Fetch(ctx); err != nil {
		snap.PFErr = err.Error()
	} else {
		snap.Forex, _ = sources.SelectPairs(all, []string{"EUR/USD"})
		snap.Rub, _ = sources.SelectPairs(all, []string{"USD/RUB", "EUR/RUB", "CNY/RUB"})
	}
	if all, date, err := a.cbr.Fetch(ctx); err != nil {
		snap.CBRErr = err.Error()
	} else {
		snap.CBR, _ = sources.SelectCBR(all, []string{"USD", "EUR", "CNY", "AED"})
		snap.CBRDate = date
	}
	if v, err := a.xe.FetchEURUSD(ctx); err != nil {
		snap.XEErr = err.Error()
	} else {
		snap.XEEURUSD = v
	}
	if v, err := a.investing.FetchUSDRUB(ctx); err != nil {
		snap.InvestingErr = err.Error()
	} else {
		snap.Investing = v
	}
	if q, err := a.rapira.FetchUSDTRUB(ctx); err != nil {
		snap.RapiraErr = err.Error()
	} else {
		snap.Rapira = q.Last
	}
	return snap
}

func (a *API) getHolidays(w http.ResponseWriter, r *http.Request, s session) {
	if !s.requireSection(w, appdb.SecHolidays) {
		return
	}
	if a.holidays == nil {
		writeJSON(w, []any{})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	items, days, err := a.holidays.Upcoming(ctx, time.Now(), 14)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	type row struct {
		Name        string `json:"name"`
		CountryName string `json:"country_name"`
		Date        string `json:"date"`
		DateTo      string `json:"date_to"`
		SourceURL   string `json:"source_url"`
		DaysLeft    int    `json:"days_left"`
	}
	out := make([]row, 0, len(items))
	for i, it := range items {
		d := 0
		if i < len(days) {
			d = days[i]
		}
		to := ""
		if !it.DateTo.IsZero() {
			to = it.DateTo.Format("02.01.2006")
		}
		out = append(out, row{
			Name: it.Name, CountryName: it.CountryName,
			Date: it.Date.Format("02.01.2006"), DateTo: to,
			SourceURL: it.SourceURL, DaysLeft: d,
		})
	}
	writeJSON(w, out)
}

func (a *API) balance(w http.ResponseWriter, r *http.Request, s session) {
	if !s.requireSection(w, appdb.SecBalance) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{
			"companies":  a.db.CompaniesCopy(),
			"accounts":   a.db.AccountsCopy(),
			"morning_ok": a.db.BalanceSlotOK("morning", time.Now()),
			"evening_ok": a.db.BalanceSlotOK("evening", time.Now()),
		})
	case http.MethodPost:
		var in struct {
			CompanyID int64   `json:"company_id"`
			Bank      string  `json:"bank"`
			Currency  string  `json:"currency"`
			Amount    float64 `json:"amount"`
			Slot      string  `json:"slot"`
		}
		if err := readJSON(r, &in); err != nil || in.CompanyID == 0 || in.Bank == "" {
			http.Error(w, "company_id, bank, amount", http.StatusBadRequest)
			return
		}
		acc := a.db.UpsertAccount(in.CompanyID, in.Bank, in.Currency, in.Amount, s.Name)
		if in.Slot == "morning" || in.Slot == "evening" {
			a.db.MarkBalanceSlot(in.Slot)
		} else {
			h := time.Now().Hour()
			if h < 15 {
				a.db.MarkBalanceSlot("morning")
			} else {
				a.db.MarkBalanceSlot("evening")
			}
		}
		writeJSON(w, acc)
	default:
		http.Error(w, "метод", http.StatusMethodNotAllowed)
	}
}

func (a *API) compliance(w http.ResponseWriter, r *http.Request, s session) {
	if !s.requireSection(w, appdb.SecCompliance) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, a.db.CompliancesCopy())
	case http.MethodPost:
		var in struct {
			ID         int64  `json:"id"`
			Subject    string `json:"subject"`
			Text       string `json:"text"`
			Status     string `json:"status"`
			RequestUID string `json:"request_uid"`
			RequestID  int64  `json:"request_id"`
			Key        string `json:"key"`
			Answer     string `json:"answer"`
		}
		if err := readJSON(r, &in); err != nil {
			http.Error(w, "json", http.StatusBadRequest)
			return
		}
		if in.ID != 0 && in.Key != "" {
			rec, err := a.db.AnswerCompliance(in.ID, in.Key, in.Answer)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, rec)
			return
		}
		if in.ID != 0 && strings.TrimSpace(in.Status) != "" {
			rec, err := a.db.SetComplianceStatus(in.ID, in.Status)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, rec)
			return
		}
		if in.ID != 0 {
			rec, err := a.db.SetComplianceText(in.ID, in.Text)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, rec)
			return
		}
		if strings.TrimSpace(in.Subject) == "" && strings.TrimSpace(in.Text) == "" && in.RequestID == 0 {
			http.Error(w, "напишите текст проверки", http.StatusBadRequest)
			return
		}
		writeJSON(w, a.db.AddCompliance(in.Subject, in.Text, s.Name, s.ID, in.RequestUID, in.RequestID))
		a.db.BumpInbox(appdb.SecCompliance)
	default:
		http.Error(w, "метод", http.StatusMethodNotAllowed)
	}
}

func (a *API) disputes(w http.ResponseWriter, r *http.Request, s session) {
	if r.Method == http.MethodGet {
		writeJSON(w, a.db.DisputesFor(s.ID, s.Admin))
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "метод", http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		ID      int64  `json:"id"`
		Section string `json:"section"`
		RefUID  string `json:"ref_uid"`
		Text    string `json:"text"`
		Reply   string `json:"reply"`
		Status  string `json:"status"`
	}
	if err := readJSON(r, &in); err != nil {
		http.Error(w, "json", http.StatusBadRequest)
		return
	}
	if in.ID != 0 && in.Status != "" {
		if !s.Admin {
			http.Error(w, "закрыть спор может администратор", http.StatusForbidden)
			return
		}
		if err := a.db.SetDisputeStatus(in.ID, in.Status); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	if in.ID != 0 && strings.TrimSpace(in.Reply) != "" {
		d, err := a.db.ReplyDispute(in.ID, s.ID, s.Name, in.Reply, s.Admin)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.notifyPeerIDs(a.db.DisputePeers(d), appdb.SecApprovals, appdb.NtfApprovals, s.ID, "Новый ответ в споре <b>"+tgEsc(d.UID)+"</b>. Mini App → Согласование → Споры.")
		writeJSON(w, d)
		return
	}
	if strings.TrimSpace(in.Text) == "" {
		http.Error(w, "text", http.StatusBadRequest)
		return
	}
	if in.Section == "approvals" && !s.Admin {
		http.Error(w, "спор в согласовании может открыть только администратор", http.StatusForbidden)
		return
	}
	d, err := a.db.AddDispute(in.Section, in.RefUID, in.Text, s.Name, s.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.notifyPeerIDs(a.db.DisputePeers(d), appdb.SecApprovals, appdb.NtfApprovals, s.ID, "Вам отправили спор <b>"+tgEsc(d.UID)+"</b> по "+tgEsc(d.RefUID)+". Mini App → Согласование → Споры — ответьте кнопкой.")
	writeJSON(w, d)
}

func (a *API) inbox(w http.ResponseWriter, r *http.Request, s session) {
	if r.Method == http.MethodGet {
		writeJSON(w, a.db.Unread(s.ID, s.Admin))
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "метод", http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		Seen       string `json:"seen"`
		ID         int64  `json:"id"`
		HideClosed string `json:"hide_closed"`
		Hide       string `json:"hide"`
	}
	if err := readJSON(r, &in); err != nil {
		http.Error(w, "json", http.StatusBadRequest)
		return
	}
	if in.Seen == "dsp" && in.ID != 0 {
		a.db.MarkSeen(s.ID, fmt.Sprintf("dsp:%d", in.ID))
	} else if strings.TrimSpace(in.Seen) != "" {
		a.db.MarkSeen(s.ID, in.Seen)
	}
	if strings.TrimSpace(in.HideClosed) != "" {
		a.db.HideClosed(s.ID, in.HideClosed)
	}
	if strings.TrimSpace(in.Hide) != "" {
		if err := a.db.HideOne(s.ID, in.Hide); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	writeJSON(w, a.db.Unread(s.ID, s.Admin))
}

func requestNotifyHTML(verb, uid, title, extra string) string {
	msg := verb + " <b>" + tgEsc(uid) + "</b>"
	title = strings.TrimSpace(title)
	if title != "" && !strings.EqualFold(title, uid) {
		msg += ": " + tgEsc(title)
	}
	extra = strings.TrimSpace(extra)
	if extra != "" && !strings.EqualFold(extra, uid) && !strings.EqualFold(extra, title) {
		msg += " · " + tgEsc(extra)
	}
	return msg
}

func (a *API) notifyUsers(section, key string, except int64, html string) {
	a.db.BumpInbox(section)
	if a.bot == nil {
		return
	}
	for _, id := range a.db.NotifyIDs(section, key) {
		if id == except {
			continue
		}
		if err := a.bot.Notify(id, html); err != nil {
			log.Printf("notify %d: %v", id, err)
		}
	}
}

func (a *API) notifyPeerIDs(ids []int64, section, key string, except int64, html string) {
	bump := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id != 0 && id != except {
			bump = append(bump, id)
		}
	}
	a.db.BumpInboxIDs(section, bump)
	if a.bot == nil {
		return
	}
	ok := map[int64]bool{}
	for _, id := range a.db.NotifyIDs(section, key) {
		ok[id] = true
	}
	for _, id := range ids {
		if id == 0 || id == except || !ok[id] {
			continue
		}
		if err := a.bot.Notify(id, html); err != nil {
			log.Printf("notify %d: %v", id, err)
		}
	}
}

func (a *API) notifyPrefs(w http.ResponseWriter, r *http.Request, s session) {
	if r.Method == http.MethodGet {
		writeJSON(w, appdb.NormalizeNotify(s.User.Notify))
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "метод", http.StatusMethodNotAllowed)
		return
	}
	var in appdb.Notify
	if err := readJSON(r, &in); err != nil {
		http.Error(w, "json", http.StatusBadRequest)
		return
	}
	n, err := a.db.SetNotify(s.ID, in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, n)
}

func (a *API) payments(w http.ResponseWriter, r *http.Request, s session) {
	switch r.Method {
	case http.MethodGet:
		if !s.canPayments() && !s.canSection(appdb.SecRequests) {
			deny(w, "нет доступа", http.StatusForbidden)
			return
		}
		writeJSON(w, a.db.PaymentsViews(s.canPayments(), s.ID))
	case http.MethodPost:
		var in struct {
			Action    string `json:"action"`
			ID        int64  `json:"id"`
			RequestID int64  `json:"request_id"`
			Text      string `json:"text"`
			Status    string `json:"status"`
		}
		if err := readJSON(r, &in); err != nil {
			http.Error(w, "json", http.StatusBadRequest)
			return
		}
		if in.Action == "retry_delivery" {
			if !s.Admin && !s.Owner {
				http.Error(w, "только администратор", 403)
				return
			}
			if err := a.deliver("payment", in.ID); err != nil {
				http.Error(w, err.Error(), 502)
				return
			}
			d, _ := a.db.DeliveryRecord("payment", in.ID)
			writeJSON(w, d)
			return
		}
		if in.ID != 0 && in.Status == "sent" {
			if !s.canPayments() {
				http.Error(w, "отметить отправку может тот, кто занимается отправкой", http.StatusForbidden)
				return
			}
			rec, err := a.db.MarkPaymentSent(in.ID, s.ID, s.Name)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if rec.CreatedBy != 0 && rec.CreatedBy != s.ID {
				_ = a.bot.Notify(rec.CreatedBy, "Оплата по заявке <b>"+tgEsc(rec.RequestTitle)+"</b> отмечена как отправлено · "+tgEsc(s.Name))
			}
			writeJSON(w, rec)
			return
		}
		if in.ID != 0 && in.Status == "pending" {
			rec, err := a.db.ReturnPayment(in.ID, s.ID, s.Admin || s.Owner)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, rec)
			return
		}
		if in.ID != 0 && (in.Status == "deleted" || in.Status == "cancel") {
			if err := a.db.DeletePayment(in.ID, s.ID, s.Admin || s.Owner); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		if !s.requireSection(w, appdb.SecRequests) {
			return
		}
		if a.db.AppSettings().PaymentChatID == 0 {
			http.Error(w, "настройте чат оплаты", 400)
			return
		}
		rec, err := a.db.AddPayment(in.RequestID, in.Text, s.Name, s.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = a.deliver("payment", rec.ID)
		rec.Delivery, _ = a.db.DeliveryRecord("payment", rec.ID)
		a.notifyPayments(s.ID, "Новая оплата в отправку\n\n<b>"+tgEsc(rec.RequestTitle)+"</b>\n"+tgEsc(rec.Context)+"\n\nЧто отправить:\n"+tgEsc(rec.Text)+"\n\nMini App → Отправка")
		writeJSON(w, rec)
	default:
		http.Error(w, "метод", http.StatusMethodNotAllowed)
	}
}

func (a *API) dealFiles(w http.ResponseWriter, r *http.Request, s session) {
	if !s.requireSection(w, appdb.SecRequests) {
		return
	}
	if r.Method == http.MethodGet {
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id == 0 {
			http.Error(w, "нужен id", http.StatusBadRequest)
			return
		}
		f, data, err := a.db.DealFileBytes(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		name := f.Name
		if name == "" {
			name = "file"
		}
		w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(name, `"`, "")+`"`)
		if f.Mime != "" {
			w.Header().Set("Content-Type", f.Mime)
		} else {
			w.Header().Set("Content-Type", "application/octet-stream")
		}
		_, _ = w.Write(data)
		return
	}
	if r.Method == http.MethodDelete {
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err := a.db.DeleteDealFile(id, s.ID, s.Admin || s.Owner); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "метод", http.StatusMethodNotAllowed)
		return
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		r.Body = http.MaxBytesReader(w, r.Body, 25<<20)
		if err := r.ParseMultipartForm(25 << 20); err != nil {
			http.Error(w, "файл слишком большой (до 25 МБ)", http.StatusBadRequest)
			return
		}
		reqID, _ := strconv.ParseInt(r.FormValue("request_id"), 10, 64)
		headers := r.MultipartForm.File["file"]
		if len(headers) == 0 {
			headers = r.MultipartForm.File["files"]
		}
		if len(headers) == 0 {
			http.Error(w, "нужен файл", http.StatusBadRequest)
			return
		}
		recs := make([]appdb.DealFile, 0, len(headers))
		for _, hdr := range headers {
			fh, err := hdr.Open()
			if err != nil {
				http.Error(w, "не удалось прочитать файл", http.StatusBadRequest)
				return
			}
			data, err := io.ReadAll(fh)
			_ = fh.Close()
			if err != nil {
				http.Error(w, "не удалось прочитать файл", http.StatusBadRequest)
				return
			}
			mime := hdr.Header.Get("Content-Type")
			rec, err := a.db.AddDealFile(reqID, hdr.Filename, mime, data, s.Name, s.ID, "")
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			recs = append(recs, rec)
		}
		ids := make([]int64, 0, len(recs))
		for _, rec := range recs {
			ids = append(ids, rec.ID)
		}
		a.db.RememberSent(s.ID, ids)
		if len(recs) == 1 {
			writeJSON(w, recs[0])
			return
		}
		writeJSON(w, recs)
		return
	}
	var in struct {
		RequestID int64  `json:"request_id"`
		Text      string `json:"text"`
		Kind      string `json:"kind"`
		ID        int64  `json:"id"`
		SendSelf  bool   `json:"send_self"`
	}
	if err := readJSON(r, &in); err != nil {
		http.Error(w, "json", http.StatusBadRequest)
		return
	}
	if in.SendSelf {
		if a.bot == nil {
			http.Error(w, "бот не запущен", http.StatusBadRequest)
			return
		}
		f, err := a.db.DealFile(in.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		title := a.db.RequestTitle(f.RequestID)
		if f.Kind != "file" {
			html := "Сообщение из заявки"
			if title != "" {
				html += " <b>" + tgEsc(title) + "</b>"
			}
			html += ":\n\n" + tgEsc(f.Text)
			if err := a.bot.Notify(s.ID, html); err != nil {
				http.Error(w, "не отправилось в личку: "+err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]any{"ok": true, "sent": "message"})
			return
		}
		_, data, err := a.db.DealFileBytes(f.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cap := "Заявка"
		if title != "" {
			cap = title
		}
		if t := strings.TrimSpace(f.Text); t != "" {
			cap += "\n" + t
		}
		if err := a.bot.SendDealToUser(s.ID, f.Name, f.Mime, data, cap); err != nil {
			http.Error(w, "не отправилось в личку: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "sent": "file"})
		return
	}
	rec, err := a.db.AddDealNote(in.RequestID, in.Kind, in.Text, s.Name, s.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.db.RememberSent(s.ID, []int64{rec.ID})
	writeJSON(w, rec)
}

func (a *API) appealChat(kind string) int64 {
	lawyer, docs := a.db.AppealChats()
	if kind == "lawyer" {
		if lawyer != 0 {
			return lawyer
		}
		return secrets.LawyerChatID
	}
	if docs != 0 {
		return docs
	}
	return secrets.DocsChatID
}

func parseChatID(raw string, num int64) (int64, error) {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, " ", "")
	if s == "" && num == 0 {
		return 0, nil
	}
	if s == "" {
		return num, nil
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("не похоже на ID чата")
	}
	return id, nil
}

func (a *API) appeals(w http.ResponseWriter, r *http.Request, s session) {
	if !s.canAppeals() && !s.canSection(appdb.SecRequests) {
		deny(w, "нет доступа к обращениям", http.StatusForbidden)
		return
	}
	lawyer := s.Admin || s.canSection(appdb.SecAppealLawyer)
	docs := s.Admin || s.canSection(appdb.SecAppealDocs)
	if r.Method == http.MethodGet {
		lawChat, docsChat := a.appealChat("lawyer"), a.appealChat("docs")
		writeJSON(w, map[string]any{
			"items":          a.db.AppealsList(s.ID, s.Admin || s.Owner, lawyer, docs),
			"stats":          a.db.AppealStats(),
			"lawyer_chat_id": lawChat,
			"docs_chat_id":   docsChat,
			"chats":          a.db.ChatsCopy(),
			"can_edit_chats": s.Admin || s.Owner,
		})
		return
	}
	if r.Method == http.MethodDelete {
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err := a.db.CancelAppeal(id, s.ID, s.Admin || s.Owner); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "метод", http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		ID        int64  `json:"id"`
		RequestID int64  `json:"request_id"`
		Kind      string `json:"kind"`
		Text      string `json:"text"`
		Status    string `json:"status"`
		ChatID    int64  `json:"chat_id"`
		ChatRaw   string `json:"chat_raw"`
		SetChat   bool   `json:"set_chat"`
	}
	if err := readJSON(r, &in); err != nil {
		http.Error(w, "json", http.StatusBadRequest)
		return
	}
	if in.SetChat {
		if !s.Admin && !s.Owner {
			deny(w, "чат задаёт только админ", http.StatusForbidden)
			return
		}
		id, err := parseChatID(in.ChatRaw, in.ChatID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		law, doc, err := a.db.SetAppealChat(in.Kind, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "lawyer_chat_id": law, "docs_chat_id": doc})
		return
	}
	if in.ID != 0 && in.Status == "taken" {
		rec, err := a.db.TakeAppeal(in.ID, s.ID, s.Name, s.Admin || s.Owner, lawyer, docs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if a.bot != nil {
			a.bot.AnnounceAppeal(rec, "taken")
		}
		writeJSON(w, rec)
		return
	}
	if in.ID != 0 && in.Status == "closed" {
		rec, err := a.db.CloseAppeal(in.ID, s.ID, s.Name, s.Admin || s.Owner, lawyer, docs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if a.bot != nil {
			a.bot.AnnounceAppeal(rec, "closed")
		}
		writeJSON(w, rec)
		return
	}
	if !s.canSection(appdb.SecAppeals) && !s.canSection(appdb.SecRequests) && !s.Admin {
		deny(w, "нельзя создать обращение", http.StatusForbidden)
		return
	}
	kind := strings.TrimSpace(in.Kind)
	if kind != "lawyer" {
		kind = "docs"
	}
	chatID := a.appealChat(kind)
	if chatID != 0 {
		if a.bot == nil {
			http.Error(w, "бот не запущен", http.StatusBadRequest)
			return
		}
		if err := a.bot.EnsureInGroup(chatID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	rec, ctx, err := a.db.AddAppeal(in.RequestID, kind, in.Text, s.Name, s.ID, chatID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if kind == "lawyer" {
		a.db.BumpInbox(appdb.SecAppealLawyer)
	} else {
		a.db.BumpInbox(appdb.SecAppealDocs)
	}
	errText := ""
	mid := 0
	if a.bot != nil && chatID != 0 {
		sent, sendErr := a.bot.PostAppeal(rec, ctx, s.Username)
		if sendErr != nil {
			errText = botChatErr(sendErr).Error()
			log.Printf("appeal chat %d: %v", chatID, sendErr)
			_ = a.db.SetAppealSent(rec.ID, chatID, 0, errText)
			http.Error(w, errText, http.StatusBadRequest)
			return
		}
		mid = sent
	}
	out := a.db.SetAppealSent(rec.ID, chatID, mid, errText)
	writeJSON(w, out)
}

func (a *API) tasks(w http.ResponseWriter, r *http.Request, s session) {
	if !s.requireSection(w, appdb.SecTasks) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, a.db.TasksViews())
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err := a.db.DeleteTask(id, s.ID, s.Admin || s.Owner); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodPost:
		var in struct {
			ID         int64  `json:"id"`
			Title      string `json:"title"`
			Text       string `json:"text"`
			Status     string `json:"status"`
			RequestID  int64  `json:"request_id"`
			EmployeeID int64  `json:"employee_id"`
			ManagerID  int64  `json:"manager_id"`
		}
		if err := readJSON(r, &in); err != nil {
			http.Error(w, "json", http.StatusBadRequest)
			return
		}
		if in.ID != 0 && in.Status == "deleted" {
			if err := a.db.DeleteTask(in.ID, s.ID, s.Admin || s.Owner); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		if in.ID != 0 && strings.TrimSpace(in.Status) != "" && strings.TrimSpace(in.Title) == "" {
			rec, err := a.db.SetTaskStatus(in.ID, in.Status)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, rec)
			return
		}
		if in.ID != 0 {
			rec, err := a.db.UpdateTask(in.ID, in.Title, in.Text, in.RequestID, in.EmployeeID, s.ID, s.Admin || s.Owner)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if strings.TrimSpace(in.Status) != "" {
				rec, err = a.db.SetTaskStatus(in.ID, in.Status)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
			writeJSON(w, rec)
			return
		}
		rec, err := a.db.AddTask(in.Title, in.Text, in.Status, in.RequestID, in.EmployeeID, in.ManagerID, s.ID, s.Name, s.Admin || s.Owner)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.db.BumpInbox(appdb.SecTasks)
		writeJSON(w, rec)
	default:
		http.Error(w, "метод", http.StatusMethodNotAllowed)
	}
}

func (a *API) botName() string {
	if a.bot == nil {
		return ""
	}
	return a.bot.Username()
}

func (a *API) attach(w http.ResponseWriter, r *http.Request, s session) {
	if !s.canSection(appdb.SecRequests) && !s.canSection(appdb.SecDocuments) {
		deny(w, "нет доступа к разделу", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodGet:
		wai, rec, ok := a.db.AttachWaitOf(s.ID)
		if !ok {
			writeJSON(w, map[string]any{"waiting": false, "bot_username": a.botName()})
			return
		}
		kind := "deal"
		if wai.IsVault() {
			kind = "vault"
		}
		writeJSON(w, map[string]any{
			"waiting": true, "kind": kind, "request_id": wai.RequestID, "until": wai.Until,
			"request_title": rec.Title, "request_uid": rec.UID, "bot_username": a.botName(),
			"scope": wai.Scope, "owner_id": wai.OwnerID, "folder": wai.Folder, "label": wai.Label,
		})
	case http.MethodDelete:
		a.db.ClearAttachWait(s.ID)
		writeJSON(w, map[string]any{"ok": true, "waiting": false})
	case http.MethodPost:
		var in struct {
			RequestID int64  `json:"request_id"`
			Cancel    bool   `json:"cancel"`
			Undo      bool   `json:"undo"`
			ClearMine bool   `json:"clear_mine"`
			Vault     bool   `json:"vault"`
			Scope     string `json:"scope"`
			OwnerID   int64  `json:"owner_id"`
			Folder    string `json:"folder"`
		}
		if err := readJSON(r, &in); err != nil {
			http.Error(w, "json", http.StatusBadRequest)
			return
		}
		wantVault := in.Vault || strings.TrimSpace(in.Scope) != ""
		if in.Undo {
			n, err := a.db.UndoLastSent(s.ID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]any{"ok": true, "undone": n})
			return
		}
		if in.ClearMine {
			if wantVault {
				if !s.requireSection(w, appdb.SecDocuments) {
					return
				}
				n, err := a.db.DeleteOwnVault(in.Scope, in.OwnerID, in.Folder, s.ID)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				writeJSON(w, map[string]any{"ok": true, "cleared": n})
				return
			}
			if !s.requireSection(w, appdb.SecRequests) {
				return
			}
			n, err := a.db.DeleteOwnOnRequest(in.RequestID, s.ID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]any{"ok": true, "cleared": n})
			return
		}
		if in.Cancel {
			a.db.ClearAttachWait(s.ID)
			writeJSON(w, map[string]any{"ok": true, "waiting": false})
			return
		}
		if wantVault {
			if !s.requireSection(w, appdb.SecDocuments) {
				return
			}
			rec, err := a.db.SetVaultAttachWait(s.ID, in.Scope, in.OwnerID, in.Folder, 2*time.Hour)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			uname := a.botName()
			if a.bot != nil {
				hint := "Жду пересылку в документы <b>" + tgEsc(rec.Label) + "</b>.\n\n" +
					"Перешлите сюда файлы, фото или видео из Telegram — положу в Mini App.\n" +
					"Можно несколько подряд. Ошиблись — кнопка «Удалить» под ответом или /undo.\n/done — закончить."
				if err := a.bot.NotifyAttach(s.ID, hint); err != nil {
					log.Printf("vault attach hint %d: %v", s.ID, err)
				}
			}
			writeJSON(w, map[string]any{
				"ok": true, "waiting": true, "kind": "vault", "until": rec.Until,
				"scope": rec.Scope, "owner_id": rec.OwnerID, "folder": rec.Folder,
				"label": rec.Label, "bot_username": uname, "start": vaultStartArg(rec.Scope, rec.OwnerID, rec.Folder),
			})
			return
		}
		if !s.requireSection(w, appdb.SecRequests) {
			return
		}
		if in.RequestID == 0 {
			http.Error(w, "нужна заявка", http.StatusBadRequest)
			return
		}
		rec, err := a.db.SetAttachWait(s.ID, in.RequestID, 2*time.Hour)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		uname := a.botName()
		if a.bot != nil {
			hint := "Жду пересылку для заявки <b>" + tgEsc(rec.Title) + "</b> (" + tgEsc(rec.UID) + ").\n\n" +
				"Перешлите сюда файлы, фото или сообщения из Telegram — положу в Mini App, в таблицу не уйдёт.\n" +
				"Можно несколько подряд. Ошиблись — кнопка «Удалить» под ответом или /undo.\n/done — закончить."
			if err := a.bot.NotifyAttach(s.ID, hint); err != nil {
				log.Printf("attach hint %d: %v", s.ID, err)
			}
		}
		writeJSON(w, map[string]any{
			"ok": true, "waiting": true, "kind": "deal", "request_id": rec.ID, "until": time.Now().Add(2 * time.Hour),
			"request_title": rec.Title, "bot_username": uname,
		})
	default:
		http.Error(w, "метод", http.StatusMethodNotAllowed)
	}
}

func (a *API) notifyPayments(except int64, html string) {
	a.db.BumpInbox(appdb.SecPayments)
	if a.bot == nil {
		return
	}
	seen := map[int64]bool{except: true}
	send := func(id int64) {
		if id == 0 || seen[id] {
			return
		}
		seen[id] = true
		if err := a.bot.Notify(id, html); err != nil {
			log.Printf("pay notify %d: %v", id, err)
		}
	}
	for _, id := range secrets.MainAdminIDs {
		send(id)
	}
	for _, id := range a.db.NotifyIDs(appdb.SecPayments, appdb.NtfPayments) {
		send(id)
	}
}

func (a *API) documents(w http.ResponseWriter, r *http.Request, s session) {
	if id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64); id != 0 && !s.Admin && !s.Owner && a.db.IsSystemArchive(id) {
		http.Error(w, "только администратор", 403)
		return
	}
	if !s.requireSection(w, appdb.SecDocuments) {
		return
	}
	if r.Method == http.MethodGet {
		if r.URL.Query().Get("destinations") == "1" {
			writeJSON(w, a.db.VaultDestinations())
			return
		}
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id != 0 {
			if r.URL.Query().Get("archive") == "1" {
				f, snapshot, err := a.db.RequestArchivePreview(id)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				writeJSON(w, map[string]any{"file": f, "report": snapshot.Report(), "entries": snapshot.Entries})
				return
			}
			if entry := r.URL.Query().Get("entry"); entry != "" {
				f, data, err := a.db.RequestArchiveEntry(id, entry)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": f.Name}))
				w.Header().Set("Content-Type", "application/octet-stream")
				if f.Mime != "" {
					w.Header().Set("Content-Type", f.Mime)
				}
				w.Header().Set("X-Content-Type-Options", "nosniff")
				_, _ = w.Write(data)
				return
			}
			f, data, err := a.db.VaultFileBytes(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			name := f.Name
			if name == "" {
				name = "file"
			}
			w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(name, `"`, "")+`"`)
			if f.Mime != "" {
				w.Header().Set("Content-Type", f.Mime)
			} else {
				w.Header().Set("Content-Type", "application/octet-stream")
			}
			_, _ = w.Write(data)
			return
		}
		scope := r.URL.Query().Get("scope")
		folder := r.URL.Query().Get("folder")
		ownerID, _ := strconv.ParseInt(r.URL.Query().Get("owner_id"), 10, 64)
		contour, _, _ := appdb.ParseVaultContour(folder)
		writeJSON(w, map[string]any{
			"companies":      a.db.CompaniesCopy(),
			"counterparties": a.db.CounterpartiesCopy(),
			"files":          a.db.VaultFilesCopy(scope, ownerID, folder, s.ID),
			"tabs":           a.db.VaultTabsCopy(ownerID, contour),
			"can_edit":       s.Admin || s.Owner,
		})
		return
	}
	if r.Method == http.MethodDelete {
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err := a.db.DeleteVaultFile(id, s.ID, s.Admin || s.Owner); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "метод", http.StatusMethodNotAllowed)
		return
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		r.Body = http.MaxBytesReader(w, r.Body, 25<<20)
		if err := r.ParseMultipartForm(25 << 20); err != nil {
			http.Error(w, "файл слишком большой (до 25 МБ)", http.StatusBadRequest)
			return
		}
		scope := r.FormValue("scope")
		folder := r.FormValue("folder")
		ownerID, _ := strconv.ParseInt(r.FormValue("owner_id"), 10, 64)
		headers := r.MultipartForm.File["file"]
		if len(headers) == 0 {
			headers = r.MultipartForm.File["files"]
		}
		if len(headers) == 0 {
			http.Error(w, "нужен файл", http.StatusBadRequest)
			return
		}
		recs := make([]appdb.VaultFile, 0, len(headers))
		for _, hdr := range headers {
			fh, err := hdr.Open()
			if err != nil {
				http.Error(w, "не удалось прочитать файл", http.StatusBadRequest)
				return
			}
			data, err := io.ReadAll(fh)
			_ = fh.Close()
			if err != nil {
				http.Error(w, "не удалось прочитать файл", http.StatusBadRequest)
				return
			}
			mime := hdr.Header.Get("Content-Type")
			rec, err := a.db.AddVaultFile(scope, ownerID, folder, hdr.Filename, mime, data, s.Name, s.ID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			recs = append(recs, rec)
		}
		a.db.BumpInbox(appdb.SecDocuments)
		ids := make([]int64, 0, len(recs))
		for _, rec := range recs {
			ids = append(ids, rec.ID)
		}
		a.db.RememberVaultSent(s.ID, ids)
		if len(recs) == 1 {
			writeJSON(w, recs[0])
			return
		}
		writeJSON(w, recs)
		return
	}
	var in struct {
		Action    string `json:"action"`
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		SendSelf  bool   `json:"send_self"`
		CompanyID int64  `json:"company_id"`
		Contour   string `json:"contour"`
	}
	if err := readJSON(r, &in); err != nil {
		http.Error(w, "json", http.StatusBadRequest)
		return
	}
	if in.SendSelf || in.Action == "send_self" {
		if a.bot == nil {
			http.Error(w, "бот не запущен", http.StatusBadRequest)
			return
		}
		f, data, err := a.db.VaultFileBytes(in.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cap := a.db.VaultPlaceLabel(f.Scope, f.OwnerID, f.Folder)
		if cap == "" {
			cap = "Документы"
		}
		if err := a.bot.SendDealToUser(s.ID, f.Name, f.Mime, data, cap); err != nil {
			http.Error(w, "не отправилось в личку: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "sent": "file"})
		return
	}
	if in.Action == "add_company" || in.Action == "rename_company" || in.Action == "delete_company" {
		if !s.Admin && !s.Owner {
			deny(w, "компании меняет только админ", http.StatusForbidden)
			return
		}
	}
	switch in.Action {
	case "add_company":
		c, err := a.db.AddCompany(in.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, c)
	case "rename_company":
		c, err := a.db.RenameCompany(in.ID, in.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, c)
	case "delete_company":
		if err := a.db.DeleteCompany(in.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case "add_tab":
		t, err := a.db.AddCompanyVaultTab(in.CompanyID, in.Contour, in.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, t)
	case "rename_tab":
		t, err := a.db.RenameCompanyVaultTab(in.ID, in.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, t)
	case "delete_tab":
		if err := a.db.DeleteCompanyVaultTab(in.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "неизвестное действие", http.StatusBadRequest)
	}
}

func (a *API) chats(w http.ResponseWriter, r *http.Request, s session) {
	if !s.Admin && !s.Owner {
		deny(w, "чаты правят владелец и админы", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodGet:
		law, docs := a.db.AppealChats()
		writeJSON(w, map[string]any{
			"items":          a.db.ChatsCopy(),
			"lawyer_chat_id": law,
			"docs_chat_id":   docs,
		})
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err := a.db.DeleteManagedChat(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodPost:
		var in struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			ChatID  int64  `json:"chat_id"`
			ChatRaw string `json:"chat_raw"`
		}
		if err := readJSON(r, &in); err != nil {
			http.Error(w, "json", http.StatusBadRequest)
			return
		}
		tgID, err := parseChatID(in.ChatRaw, in.ChatID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if in.ID != 0 {
			c, err := a.db.UpdateManagedChat(in.ID, in.Name, tgID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, c)
			return
		}
		c, err := a.db.AddManagedChat(in.Name, tgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, c)
	default:
		http.Error(w, "метод", http.StatusMethodNotAllowed)
	}
}
