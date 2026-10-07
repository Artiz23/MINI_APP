package appdb

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

type Payment struct {
	ID             int64     `json:"id"`
	UID            string    `json:"uid"`
	RequestID      int64     `json:"request_id"`
	Text           string    `json:"text"`
	Status         string    `json:"status"`
	CreatedBy      int64     `json:"created_by"`
	CreatedName    string    `json:"created_name"`
	CreatedAt      time.Time `json:"created_at"`
	SentBy         int64     `json:"sent_by,omitempty"`
	SentName       string    `json:"sent_name,omitempty"`
	SentAt         time.Time `json:"sent_at,omitempty"`
	EmployeeID     int64     `json:"employee_id,omitempty"`
	ManagerID      int64     `json:"manager_id,omitempty"`
	ClientID       int64     `json:"client_id,omitempty"`
	CounterpartyID int64     `json:"counterparty_id,omitempty"`
}

type PaymentView struct {
	Payment
	RequestTitle       string `json:"request_title,omitempty"`
	RequestUID         string `json:"request_uid,omitempty"`
	EmployeeName       string `json:"employee_name,omitempty"`
	ManagerName        string `json:"manager_name,omitempty"`
	ManagerAbbrev      string `json:"manager_abbrev,omitempty"`
	ClientName         string `json:"client_name,omitempty"`
	ClientWorkID       string `json:"client_work_id,omitempty"`
	CounterpartyName   string `json:"counterparty_name,omitempty"`
	CounterpartyWorkID string `json:"counterparty_work_id,omitempty"`
	Context            string `json:"context,omitempty"`
}

type DealFile struct {
	ID          int64     `json:"id"`
	UID         string    `json:"uid"`
	RequestID   int64     `json:"request_id"`
	Name        string    `json:"name"`
	Mime        string    `json:"mime,omitempty"`
	Size        int64     `json:"size,omitempty"`
	Rel         string    `json:"rel,omitempty"`
	Kind        string    `json:"kind"`
	Text        string    `json:"text,omitempty"`
	CreatedBy   int64     `json:"created_by"`
	CreatedName string    `json:"created_name"`
	CreatedAt   time.Time `json:"created_at"`
	Mine        bool      `json:"mine,omitempty"`
}

type Appeal struct {
	ID           int64     `json:"id"`
	UID          string    `json:"uid"`
	RequestID    int64     `json:"request_id"`
	Kind         string    `json:"kind"`
	Text         string    `json:"text"`
	ChatID       int64     `json:"chat_id,omitempty"`
	MessageID    int       `json:"message_id,omitempty"`
	CreatedBy    int64     `json:"created_by"`
	CreatedName  string    `json:"created_name"`
	CreatedAt    time.Time `json:"created_at"`
	Status       string    `json:"status"`
	Error        string    `json:"error,omitempty"`
	TakenBy      int64     `json:"taken_by,omitempty"`
	TakenName    string    `json:"taken_name,omitempty"`
	TakenAt      time.Time `json:"taken_at,omitempty"`
	ClosedBy     int64     `json:"closed_by,omitempty"`
	ClosedName   string    `json:"closed_name,omitempty"`
	ClosedAt     time.Time `json:"closed_at,omitempty"`
	NotifyMID    int       `json:"notify_mid,omitempty"`
	RequestUID   string    `json:"request_uid,omitempty"`
	RequestTitle string    `json:"request_title,omitempty"`
}

type RequestBundle struct {
	RequestView
	Payments  []PaymentView `json:"payments"`
	Files     []DealFile    `json:"files"`
	Appeals   []Appeal      `json:"appeals"`
	Saldo     []SaldoOp     `json:"saldo"`
	Approvals []Approval    `json:"approvals"`
}

func (s *Store) FilesDir() string {
	return filepath.Join(filepath.Dir(s.path), "files")
}

func (s *Store) requestByIDLocked(id int64) (Request, bool) {
	for _, r := range s.Requests {
		if r.ID == id {
			return r, true
		}
	}
	return Request{}, false
}

func (s *Store) requestContextLocked(r Request) string {
	v := s.viewRequestLocked(r)
	mgr := v.ManagerName
	if v.ManagerAbbrev != "" {
		mgr += " (" + v.ManagerAbbrev + ")"
	}
	cl := v.ClientName
	if v.ClientWorkID != "" {
		cl += " · " + v.ClientWorkID
	}
	cp := v.CounterpartyName
	if v.CounterpartyWorkID != "" {
		cp += " · " + v.CounterpartyWorkID
	}
	emp := v.EmployeeName
	if emp == "" {
		emp = v.CreatedName
	}
	return strings.TrimSpace(fmt.Sprintf(
		"Заявка: %s (%s)\nСотрудник: %s\nМенеджер: %s\nКлиент: %s\nКонтрагент: %s",
		v.Title, v.UID, nz(emp), nz(mgr), nz(cl), nz(cp),
	))
}

func nz(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func (s *Store) RequestBundle(id int64) (RequestBundle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requestBundleLocked(id)
}

func (s *Store) requestBundleLocked(id int64) (RequestBundle, error) {
	r, ok := s.requestByIDLocked(id)
	if !ok {
		return RequestBundle{}, fmt.Errorf("заявка не найдена")
	}
	b := RequestBundle{
		RequestView: s.viewRequestLocked(r),
		Payments:    []PaymentView{},
		Files:       []DealFile{},
		Appeals:     []Appeal{},
		Saldo:       []SaldoOp{},
		Approvals:   []Approval{},
	}
	for _, p := range s.Payments {
		if p.RequestID == id {
			b.Payments = append(b.Payments, s.paymentViewLocked(p, r))
		}
	}
	for _, f := range s.DealFiles {
		if f.RequestID == id {
			b.Files = append(b.Files, f)
		}
	}
	for _, a := range s.Appeals {
		if a.RequestID == id {
			b.Appeals = append(b.Appeals, a)
		}
	}
	for _, op := range s.SaldoOps {
		if op.RequestID == id {
			b.Saldo = append(b.Saldo, op)
		}
	}
	for _, a := range s.Approvals {
		if a.RequestID == id || (a.RequestUID != "" && a.RequestUID == r.UID) {
			b.Approvals = append(b.Approvals, a)
		}
	}
	sort.Slice(b.Payments, func(i, j int) bool { return b.Payments[i].ID > b.Payments[j].ID })
	sort.Slice(b.Files, func(i, j int) bool { return b.Files[i].ID > b.Files[j].ID })
	sort.Slice(b.Appeals, func(i, j int) bool { return b.Appeals[i].ID > b.Appeals[j].ID })
	sort.Slice(b.Approvals, func(i, j int) bool { return b.Approvals[i].ID > b.Approvals[j].ID })
	return b, nil
}

func (s *Store) paymentViewLocked(p Payment, r Request) PaymentView {
	v := s.viewRequestLocked(r)
	return PaymentView{
		Payment:      p,
		RequestTitle: v.Title, RequestUID: v.UID,
		EmployeeName: v.EmployeeName, ManagerName: v.ManagerName, ManagerAbbrev: v.ManagerAbbrev,
		ClientName: v.ClientName, ClientWorkID: v.ClientWorkID,
		CounterpartyName: v.CounterpartyName, CounterpartyWorkID: v.CounterpartyWorkID,
		Context: s.requestContextLocked(r),
	}
}

func (s *Store) AddPayment(requestID int64, text, byName string, by int64) (PaymentView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	text = strings.TrimSpace(text)
	if text == "" {
		return PaymentView{}, fmt.Errorf("напишите, что нужно отправить")
	}
	r, ok := s.requestByIDLocked(requestID)
	if !ok {
		return PaymentView{}, fmt.Errorf("заявка не найдена")
	}
	id := s.nextLocked()
	p := Payment{
		ID: id, UID: uid("pay", id), RequestID: requestID, Text: text, Status: "pending",
		CreatedBy: by, CreatedName: byName, CreatedAt: time.Now(),
		EmployeeID: r.EmployeeID, ManagerID: r.ManagerID, ClientID: r.ClientID, CounterpartyID: r.CounterpartyID,
	}
	s.Payments = append(s.Payments, p)
	_ = s.saveLocked()
	return s.paymentViewLocked(p, r), nil
}

func (s *Store) MarkPaymentSent(id, by int64, byName string) (PaymentView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Payments {
		if s.Payments[i].ID != id {
			continue
		}
		s.Payments[i].Status = "sent"
		s.Payments[i].SentBy = by
		s.Payments[i].SentName = byName
		s.Payments[i].SentAt = time.Now()
		r, _ := s.requestByIDLocked(s.Payments[i].RequestID)
		_ = s.saveLocked()
		return s.paymentViewLocked(s.Payments[i], r), nil
	}
	return PaymentView{}, fmt.Errorf("оплата не найдена")
}

func (s *Store) PaymentsViews(all bool, userID int64) []PaymentView {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []PaymentView{}
	for _, p := range s.Payments {
		if !all && p.CreatedBy != userID {
			continue
		}
		r, ok := s.requestByIDLocked(p.RequestID)
		if !ok {
			out = append(out, PaymentView{Payment: p})
			continue
		}
		out = append(out, s.paymentViewLocked(p, r))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func (s *Store) PendingPaymentsCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, p := range s.Payments {
		if p.Status != "sent" {
			n++
		}
	}
	return n
}

func (s *Store) AddDealNote(requestID int64, kind, text, byName string, by int64) (DealFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	text = strings.TrimSpace(text)
	if text == "" {
		return DealFile{}, fmt.Errorf("пустой текст")
	}
	if _, ok := s.requestByIDLocked(requestID); !ok {
		return DealFile{}, fmt.Errorf("заявка не найдена")
	}
	if kind != "message" {
		kind = "note"
	}
	id := s.nextLocked()
	f := DealFile{
		ID: id, UID: uid("doc", id), RequestID: requestID, Kind: kind, Text: text,
		Name: "сообщение", CreatedBy: by, CreatedName: byName, CreatedAt: time.Now(),
	}
	s.DealFiles = append(s.DealFiles, f)
	_ = s.saveLocked()
	return f, nil
}

func (s *Store) AddDealFile(requestID int64, name, mime string, data []byte, byName string, by int64, caption string) (DealFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.requestByIDLocked(requestID); !ok {
		return DealFile{}, fmt.Errorf("заявка не найдена")
	}
	if len(data) == 0 {
		return DealFile{}, fmt.Errorf("пустой файл")
	}
	id := s.nextLocked()
	safe := sanitizeFileName(name)
	rel := fmt.Sprintf("%d_%s", id, safe)
	dir := s.FilesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return DealFile{}, err
	}
	full := filepath.Join(dir, rel)
	if err := os.WriteFile(full, data, 0o600); err != nil {
		return DealFile{}, err
	}
	f := DealFile{
		ID: id, UID: uid("doc", id), RequestID: requestID, Kind: "file",
		Name: name, Mime: mime, Size: int64(len(data)), Rel: rel, Text: strings.TrimSpace(caption),
		CreatedBy: by, CreatedName: byName, CreatedAt: time.Now(),
	}
	s.DealFiles = append(s.DealFiles, f)
	_ = s.saveLocked()
	return f, nil
}

func (s *Store) DealFileBytes(id int64) (DealFile, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.DealFiles {
		if f.ID != id {
			continue
		}
		if f.Kind != "file" || f.Rel == "" {
			return f, nil, fmt.Errorf("это не файл")
		}
		data, err := os.ReadFile(filepath.Join(s.FilesDir(), f.Rel))
		return f, data, err
	}
	return DealFile{}, nil, fmt.Errorf("файл не найден")
}

func (s *Store) DealFile(id int64) (DealFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.DealFiles {
		if f.ID == id {
			return f, nil
		}
	}
	return DealFile{}, fmt.Errorf("запись не найдена")
}

func (s *Store) RequestTitle(id int64) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requestByIDLocked(id)
	if !ok {
		return ""
	}
	if strings.TrimSpace(r.Title) != "" {
		return r.Title
	}
	return r.UID
}

func appealKind(kind string) string {
	if kind == "lawyer" {
		return "lawyer"
	}
	return "docs"
}

func appealIsOpen(st string) bool {
	switch st {
	case "taken", "closed", "cancelled":
		return false
	default:
		return true
	}
}

func (s *Store) decorateAppealLocked(a Appeal) Appeal {
	if a.RequestID == 0 {
		return a
	}
	if r, ok := s.requestByIDLocked(a.RequestID); ok {
		a.RequestUID = r.UID
		a.RequestTitle = r.Title
	}
	return a
}

func (s *Store) AddAppeal(requestID int64, kind, text, byName string, by, chatID int64) (Appeal, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	text = strings.TrimSpace(text)
	if text == "" {
		return Appeal{}, "", fmt.Errorf("напишите обращение")
	}
	kind = appealKind(kind)
	ctx := "без заявки"
	if requestID != 0 {
		r, ok := s.requestByIDLocked(requestID)
		if !ok {
			return Appeal{}, "", fmt.Errorf("заявка не найдена")
		}
		ctx = s.requestContextLocked(r)
	}
	id := s.nextLocked()
	a := Appeal{
		ID: id, UID: uid("apl", id), RequestID: requestID, Kind: kind, Text: text,
		ChatID: chatID, CreatedBy: by, CreatedName: byName, CreatedAt: time.Now(), Status: "open",
	}
	s.Appeals = append(s.Appeals, a)
	_ = s.saveLocked()
	return s.decorateAppealLocked(a), ctx, nil
}

func (s *Store) SetAppealSent(id int64, chatID int64, messageID int, sendErr string) Appeal {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Appeals {
		if s.Appeals[i].ID != id {
			continue
		}
		if sendErr != "" {
			s.Appeals[i].Error = sendErr
		} else {
			s.Appeals[i].Error = ""
			s.Appeals[i].ChatID = chatID
			s.Appeals[i].MessageID = messageID
		}
		_ = s.saveLocked()
		return s.decorateAppealLocked(s.Appeals[i])
	}
	return Appeal{}
}

func (s *Store) AppealsList(viewer int64, admin, lawyer, docs bool) []Appeal {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Appeal{}
	for _, a := range s.Appeals {
		if a.Status == "cancelled" {
			continue
		}
		see := admin || a.CreatedBy == viewer
		if a.Kind == "lawyer" && lawyer {
			see = true
		}
		if a.Kind == "docs" && docs {
			see = true
		}
		if !see {
			continue
		}
		out = append(out, s.decorateAppealLocked(a))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func (s *Store) AppealStats() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	y, m, d := now.Date()
	stats := map[string]int{"lawyer_today": 0, "docs_today": 0, "lawyer_open": 0, "docs_open": 0, "lawyer_taken": 0, "docs_taken": 0}
	for _, a := range s.Appeals {
		if a.Status == "cancelled" {
			continue
		}
		key := a.Kind
		if key != "lawyer" {
			key = "docs"
		}
		if a.Status == "taken" {
			stats[key+"_taken"]++
		}
		if appealIsOpen(a.Status) || a.Status == "taken" {
			if a.Status != "taken" {
				stats[key+"_open"]++
			}
		}
		if a.Status == "closed" && !a.ClosedAt.IsZero() {
			cy, cm, cd := a.ClosedAt.Date()
			if cy == y && cm == m && cd == d {
				stats[key+"_today"]++
			}
		}
	}
	return stats
}

func (s *Store) TakeAppeal(id, by int64, name string, admin, lawyer, docs bool) (Appeal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Appeals {
		if s.Appeals[i].ID != id {
			continue
		}
		if !appealIsOpen(s.Appeals[i].Status) {
			return Appeal{}, fmt.Errorf("уже взята или закрыта")
		}
		if s.Appeals[i].Kind == "lawyer" && !admin && !lawyer {
			return Appeal{}, fmt.Errorf("это очередь юриста")
		}
		if s.Appeals[i].Kind != "lawyer" && !admin && !docs {
			return Appeal{}, fmt.Errorf("это очередь документалиста")
		}
		s.Appeals[i].Status = "taken"
		s.Appeals[i].TakenBy = by
		s.Appeals[i].TakenName = name
		s.Appeals[i].TakenAt = time.Now()
		_ = s.saveLocked()
		return s.decorateAppealLocked(s.Appeals[i]), nil
	}
	return Appeal{}, fmt.Errorf("обращение не найдено")
}

func (s *Store) CloseAppeal(id, by int64, name string, admin, lawyer, docs bool) (Appeal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Appeals {
		if s.Appeals[i].ID != id {
			continue
		}
		if s.Appeals[i].Status == "closed" || s.Appeals[i].Status == "cancelled" {
			return Appeal{}, fmt.Errorf("уже закрыта")
		}
		if s.Appeals[i].Kind == "lawyer" && !admin && !lawyer && s.Appeals[i].TakenBy != by {
			return Appeal{}, fmt.Errorf("закрыть может тот, кто взял, или админ")
		}
		if s.Appeals[i].Kind != "lawyer" && !admin && !docs && s.Appeals[i].TakenBy != by {
			return Appeal{}, fmt.Errorf("закрыть может тот, кто взял, или админ")
		}
		if s.Appeals[i].Kind == "lawyer" && !admin && !lawyer {
			return Appeal{}, fmt.Errorf("это очередь юриста")
		}
		if s.Appeals[i].Kind != "lawyer" && !admin && !docs {
			return Appeal{}, fmt.Errorf("это очередь документалиста")
		}
		s.Appeals[i].Status = "closed"
		s.Appeals[i].ClosedBy = by
		s.Appeals[i].ClosedName = name
		s.Appeals[i].ClosedAt = time.Now()
		_ = s.saveLocked()
		return s.decorateAppealLocked(s.Appeals[i]), nil
	}
	return Appeal{}, fmt.Errorf("обращение не найдено")
}

func (s *Store) Appeal(id int64) (Appeal, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.appealByIDLocked(id)
}

func (s *Store) appealByIDLocked(id int64) (Appeal, bool) {
	for _, a := range s.Appeals {
		if a.ID == id {
			return s.decorateAppealLocked(a), true
		}
	}
	return Appeal{}, false
}

func (s *Store) AppealChats() (lawyer, docs int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.LawyerChatID, s.DocsChatID
}

func (s *Store) RemapChatID(from, to int64) bool {
	if from == 0 || to == 0 || from == to {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	if s.LawyerChatID == from {
		s.LawyerChatID = to
		changed = true
	}
	if s.DocsChatID == from {
		s.DocsChatID = to
		changed = true
	}
	for i := range s.Chats {
		if s.Chats[i].ChatID == from {
			s.Chats[i].ChatID = to
			s.Chats[i].UpdatedAt = time.Now()
			changed = true
		}
	}
	for i := range s.Appeals {
		if s.Appeals[i].ChatID == from {
			s.Appeals[i].ChatID = to
			changed = true
		}
	}
	if !changed {
		return false
	}
	_ = s.saveLocked()
	return true
}

func (s *Store) SetAppealChat(kind string, id int64) (int64, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if appealKind(kind) == "lawyer" {
		s.LawyerChatID = id
	} else {
		s.DocsChatID = id
	}
	return s.LawyerChatID, s.DocsChatID, s.saveLocked()
}

func (s *Store) SetAppealNotify(id int64, mid int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Appeals {
		if s.Appeals[i].ID != id {
			continue
		}
		s.Appeals[i].NotifyMID = mid
		_ = s.saveLocked()
		return
	}
}

func (s *Store) AppealByChatMsg(chatID int64, mid int) (Appeal, bool) {
	if chatID == 0 {
		return Appeal{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if mid != 0 {
		for _, a := range s.Appeals {
			if a.ChatID == chatID && a.MessageID == mid {
				return s.decorateAppealLocked(a), true
			}
		}
	}
	var found Appeal
	n := 0
	for _, a := range s.Appeals {
		if a.ChatID != chatID || a.Status == "closed" || a.Status == "cancelled" {
			continue
		}
		found = a
		n++
	}
	if n == 1 {
		return s.decorateAppealLocked(found), true
	}
	return Appeal{}, false
}

func (s *Store) AppealByNotify(userID int64, mid int) (Appeal, bool) {
	if userID == 0 || mid == 0 {
		return Appeal{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.Appeals {
		if a.CreatedBy == userID && a.NotifyMID == mid {
			return s.decorateAppealLocked(a), true
		}
	}
	return Appeal{}, false
}

func sanitizeFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "._")
	if out == "" {
		return "file"
	}
	if len(out) > 80 {
		out = out[len(out)-80:]
	}
	return out
}

func (s *Store) DeletePayment(id, by int64, _ bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.Payments {
		if s.Payments[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("оплата не найдена")
	}
	if s.Payments[idx].CreatedBy != by {
		return fmt.Errorf("удалить может только тот, кто отправил")
	}
	s.Payments = append(s.Payments[:idx], s.Payments[idx+1:]...)
	return s.saveLocked()
}

func (s *Store) ReturnPayment(id, by int64, _ bool) (PaymentView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Payments {
		if s.Payments[i].ID != id {
			continue
		}
		if s.Payments[i].CreatedBy != by {
			return PaymentView{}, fmt.Errorf("вернуть может только тот, кто отправил")
		}
		s.Payments[i].Status = "pending"
		s.Payments[i].SentBy = 0
		s.Payments[i].SentName = ""
		s.Payments[i].SentAt = time.Time{}
		r, _ := s.requestByIDLocked(s.Payments[i].RequestID)
		_ = s.saveLocked()
		return s.paymentViewLocked(s.Payments[i], r), nil
	}
	return PaymentView{}, fmt.Errorf("оплата не найдена")
}

func (s *Store) DeleteDealFile(id, by int64, _ bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.DealFiles {
		if s.DealFiles[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("запись не найдена")
	}
	if s.DealFiles[idx].CreatedBy != by {
		return fmt.Errorf("удалить может только тот, кто добавил")
	}
	rel := s.DealFiles[idx].Rel
	s.DealFiles = append(s.DealFiles[:idx], s.DealFiles[idx+1:]...)
	if rel != "" {
		_ = os.Remove(filepath.Join(s.FilesDir(), rel))
	}
	return s.saveLocked()
}

func (s *Store) CancelAppeal(id, by int64, _ bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Appeals {
		if s.Appeals[i].ID != id {
			continue
		}
		if s.Appeals[i].CreatedBy != by {
			return fmt.Errorf("удалить может только тот, кто отправил")
		}
		s.Appeals[i].Status = "cancelled"
		_ = s.saveLocked()
		return nil
	}
	return fmt.Errorf("обращение не найдено")
}

type AttachWait struct {
	UserID    int64     `json:"user_id"`
	Kind      string    `json:"kind,omitempty"`
	RequestID int64     `json:"request_id,omitempty"`
	Scope     string    `json:"scope,omitempty"`
	OwnerID   int64     `json:"owner_id,omitempty"`
	Folder    string    `json:"folder,omitempty"`
	Label     string    `json:"label,omitempty"`
	Until     time.Time `json:"until"`
}

func (w AttachWait) IsVault() bool {
	return w.Kind == "vault"
}

func (s *Store) SetAttachWait(userID, requestID int64, d time.Duration) (RequestView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requestByIDLocked(requestID)
	if !ok || r.Status == "deleted" {
		return RequestView{}, fmt.Errorf("заявка не найдена")
	}
	until := time.Now().Add(d)
	out := []AttachWait{}
	for _, w := range s.AttachWaits {
		if w.UserID == userID || w.Until.Before(time.Now()) {
			continue
		}
		out = append(out, w)
	}
	out = append(out, AttachWait{UserID: userID, Kind: "deal", RequestID: requestID, Until: until})
	s.AttachWaits = out
	_ = s.saveLocked()
	return s.viewRequestLocked(r), nil
}

func (s *Store) SetVaultAttachWait(userID int64, scope string, ownerID int64, folder string, d time.Duration) (AttachWait, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scope = normVaultScope(scope)
	folder = normVaultFolder(scope, folder)
	if err := s.vaultPlaceOKLocked(scope, ownerID); err != nil {
		return AttachWait{}, err
	}
	until := time.Now().Add(d)
	out := []AttachWait{}
	for _, w := range s.AttachWaits {
		if w.UserID == userID || w.Until.Before(time.Now()) {
			continue
		}
		out = append(out, w)
	}
	rec := AttachWait{
		UserID: userID, Kind: "vault", Scope: scope, OwnerID: ownerID, Folder: folder,
		Label: s.vaultLabelLocked(scope, ownerID, folder), Until: until,
	}
	out = append(out, rec)
	s.AttachWaits = out
	_ = s.saveLocked()
	return rec, nil
}

func (s *Store) AttachWaitOf(userID int64) (AttachWait, RequestView, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for _, w := range s.AttachWaits {
		if w.UserID != userID || w.Until.Before(now) {
			continue
		}
		if w.IsVault() {
			label := w.Label
			if label == "" {
				label = s.vaultLabelLocked(w.Scope, w.OwnerID, w.Folder)
			}
			view := RequestView{}
			view.Title = label
			view.UID = "docs"
			return w, view, true
		}
		r, ok := s.requestByIDLocked(w.RequestID)
		if !ok || r.Status == "deleted" {
			return AttachWait{}, RequestView{}, false
		}
		return w, s.viewRequestLocked(r), true
	}
	return AttachWait{}, RequestView{}, false
}

func (s *Store) ClearAttachWait(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []AttachWait{}
	now := time.Now()
	for _, w := range s.AttachWaits {
		if w.UserID == userID || w.Until.Before(now) {
			continue
		}
		out = append(out, w)
	}
	s.AttachWaits = out
	_ = s.saveLocked()
}

type SentBatch struct {
	UserID int64     `json:"user_id"`
	IDs    []int64   `json:"ids"`
	Kind   string    `json:"kind,omitempty"`
	At     time.Time `json:"at"`
}

func (s *Store) RememberSent(userID int64, ids []int64) {
	s.RememberSentKind(userID, ids, "")
}

func (s *Store) RememberVaultSent(userID int64, ids []int64) {
	s.RememberSentKind(userID, ids, "vault")
}

func (s *Store) RememberSentKind(userID int64, ids []int64, kind string) {
	if userID == 0 || len(ids) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []SentBatch{}
	for _, b := range s.LastSent {
		if b.UserID == userID {
			continue
		}
		out = append(out, b)
	}
	out = append(out, SentBatch{UserID: userID, IDs: append([]int64{}, ids...), Kind: kind, At: time.Now()})
	s.LastSent = out
	_ = s.saveLocked()
}

func (s *Store) UndoLastSent(userID int64) (int, error) {
	s.mu.Lock()
	ids := []int64{}
	kind := ""
	kept := []SentBatch{}
	found := false
	for _, b := range s.LastSent {
		if !found && b.UserID == userID {
			ids = append([]int64{}, b.IDs...)
			kind = b.Kind
			found = true
			continue
		}
		kept = append(kept, b)
	}
	if !found {
		var bestDeal DealFile
		for _, f := range s.DealFiles {
			if f.CreatedBy != userID {
				continue
			}
			if bestDeal.ID == 0 || f.CreatedAt.After(bestDeal.CreatedAt) || (f.CreatedAt.Equal(bestDeal.CreatedAt) && f.ID > bestDeal.ID) {
				bestDeal = f
			}
		}
		var bestVault VaultFile
		for _, f := range s.VaultFiles {
			if f.CreatedBy != userID {
				continue
			}
			if bestVault.ID == 0 || f.CreatedAt.After(bestVault.CreatedAt) || (f.CreatedAt.Equal(bestVault.CreatedAt) && f.ID > bestVault.ID) {
				bestVault = f
			}
		}
		useVault := bestVault.ID != 0 && (bestDeal.ID == 0 || bestVault.CreatedAt.After(bestDeal.CreatedAt) || (bestVault.CreatedAt.Equal(bestDeal.CreatedAt) && bestVault.ID > bestDeal.ID))
		if useVault {
			ids = []int64{bestVault.ID}
			kind = "vault"
		} else if bestDeal.ID != 0 {
			ids = []int64{bestDeal.ID}
		}
	}
	s.LastSent = kept
	_ = s.saveLocked()
	s.mu.Unlock()
	if len(ids) == 0 {
		return 0, fmt.Errorf("нечего отменять — нет вашей последней отправки")
	}
	n := 0
	for _, id := range ids {
		if kind == "vault" {
			if err := s.DeleteVaultFile(id, userID, false); err == nil {
				n++
			}
			continue
		}
		if err := s.DeleteDealFile(id, userID, false); err == nil {
			n++
		}
	}
	if n == 0 {
		return 0, fmt.Errorf("уже удалено или это отправили не вы")
	}
	return n, nil
}

func (s *Store) DeleteOwnOnRequest(requestID, userID int64) (int, error) {
	if userID == 0 {
		return 0, fmt.Errorf("нет пользователя")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if requestID == 0 {
		for _, w := range s.AttachWaits {
			if w.UserID == userID && w.Until.After(now) {
				if w.IsVault() {
					n, err := s.deleteOwnVaultLocked(w.Scope, w.OwnerID, w.Folder, userID)
					if err != nil {
						return 0, err
					}
					return n, s.saveLocked()
				}
				requestID = w.RequestID
				break
			}
		}
	}
	if requestID == 0 {
		var best DealFile
		for _, f := range s.DealFiles {
			if f.CreatedBy != userID {
				continue
			}
			if best.ID == 0 || f.CreatedAt.After(best.CreatedAt) || (f.CreatedAt.Equal(best.CreatedAt) && f.ID > best.ID) {
				best = f
			}
		}
		requestID = best.RequestID
	}
	if requestID == 0 {
		return 0, fmt.Errorf("нет ваших файлов или сообщений")
	}
	kept := []DealFile{}
	rels := []string{}
	n := 0
	idGone := map[int64]bool{}
	for _, f := range s.DealFiles {
		if f.RequestID == requestID && f.CreatedBy == userID {
			n++
			idGone[f.ID] = true
			if f.Rel != "" {
				rels = append(rels, f.Rel)
			}
			continue
		}
		kept = append(kept, f)
	}
	if n == 0 {
		return 0, fmt.Errorf("нет ваших файлов или сообщений в этой заявке")
	}
	s.DealFiles = kept
	batches := []SentBatch{}
	for _, b := range s.LastSent {
		if b.UserID != userID {
			batches = append(batches, b)
			continue
		}
		left := []int64{}
		for _, id := range b.IDs {
			if !idGone[id] {
				left = append(left, id)
			}
		}
		if len(left) > 0 {
			b.IDs = left
			batches = append(batches, b)
		}
	}
	s.LastSent = batches
	dir := s.FilesDir()
	for _, rel := range rels {
		_ = os.Remove(filepath.Join(dir, rel))
	}
	_ = s.saveLocked()
	return n, nil
}
