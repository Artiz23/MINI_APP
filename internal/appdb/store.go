package appdb

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"tg-bot-orh3/internal/saldo"
)

type Role string

const (
	RoleOwner    Role = "owner"
	RoleOperator Role = "operator"
)

type User struct {
	ID                 int64                `json:"id"`
	Name               string               `json:"name"`
	Username           string               `json:"username"`
	Role               Role                 `json:"role"`
	Access             Access               `json:"access,omitempty"`
	BalanceResponsible bool                 `json:"balance_responsible"`
	AddedAt            time.Time            `json:"added_at"`
	Seen               map[string]time.Time `json:"seen,omitempty"`
	Inbox              map[string]int       `json:"inbox,omitempty"`
	Hidden             map[string]bool      `json:"hidden,omitempty"`
	Notify             Notify               `json:"notify,omitempty"`
	Dirs               []string             `json:"dirs,omitempty"`
}

type Request struct {
	ClosedAt         time.Time        `json:"closed_at,omitempty"`
	AgentID          int64            `json:"agent_id,omitempty"`
	Economics        RequestEconomics `json:"economics"`
	Comment          string           `json:"comment,omitempty"`
	CommentUpdatedAt time.Time        `json:"comment_updated_at,omitempty"`
	CommentUpdatedBy int64            `json:"comment_updated_by,omitempty"`
	CloseReason      string           `json:"close_reason,omitempty"`
	WorkflowStage    RequestStage     `json:"workflow_stage,omitempty"`
	ID               int64            `json:"id"`
	UID              string           `json:"uid"`
	Title            string           `json:"title"`
	Status           string           `json:"status"`
	ThreadID         int64            `json:"thread_id,omitempty"`
	ThreadLink       string           `json:"thread_link,omitempty"`
	CreatedBy        int64            `json:"created_by"`
	CreatedName      string           `json:"created_name"`
	CreatedAt        time.Time        `json:"created_at"`
	TableRef         string           `json:"table_ref,omitempty"`
	Notes            string           `json:"notes,omitempty"`
	EmployeeID       int64            `json:"employee_id,omitempty"`
	ManagerID        int64            `json:"manager_id,omitempty"`
	ClientID         int64            `json:"client_id,omitempty"`
	CounterpartyID   int64            `json:"counterparty_id,omitempty"`
}

type Approval struct {
	Delivery
	ID          int64     `json:"id"`
	UID         string    `json:"uid"`
	RequestUID  string    `json:"request_uid,omitempty"`
	RequestID   int64     `json:"request_id,omitempty"`
	Preview     string    `json:"preview"`
	Status      string    `json:"status"`
	ManagerName string    `json:"manager_name"`
	ManagerID   int64     `json:"manager_id"`
	CreatedAt   time.Time `json:"created_at"`
	DecidedBy   string    `json:"decided_by,omitempty"`
	DecidedAt   time.Time `json:"decided_at,omitempty"`
}

type SaldoOp struct {
	ID               int64     `json:"id"`
	UID              string    `json:"uid"`
	CP               string    `json:"cp"`
	Kind             string    `json:"kind"`
	Currency         string    `json:"currency"`
	Action           string    `json:"action"`
	Amount           float64   `json:"amount"`
	Manager          string    `json:"manager"`
	ManagerID        int64     `json:"manager_id"`
	CreatedAt        time.Time `json:"created_at"`
	CloseLotOpID     int64     `json:"close_lot_op_id,omitempty"`
	RequestID        int64     `json:"request_id,omitempty"`
	EmployeeID       int64     `json:"employee_id,omitempty"`
	ClientID         int64     `json:"client_id,omitempty"`
	CounterpartyID   int64     `json:"counterparty_id,omitempty"`
	CatalogManagerID int64     `json:"catalog_manager_id,omitempty"`
}

type SaldoLot struct {
	OpID         int64     `json:"op_id"`
	UID          string    `json:"uid"`
	CP           string    `json:"cp"`
	Kind         string    `json:"kind"`
	Currency     string    `json:"currency"`
	Amount       float64   `json:"amount"`
	Remaining    float64   `json:"remaining"`
	Manager      string    `json:"manager"`
	ManagerID    int64     `json:"manager_id"`
	CreatedAt    time.Time `json:"created_at"`
	RequestID    int64     `json:"request_id,omitempty"`
	RequestTitle string    `json:"request_title,omitempty"`
}

type Company struct {
	ID   int64  `json:"id"`
	UID  string `json:"uid"`
	Name string `json:"name"`
}

type Account struct {
	ID        int64     `json:"id"`
	UID       string    `json:"uid"`
	CompanyID int64     `json:"company_id"`
	Bank      string    `json:"bank"`
	Currency  string    `json:"currency"`
	Amount    float64   `json:"amount"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by"`
}

type ComplianceQ struct {
	Key    string `json:"key"`
	Text   string `json:"text"`
	Answer string `json:"answer,omitempty"`
}

type Compliance struct {
	ID             int64         `json:"id"`
	UID            string        `json:"uid"`
	Subject        string        `json:"subject"`
	Status         string        `json:"status"`
	Questions      []ComplianceQ `json:"questions,omitempty"`
	Text           string        `json:"text,omitempty"`
	CreatedBy      int64         `json:"created_by"`
	CreatedName    string        `json:"created_name"`
	CreatedAt      time.Time     `json:"created_at"`
	DueAt          time.Time     `json:"due_at"`
	RequestUID     string        `json:"request_uid,omitempty"`
	RequestID      int64         `json:"request_id,omitempty"`
	EmployeeID     int64         `json:"employee_id,omitempty"`
	ManagerID      int64         `json:"manager_id,omitempty"`
	ClientID       int64         `json:"client_id,omitempty"`
	CounterpartyID int64         `json:"counterparty_id,omitempty"`
}

type DisputeMsg struct {
	ID     int64     `json:"id"`
	By     int64     `json:"by"`
	ByName string    `json:"by_name"`
	Text   string    `json:"text"`
	At     time.Time `json:"at"`
}

type Dispute struct {
	ID          int64        `json:"id"`
	UID         string       `json:"uid"`
	Section     string       `json:"section"`
	RefUID      string       `json:"ref_uid"`
	Text        string       `json:"text"`
	Status      string       `json:"status"`
	CreatedBy   int64        `json:"created_by"`
	CreatedName string       `json:"created_name,omitempty"`
	TargetID    int64        `json:"target_id,omitempty"`
	TargetName  string       `json:"target_name,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	LastAt      time.Time    `json:"last_at,omitempty"`
	Messages    []DisputeMsg `json:"messages,omitempty"`
}

type Store struct {
	mu                sync.Mutex
	path              string
	SchemaVersion     int             `json:"schema_version"`
	MigrationWarnings []string        `json:"migration_warnings,omitempty"`
	AuditEvents       []AuditEvent    `json:"audit_events,omitempty"`
	Settings          MiniAppSettings `json:"settings"`
	Agents            []Agent         `json:"agents,omitempty"`
	ActivitySlices    []ActivitySlice `json:"activity_slices,omitempty"`
	Meetings          []Meeting       `json:"meetings,omitempty"`

	Users             []User            `json:"users"`
	Requests          []Request         `json:"requests"`
	Approvals         []Approval        `json:"approvals"`
	SaldoOps          []SaldoOp         `json:"saldo_ops"`
	SaldoBalances     []SaldoBalance    `json:"saldo_balances,omitempty"`
	Companies         []Company         `json:"companies"`
	Accounts          []Account         `json:"accounts"`
	Compliances       []Compliance      `json:"compliances"`
	Disputes          []Dispute         `json:"disputes"`
	Payments          []Payment         `json:"payments,omitempty"`
	DealFiles         []DealFile        `json:"deal_files,omitempty"`
	Appeals           []Appeal          `json:"appeals,omitempty"`
	Tasks             []Task            `json:"tasks,omitempty"`
	AttachWaits       []AttachWait      `json:"attach_waits,omitempty"`
	Employees         []Employee        `json:"employees,omitempty"`
	Managers          []Manager         `json:"managers,omitempty"`
	Clients           []Client          `json:"clients,omitempty"`
	Counterparties    []Counterparty    `json:"counterparties,omitempty"`
	Positions         []Position        `json:"positions,omitempty"`
	SaldoCPs          []string          `json:"saldo_cps,omitempty"`
	SaldoCurrencies   []string          `json:"saldo_currencies,omitempty"`
	NextID            int64             `json:"next_id"`
	MorningOK         string            `json:"morning_ok,omitempty"`
	EveningOK         string            `json:"evening_ok,omitempty"`
	LastEveningDigest map[int64]string  `json:"last_evening_digest,omitempty"`
	LastHolidayDay    map[int64]string  `json:"last_holiday_day,omitempty"`
	LastSent          []SentBatch       `json:"last_sent,omitempty"`
	LawyerChatID      int64             `json:"lawyer_chat_id,omitempty"`
	DocsChatID        int64             `json:"docs_chat_id,omitempty"`
	Chats             []ManagedChat     `json:"chats,omitempty"`
	VaultFiles        []VaultFile       `json:"vault_files,omitempty"`
	VaultTabs         []CompanyVaultTab `json:"vault_tabs,omitempty"`
}

func Load(path string) (*Store, error) {
	s := &Store{path: path, NextID: 1}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.SchemaVersion = CurrentSchemaVersion
			s.seed()
			return s, s.saveLocked()
		}
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	s.path = path
	if s.SchemaVersion > CurrentSchemaVersion {
		return nil, fmt.Errorf("schema version %d is newer than supported %d", s.SchemaVersion, CurrentSchemaVersion)
	}
	migrated := s.SchemaVersion < CurrentSchemaVersion
	if migrated {
		if err := backupBeforeMigration(path, data); err != nil {
			return nil, err
		}
		s.migrateSchemaLocked()
	}
	if s.NextID == 0 {
		s.NextID = 1
	}
	changed := migrated
	if len(s.Companies) == 0 {
		s.seedCompanies()
		changed = true
	}
	if len(s.SaldoCPs) == 0 {
		s.SaldoCPs = append([]string{}, saldo.DefaultCounterparties()...)
		changed = true
	}
	if len(s.SaldoCurrencies) == 0 {
		s.SaldoCurrencies = append([]string{}, saldo.DefaultCurrencies()...)
		changed = true
	}
	if s.migrateCatalogLocked() {
		changed = true
	}
	if s.seedManagedChatsLocked() {
		changed = true
	}
	if s.Tasks == nil {
		s.Tasks = []Task{}
	}
	for i := range s.Users {
		if s.Users[i].Role == RoleOwner {
			s.Users[i].Access = AllAccess()
			continue
		}
		if len(s.Users[i].Access) == 0 {
			s.Users[i].Access = DefaultAccess()
			changed = true
		} else {
			if _, ok := s.Users[i].Access[SecTasks]; !ok {
				s.Users[i].Access[SecTasks] = true
				changed = true
			}
			s.Users[i].Access = NormalizeAccess(s.Users[i].Access)
		}
	}
	if migrated {
		s.SaldoBalances = s.rebuildBalancesLocked()
	}
	if !s.saldoReconcilesLocked() {
		log.Printf("miniapp: saldo balances differ from journal; admin rebuild required")
	}
	if changed {
		if err := s.saveLocked(); err != nil {
			return s, err
		}
	}
	return s, nil
}

func (s *Store) seed() {
	s.seedCompanies()
	s.SaldoCPs = append([]string{}, saldo.DefaultCounterparties()...)
	s.SaldoCurrencies = append([]string{}, saldo.DefaultCurrencies()...)
}

func (s *Store) seedCompanies() {
	names := []string{"Intermar", "Everrock", "SIM (Кипр)", "Solventa (Кипр)"}
	for _, n := range names {
		id := s.nextLocked()
		s.Companies = append(s.Companies, Company{ID: id, UID: uid("cmp", id), Name: n})
	}
}

func uid(prefix string, id int64) string {
	return fmt.Sprintf("%s-%d", prefix, id)
}

func (s *Store) nextLocked() int64 {
	id := s.NextID
	if id == 0 {
		id = 1
	}
	s.NextID = id + 1
	return id
}

func (s *Store) saveLocked() (err error) {
	stages := make([]RequestStage, len(s.Requests))
	for i, r := range s.Requests {
		stages[i] = r.WorkflowStage
	}
	defer func() {
		if err != nil {
			for i, stage := range stages {
				s.Requests[i].WorkflowStage = stage
			}
		}
	}()
	s.syncWorkflowLocked()
	if err = os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err = os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) UpsertUser(u User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Users {
		if s.Users[i].ID == u.ID {
			if u.Name != "" {
				s.Users[i].Name = u.Name
			}
			if u.Username != "" {
				s.Users[i].Username = u.Username
			}
			if u.Role == RoleOwner {
				s.Users[i].Role = RoleOwner
				s.Users[i].Access = AllAccess()
			}
			_ = s.saveLocked()
			return
		}
	}
	if u.AddedAt.IsZero() {
		u.AddedAt = time.Now()
	}
	if u.Role == RoleOwner {
		u.Access = AllAccess()
	} else if len(u.Access) == 0 {
		u.Access = DefaultAccess()
	} else {
		u.Access = NormalizeAccess(u.Access)
	}
	s.Users = append(s.Users, u)
	_ = s.saveLocked()
}

func (s *Store) User(id int64) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.Users {
		if u.ID == id {
			return u, true
		}
	}
	return User{}, false
}

func (s *Store) userLocked(id int64) (User, bool) {
	for _, u := range s.Users {
		if u.ID == id {
			return u, true
		}
	}
	return User{}, false
}

func (s *Store) AccessPool() []User {
	s.mu.Lock()
	defer s.mu.Unlock()
	taken := map[int64]bool{}
	for _, e := range s.Employees {
		if e.TelegramID != 0 {
			taken[e.TelegramID] = true
		}
	}
	out := make([]User, 0, len(s.Users))
	for _, u := range s.Users {
		if taken[u.ID] {
			continue
		}
		cp := u
		cp.Access = u.Access.Copy()
		cp.Dirs = append([]string(nil), u.Dirs...)
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func (s *Store) UsersCopy() []User {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]User(nil), s.Users...)
	if out == nil {
		out = []User{}
	}
	for i := range out {
		out[i].Access = out[i].Access.Copy()
		out[i].Dirs = append([]string(nil), out[i].Dirs...)
	}
	return out
}

func (s *Store) SetUserRole(id int64, role Role, balanceResp bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Users {
		if s.Users[i].ID == id {
			if s.Users[i].Role == RoleOwner {
				s.Users[i].BalanceResponsible = balanceResp
				return s.saveLocked()
			}
			s.Users[i].Role = role
			s.Users[i].BalanceResponsible = balanceResp
			return s.saveLocked()
		}
	}
	return fmt.Errorf("пользователь %d не найден", id)
}

func (s *Store) SaveUser(u User) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u.ID == 0 {
		return User{}, fmt.Errorf("нужен id")
	}
	if u.Name == "" {
		u.Name = fmt.Sprintf("%d", u.ID)
	}
	if u.Role == RoleOwner || u.Role == RoleAdmin {
		u.Access = AllAccess()
	} else {
		u.Access = NormalizeAccess(u.Access)
		u.Role = RoleOperator
	}
	u.Dirs = NormalizeDirs(u.Dirs)
	if u.AddedAt.IsZero() {
		u.AddedAt = time.Now()
	}
	for i := range s.Users {
		if s.Users[i].ID != u.ID {
			continue
		}
		if s.Users[i].Role == RoleOwner {
			s.Users[i].Name = u.Name
			if u.Username != "" {
				s.Users[i].Username = u.Username
			}
			s.Users[i].Access = AllAccess()
			s.Users[i].Role = RoleOwner
			s.Users[i].Dirs = NormalizeDirs(u.Dirs)
			s.Users[i].Notify = NormalizeNotify(s.Users[i].Notify)
			s.linkUserToEmployeeLocked(s.Users[i].ID, s.Users[i].Name)
			_ = s.saveLocked()
			return s.Users[i], nil
		}
		u.AddedAt = s.Users[i].AddedAt
		if u.Username == "" {
			u.Username = s.Users[i].Username
		}
		u.Seen = s.Users[i].Seen
		u.Inbox = s.Users[i].Inbox
		u.Hidden = s.Users[i].Hidden
		u.Notify = s.Users[i].Notify
		s.Users[i] = u
		s.linkUserToEmployeeLocked(u.ID, u.Name)
		_ = s.saveLocked()
		return s.Users[i], nil
	}
	s.Users = append(s.Users, u)
	s.linkUserToEmployeeLocked(u.ID, u.Name)
	_ = s.saveLocked()
	return u, nil
}

func (s *Store) AddUser(id int64, name string, role Role) User {
	u, _ := s.SaveUser(User{ID: id, Name: name, Role: role, Access: DefaultAccess()})
	return u
}

func (s *Store) RemoveUser(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.Users[:0]
	found := false
	for _, u := range s.Users {
		if u.ID == id {
			if u.Role == RoleOwner {
				return fmt.Errorf("у владельца нельзя забрать доступ")
			}
			found = true
			continue
		}
		out = append(out, u)
	}
	if !found {
		return fmt.Errorf("пользователь не найден")
	}
	s.Users = out
	return s.saveLocked()
}

func (s *Store) BalanceResponsibleIDs() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []int64
	for _, u := range s.Users {
		if u.BalanceResponsible {
			out = append(out, u.ID)
		}
	}
	return out
}

func (s *Store) AddRequest(title, name string, by int64, threadID int64, link string, managerID, clientID, counterpartyID int64, staff bool, manualUID string) (RequestView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mgr, ok := s.managerLocked(managerID)
	if !ok {
		return RequestView{}, fmt.Errorf("выберите менеджера из справочника")
	}
	if _, ok := s.clientLocked(clientID); !ok {
		return RequestView{}, fmt.Errorf("выберите клиента из справочника")
	}
	if _, ok := s.counterpartyLocked(counterpartyID); !ok {
		return RequestView{}, fmt.Errorf("выберите контрагента из справочника")
	}
	emp, ok := s.actorEmployeeLocked(by, name, staff)
	if !ok {
		return RequestView{}, fmt.Errorf("вас нет в справочнике сотрудников. Админ добавит карточку с номером — Telegram ID подтянется из Доступов сам")
	}
	now := time.Now()
	code := normalizeManualUID(manualUID)
	if code != "" {
		if len([]rune(code)) < 2 {
			return RequestView{}, fmt.Errorf("номер заявки слишком короткий")
		}
		if s.dealCodeTakenLocked(code) {
			return RequestView{}, fmt.Errorf("такой номер заявки уже есть")
		}
	} else {
		var err error
		code, err = s.nextDealCodeLocked(mgr, emp, now)
		if err != nil {
			return RequestView{}, err
		}
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = code
	}
	id := s.nextLocked()
	r := Request{
		ID: id, UID: code, Title: title, Status: "open", WorkflowStage: StageApproval,
		ThreadID: threadID, ThreadLink: link, CreatedBy: by, CreatedName: name, CreatedAt: now,
		EmployeeID: emp.ID, ManagerID: managerID, ClientID: clientID, CounterpartyID: counterpartyID,
	}
	if title != code {
		r.Comment = title
	}
	s.Requests = append(s.Requests, r)
	if err := s.saveLocked(); err != nil {
		s.Requests = s.Requests[:len(s.Requests)-1]
		s.NextID = id
		return RequestView{}, err
	}
	return s.viewRequestLocked(r), nil
}

func (s *Store) SetRequestStatus(id int64, status, tableRef, notes string, by int64) (Request, error) {
	return s.UpdateRequestState(id, status, notes, by, false)
}

func (s *Store) DeleteRequest(id, by int64, admin bool) error {
	_, err := s.UpdateRequestState(id, "deleted", "", by, admin)
	return err
}

func (s *Store) canTouchRequestLocked(r Request, by int64, admin bool) bool {
	return s.requestPermissionLocked(r, by, admin).CanEdit
}

func (s *Store) RequestsCopy() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Request(nil), s.Requests...)
	if out == nil {
		out = []Request{}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func (s *Store) AddApproval(preview, mgrName string, mgrID int64, reqUID string, requestID int64) Approval {
	s.mu.Lock()
	defer s.mu.Unlock()
	if requestID != 0 {
		for _, r := range s.Requests {
			if r.ID == requestID {
				reqUID = r.UID
				if strings.TrimSpace(preview) == "" {
					preview = s.requestContextLocked(r)
				}
				break
			}
		}
	}
	id := s.nextLocked()
	a := Approval{
		ID: id, UID: uid("apr", id), RequestUID: reqUID, RequestID: requestID, Preview: preview,
		Status: "pending", ManagerName: mgrName, ManagerID: mgrID, CreatedAt: time.Now(),
	}
	s.Approvals = append(s.Approvals, a)
	_ = s.saveLocked()
	return a
}

func (s *Store) DecideApproval(id int64, status, by string) (Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if status != "approved" && status != "rejected" {
		return Approval{}, fmt.Errorf("неверное решение")
	}
	for i, old := range s.Approvals {
		if old.ID != id {
			continue
		}
		if old.Status != "pending" {
			return Approval{}, fmt.Errorf("решение уже принято")
		}
		s.Approvals[i].Status = status
		s.Approvals[i].DecidedBy = by
		s.Approvals[i].DecidedAt = time.Now()
		if err := s.saveLocked(); err != nil {
			s.Approvals[i] = old
			return Approval{}, err
		}
		return s.Approvals[i], nil
	}
	return Approval{}, fmt.Errorf("согласование не найдено")
}

func (s *Store) RecallApproval(id, by int64, _ bool, status string) (Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status = strings.TrimSpace(status)
	if status != "withdrawn" && status != "pending" {
		return Approval{}, fmt.Errorf("можно отозвать или вернуть в очередь")
	}
	for i := range s.Approvals {
		if s.Approvals[i].ID != id {
			continue
		}
		if s.Approvals[i].ManagerID != by {
			return Approval{}, fmt.Errorf("вернуть или удалить может только тот, кто отправил")
		}
		if status == "withdrawn" && s.Approvals[i].Status != "pending" {
			return Approval{}, fmt.Errorf("отозвать можно только очередь")
		}
		s.Approvals[i].Status = status
		if status == "pending" {
			s.Approvals[i].DecidedBy = ""
			s.Approvals[i].DecidedAt = time.Time{}
		}
		_ = s.saveLocked()
		return s.Approvals[i], nil
	}
	return Approval{}, fmt.Errorf("согласование не найдено")
}

func (s *Store) ApprovalsCopy() []Approval {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Approval(nil), s.Approvals...)
	if out == nil {
		out = []Approval{}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func (s *Store) ApprovalsFor(userID int64) []Approval {
	s.mu.Lock()
	defer s.mu.Unlock()
	hidden := s.hiddenLocked(userID)
	out := []Approval{}
	for _, a := range s.Approvals {
		if hidden[fmt.Sprintf("apr:%d", a.ID)] {
			continue
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func (s *Store) ApprovalByUIDLocked(uid string) (Approval, bool) {
	for _, a := range s.Approvals {
		if a.UID == uid {
			return a, true
		}
	}
	return Approval{}, false
}

func KindLabel(k string) string {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case "swift":
		return "SWIFT"
	case "nerez":
		return "Нерезидентский рубль"
	default:
		return k
	}
}

func (s *Store) SaldoCatalog() (cps, currencies []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[string]bool{}
	for _, n := range s.SaldoCPs {
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		cps = append(cps, n)
	}
	for _, c := range s.Counterparties {
		if c.Name == "" || seen[c.Name] {
			continue
		}
		seen[c.Name] = true
		cps = append(cps, c.Name)
	}
	if cps == nil {
		cps = []string{}
	}
	currencies = append([]string{}, s.SaldoCurrencies...)
	return
}

func (s *Store) AddSaldoCP(name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("пустое имя")
	}
	for _, n := range s.SaldoCPs {
		if strings.EqualFold(n, name) {
			return "", fmt.Errorf("уже есть: %s", n)
		}
	}
	s.SaldoCPs = append(s.SaldoCPs, name)
	found := false
	for _, c := range s.Counterparties {
		if strings.EqualFold(c.Name, name) {
			found = true
			break
		}
	}
	if !found {
		id := s.nextLocked()
		s.Counterparties = append(s.Counterparties, Counterparty{
			ID: id, UID: uid("cp", id), Name: name, CreatedAt: time.Now(),
		})
	}
	_ = s.saveLocked()
	return name, nil
}

func (s *Store) findSaldoCPLocked(query string) (idx int, name string, err error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return -1, "", fmt.Errorf("пустое имя")
	}
	idx = -1
	for i, n := range s.SaldoCPs {
		if strings.EqualFold(n, query) {
			if idx >= 0 {
				return -1, "", fmt.Errorf("несколько совпадений, уточните имя")
			}
			idx, name = i, n
		}
	}
	if idx < 0 {
		return -1, "", fmt.Errorf("контрагент %q не найден", query)
	}
	return idx, name, nil
}

func (s *Store) RenameSaldoCP(from, to string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx, old, err := s.findSaldoCPLocked(from)
	if err != nil {
		return "", "", err
	}
	to = strings.TrimSpace(to)
	if to == "" {
		return "", "", fmt.Errorf("пустое новое имя")
	}
	for i, n := range s.SaldoCPs {
		if i != idx && strings.EqualFold(n, to) {
			return "", "", fmt.Errorf("уже есть: %s", n)
		}
	}
	s.SaldoCPs[idx] = to
	for i := range s.SaldoOps {
		if s.SaldoOps[i].CP == old {
			s.SaldoOps[i].CP = to
		}
	}
	for i := range s.Counterparties {
		if s.Counterparties[i].Name == old {
			s.Counterparties[i].Name = to
		}
	}
	_ = s.saveLocked()
	return old, to, nil
}

func (s *Store) DeleteSaldoCP(query string) (string, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx, name, err := s.findSaldoCPLocked(query)
	if err != nil {
		return "", 0, err
	}
	for _, op := range s.SaldoOps {
		if strings.EqualFold(op.CP, name) {
			return "", 0, fmt.Errorf("у контрагента есть финансовая история; удаление запрещено")
		}
	}
	old := append([]string(nil), s.SaldoCPs...)
	s.SaldoCPs = append(s.SaldoCPs[:idx], s.SaldoCPs[idx+1:]...)
	if err := s.saveLocked(); err != nil {
		s.SaldoCPs = old
		return "", 0, err
	}
	return name, 0, nil
}

func (s *Store) knownSaldoLocked(cp, kind, cur string) error {
	okCP := false
	for _, n := range s.SaldoCPs {
		if n == cp {
			okCP = true
			break
		}
	}
	if !okCP {
		for _, c := range s.Counterparties {
			if c.Name == cp {
				okCP = true
				break
			}
		}
	}
	if !okCP {
		return fmt.Errorf("выберите контрагента из списка")
	}
	if kind != "swift" && kind != "nerez" {
		return fmt.Errorf("тип: SWIFT или нерезидентский рубль")
	}
	cur = strings.ToUpper(cur)
	okCur := false
	for _, c := range s.SaldoCurrencies {
		if c == cur {
			okCur = true
			break
		}
	}
	if !okCur {
		return fmt.Errorf("валюта не из списка")
	}
	return nil
}

func (s *Store) AddSaldo(cp, kind, cur, action, mgr string, amt float64, mgrID, requestID, counterpartyID int64) (SaldoOp, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp = strings.TrimSpace(cp)
	kind = strings.ToLower(strings.TrimSpace(kind))
	cur = strings.ToUpper(strings.TrimSpace(cur))
	var req Request
	if requestID != 0 {
		ok := false
		for _, r := range s.Requests {
			if r.ID == requestID {
				req = r
				ok = true
				break
			}
		}
		if !ok {
			return SaldoOp{}, fmt.Errorf("заявка не найдена")
		}
		if counterpartyID == 0 {
			counterpartyID = req.CounterpartyID
		}
	}
	if counterpartyID != 0 {
		ent, ok := s.counterpartyLocked(counterpartyID)
		if !ok {
			return SaldoOp{}, fmt.Errorf("контрагент не найден")
		}
		cp = ent.Name
	} else if cp != "" {
		for _, c := range s.Counterparties {
			if strings.EqualFold(c.Name, cp) {
				counterpartyID = c.ID
				cp = c.Name
				break
			}
		}
	}
	if err := s.knownSaldoLocked(cp, kind, cur); err != nil {
		return SaldoOp{}, err
	}
	emp, _ := s.employeeByTGLocked(mgrID)
	empID := emp.ID
	if req.EmployeeID != 0 {
		empID = req.EmployeeID
	}
	id := s.nextLocked()
	op := SaldoOp{
		ID: id, UID: uid("sld", id), CP: cp, Kind: kind, Currency: cur,
		Action: action, Amount: amt, Manager: mgr, ManagerID: mgrID, CreatedAt: time.Now(),
		RequestID: requestID, EmployeeID: empID, ClientID: req.ClientID,
		CounterpartyID: counterpartyID, CatalogManagerID: req.ManagerID,
	}
	return s.applySaldoDeltaLocked(op)
}

func (s *Store) requestLabelLocked(id int64) string {
	if id == 0 {
		return "без заявки"
	}
	for _, r := range s.Requests {
		if r.ID == id {
			if t := strings.TrimSpace(r.Title); t != "" {
				return t
			}
			if r.UID != "" {
				return r.UID
			}
			return fmt.Sprintf("заявка #%d", id)
		}
	}
	return fmt.Sprintf("заявка #%d", id)
}

type SaldoLine struct {
	CP        string  `json:"cp"`
	Request   string  `json:"request"`
	RequestID int64   `json:"request_id,omitempty"`
	Kind      string  `json:"kind"`
	KindKey   string  `json:"kind_key"`
	Currency  string  `json:"currency"`
	Amount    float64 `json:"amount"`
}

func (s *Store) SaldoReportLines() []SaldoLine {
	s.mu.Lock()
	defer s.mu.Unlock()
	tot := map[string]*big.Rat{}
	meta := map[string]SaldoLine{}
	for _, op := range s.SaldoOps {
		if cp, ok := s.counterpartyLocked(op.CounterpartyID); ok {
			op.CP = cp.Name
		}
		req := s.requestLabelLocked(op.RequestID)
		key := op.CP + "\x00" + req + "\x00" + op.Kind + "\x00" + op.Currency
		if tot[key] == nil {
			tot[key] = new(big.Rat)
		}
		amount := decimalFloat(op.Amount)
		if op.Action == "minus" {
			amount.Neg(amount)
		}
		tot[key].Add(tot[key], amount)
		meta[key] = SaldoLine{
			CP: op.CP, Request: req, RequestID: op.RequestID,
			Kind: KindLabel(op.Kind), KindKey: op.Kind, Currency: op.Currency,
		}
	}
	out := make([]SaldoLine, 0, len(tot))
	for k, amt := range tot {
		if amt.Sign() == 0 {
			continue
		}
		row := meta[k]
		row.Amount, _ = amt.Float64()
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if strings.ToLower(out[i].CP) != strings.ToLower(out[j].CP) {
			return strings.ToLower(out[i].CP) < strings.ToLower(out[j].CP)
		}
		if out[i].Request != out[j].Request {
			return out[i].Request < out[j].Request
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func (s *Store) SaldoReport() string {
	rows := s.SaldoReportLines()
	var b strings.Builder
	b.WriteString("Сальдо по контрагентам и заявкам\n")
	if len(rows) == 0 {
		b.WriteString("\nПока нет ненулевых остатков.")
		return b.String()
	}
	cur := ""
	for _, r := range rows {
		if r.CP != cur {
			cur = r.CP
			b.WriteString("\n")
			b.WriteString(r.CP)
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "  %s · %s · %s: %g\n", r.Request, r.Kind, r.Currency, r.Amount)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (s *Store) SaldoDetail(cp string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp = strings.TrimSpace(cp)
	var counterpartyID int64
	for _, c := range s.Counterparties {
		if strings.EqualFold(c.Name, cp) {
			counterpartyID = c.ID
			cp = c.Name
			break
		}
	}
	tot := map[string]float64{}
	var hist []SaldoOp
	for _, op := range s.SaldoOps {
		if counterpartyID != 0 && op.CounterpartyID != counterpartyID || counterpartyID == 0 && !strings.EqualFold(op.CP, cp) {
			continue
		}
		hist = append(hist, op)
		req := s.requestLabelLocked(op.RequestID)
		key := req + " · " + KindLabel(op.Kind) + "|" + op.Currency
		if op.Action == "minus" {
			tot[key] -= op.Amount
		} else {
			tot[key] += op.Amount
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\nОстатки:\n", cp)
	if len(tot) == 0 {
		b.WriteString("пусто\n")
	} else {
		keys := make([]string, 0, len(tot))
		for k := range tot {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString("  " + k + ": ")
			fmt.Fprintf(&b, "%g\n", tot[k])
		}
	}
	sort.Slice(hist, func(i, j int) bool { return hist[i].ID > hist[j].ID })
	b.WriteString("\nИстория:\n")
	if len(hist) == 0 {
		b.WriteString("нет операций")
	}
	for _, op := range hist {
		sign := "+"
		if op.Action == "minus" {
			sign = "−"
		}
		req := s.requestLabelLocked(op.RequestID)
		fmt.Fprintf(&b, "%s %s%g %s · %s · %s · %s\n", op.CreatedAt.Format("02.01 15:04"), sign, op.Amount, op.Currency, KindLabel(op.Kind), req, op.Manager)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (s *Store) SaldoFiles() (txt, balCSV, opCSV string) {
	s = s.saldoSnapshot()
	lines := s.SaldoReportLines()
	ops := s.SaldoCopy()
	var balRows [][]string
	for _, r := range lines {
		balRows = append(balRows, []string{r.CP, r.Request, r.Kind, r.Currency, fmt.Sprintf("%g", r.Amount)})
	}
	var opRows [][]string
	s.mu.Lock()
	for i := len(ops) - 1; i >= 0; i-- {
		op := ops[i]
		act := "плюс"
		if op.Action == "minus" {
			act = "минус"
		}
		opRows = append(opRows, []string{
			fmt.Sprintf("%d", op.ID), op.CreatedAt.Format("02.01.2006 15:04"),
			op.CP, s.requestLabelLocked(op.RequestID), KindLabel(op.Kind), op.Currency, act, fmt.Sprintf("%g", op.Amount), op.Manager,
		})
	}
	s.mu.Unlock()
	balCSV = csvString([]string{"Контрагент", "Заявка", "Тип сальдо", "Валюта", "Остаток"}, balRows)
	opCSV = csvString([]string{"ID", "Дата", "Контрагент", "Заявка", "Тип сальдо", "Валюта", "Действие", "Сумма", "Кто записал"}, opRows)
	txt = s.SaldoReport() + "\n\n" + "Операции: " + fmt.Sprintf("%d", len(ops))
	return
}

func csvString(head []string, rows [][]string) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write(head)
	_ = w.WriteAll(rows)
	w.Flush()
	return b.String()
}

func (s *Store) CloseSaldoLot(opID, managerID int64, managerName string) (SaldoLot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lots := rebuildSaldoLots(s.SaldoOps)
	var lot SaldoLot
	found := false
	for _, l := range lots {
		if l.OpID == opID {
			lot = l
			found = true
			break
		}
	}
	if !found || lot.Remaining <= 1e-8 {
		return SaldoLot{}, fmt.Errorf("постановка не найдена или уже закрыта")
	}
	if lot.ManagerID != managerID {
		return SaldoLot{}, fmt.Errorf("это не ваша постановка")
	}
	id := s.nextLocked()
	src := SaldoOp{}
	for _, o := range s.SaldoOps {
		if o.ID == lot.OpID {
			src = o
			break
		}
	}
	op := SaldoOp{
		ID: id, UID: uid("sld", id), CP: lot.CP, Kind: lot.Kind, Currency: lot.Currency,
		Action: "minus", Amount: lot.Remaining, Manager: managerName, ManagerID: managerID,
		CreatedAt: time.Now(), CloseLotOpID: lot.OpID,
		RequestID: src.RequestID, EmployeeID: src.EmployeeID, ClientID: src.ClientID,
		CounterpartyID: src.CounterpartyID, CatalogManagerID: src.CatalogManagerID,
	}
	if _, err := s.applySaldoDeltaLocked(op); err != nil {
		return SaldoLot{}, err
	}
	lot.Remaining = 0
	return lot, nil
}

func rebuildSaldoLots(ops []SaldoOp) []SaldoLot {
	var lots []SaldoLot
	for _, op := range ops {
		switch op.Action {
		case "minus":
			if op.CloseLotOpID != 0 {
				lots = consumeLotByOpID(lots, op.CloseLotOpID, op.Amount)
			} else {
				lots = consumeLotsFIFO(lots, op.CP, op.Kind, op.Currency, op.Amount, op.ManagerID)
			}
		default:
			lots = append(lots, SaldoLot{
				OpID: op.ID, UID: op.UID, CP: op.CP, Kind: op.Kind, Currency: op.Currency,
				Amount: op.Amount, Remaining: op.Amount, Manager: op.Manager, ManagerID: op.ManagerID,
				CreatedAt: op.CreatedAt, RequestID: op.RequestID,
			})
		}
	}
	out := lots[:0]
	for _, l := range lots {
		if l.Remaining > 1e-8 {
			out = append(out, l)
		}
	}
	return out
}

func consumeLotByOpID(lots []SaldoLot, opID int64, amount float64) []SaldoLot {
	for i := range lots {
		if lots[i].OpID != opID {
			continue
		}
		take := lots[i].Remaining
		if take > amount {
			take = amount
		}
		lots[i].Remaining -= take
		break
	}
	return lots
}

func consumeLotsFIFO(lots []SaldoLot, cp, kind, cur string, amount float64, managerID int64) []SaldoLot {
	left := amount
	cur = strings.ToUpper(cur)
	for i := range lots {
		if left <= 1e-8 {
			break
		}
		if managerID != 0 && lots[i].ManagerID != managerID {
			continue
		}
		if lots[i].CP != cp || lots[i].Kind != kind || !strings.EqualFold(lots[i].Currency, cur) {
			continue
		}
		take := lots[i].Remaining
		if take > left {
			take = left
		}
		lots[i].Remaining -= take
		left -= take
	}
	return lots
}

func (s *Store) OpenLotsByManager(managerID int64) []SaldoLot {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []SaldoLot
	for _, l := range rebuildSaldoLots(s.SaldoOps) {
		if l.ManagerID == managerID {
			l.RequestTitle = s.requestLabelLocked(l.RequestID)
			out = append(out, l)
		}
	}
	return out
}

func (s *Store) OpenLotsAll() []SaldoLot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return rebuildSaldoLots(s.SaldoOps)
}

func (s *Store) EveningSent(userID int64, day string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.LastEveningDigest == nil {
		return false
	}
	return s.LastEveningDigest[userID] == day
}

func (s *Store) MarkEveningSent(userID int64, day string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.LastEveningDigest == nil {
		s.LastEveningDigest = map[int64]string{}
	}
	s.LastEveningDigest[userID] = day
	_ = s.saveLocked()
}

func (s *Store) NotifyIDs(section, key string) []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []int64
	for _, u := range s.Users {
		if section == SecBalance {
			if !u.BalanceResponsible && !u.CanSection(SecBalance) {
				continue
			}
		} else if !u.CanSection(section) {
			continue
		}
		if !u.Notify.On(key) {
			continue
		}
		ids = append(ids, u.ID)
	}
	return ids
}

func (s *Store) Wants(userID int64, section, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.Users {
		if u.ID != userID {
			continue
		}
		if section != "" && !u.CanSection(section) {
			return false
		}
		return u.Notify.On(key)
	}
	return false
}

func (s *Store) SetNotify(userID int64, n Notify) (Notify, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Users {
		if s.Users[i].ID == userID {
			s.Users[i].Notify = NormalizeNotify(n)
			_ = s.saveLocked()
			return NormalizeNotify(s.Users[i].Notify), nil
		}
	}
	return nil, fmt.Errorf("пользователь не найден")
}

func (s *Store) HolidaySent(userID int64, day string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.LastHolidayDay == nil {
		return false
	}
	return s.LastHolidayDay[userID] == day
}

func (s *Store) MarkHolidaySent(userID int64, day string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.LastHolidayDay == nil {
		s.LastHolidayDay = map[int64]string{}
	}
	s.LastHolidayDay[userID] = day
	_ = s.saveLocked()
}

func (s *Store) SaldoCopy() []SaldoOp {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]SaldoOp(nil), s.SaldoOps...)
	for i := range out {
		if c, ok := s.counterpartyLocked(out[i].CounterpartyID); ok {
			out[i].CP = c.Name
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func (s *Store) SaldoTotals() map[string]float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	tot := map[string]float64{}
	for _, v := range s.SaldoBalances {
		name := v.CP
		if c, ok := s.counterpartyLocked(v.CounterpartyID); ok {
			name = c.Name
		}
		n, _ := strconv.ParseFloat(v.Amount, 64)
		tot[name+"|"+v.Kind+"|"+v.Currency] += n
	}
	return tot
}

func (s *Store) CompaniesCopy() []Company {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Company(nil), s.Companies...)
}

func (s *Store) AccountsCopy() []Account {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Account(nil), s.Accounts...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func (s *Store) UpsertAccount(companyID int64, bank, cur string, amount float64, by string) Account {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur = strings.ToUpper(cur)
	for i := range s.Accounts {
		a := &s.Accounts[i]
		if a.CompanyID == companyID && strings.EqualFold(a.Bank, bank) && a.Currency == cur {
			a.Amount = amount
			a.UpdatedAt = time.Now()
			a.UpdatedBy = by
			_ = s.saveLocked()
			return *a
		}
	}
	id := s.nextLocked()
	a := Account{
		ID: id, UID: uid("acc", id), CompanyID: companyID, Bank: bank, Currency: cur,
		Amount: amount, UpdatedAt: time.Now(), UpdatedBy: by,
	}
	s.Accounts = append(s.Accounts, a)
	_ = s.saveLocked()
	return a
}

func (s *Store) MarkBalanceSlot(slot string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	day := time.Now().Format("2006-01-02")
	if slot == "morning" {
		s.MorningOK = day
	} else {
		s.EveningOK = day
	}
	_ = s.saveLocked()
}

func (s *Store) BalanceSlotOK(slot string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	day := now.Format("2006-01-02")
	if slot == "morning" {
		return s.MorningOK == day
	}
	return s.EveningOK == day
}

func (s *Store) AddCompliance(subject, text, name string, by int64, reqUID string, requestID int64) Compliance {
	s.mu.Lock()
	defer s.mu.Unlock()
	var req Request
	if requestID != 0 {
		for _, r := range s.Requests {
			if r.ID == requestID {
				req = r
				break
			}
		}
		if req.ID != 0 {
			reqUID = req.UID
			if strings.TrimSpace(subject) == "" {
				if cp, ok := s.counterpartyLocked(req.CounterpartyID); ok {
					subject = cp.Name
				} else {
					subject = req.Title
				}
			}
		}
	}
	subject = strings.TrimSpace(subject)
	text = strings.TrimSpace(text)
	if subject == "" {
		subject = "Проверка"
	}
	id := s.nextLocked()
	c := Compliance{
		ID: id, UID: uid("cmpc", id), Subject: subject, Text: text, Status: "open",
		CreatedBy: by, CreatedName: name, CreatedAt: time.Now(), DueAt: time.Now().Add(48 * time.Hour),
		RequestUID: reqUID, RequestID: req.ID,
		EmployeeID: req.EmployeeID, ManagerID: req.ManagerID, ClientID: req.ClientID, CounterpartyID: req.CounterpartyID,
	}
	s.Compliances = append(s.Compliances, c)
	_ = s.saveLocked()
	return c
}

func (s *Store) SetComplianceText(id int64, text string) (Compliance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Compliances {
		if s.Compliances[i].ID != id {
			continue
		}
		s.Compliances[i].Text = strings.TrimSpace(text)
		_ = s.saveLocked()
		return s.Compliances[i], nil
	}
	return Compliance{}, fmt.Errorf("проверка не найдена")
}

func (s *Store) SetComplianceStatus(id int64, status string) (Compliance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status = strings.TrimSpace(status)
	if status != "done" && status != "open" {
		return Compliance{}, fmt.Errorf("статус: open или done")
	}
	for i := range s.Compliances {
		if s.Compliances[i].ID != id {
			continue
		}
		s.Compliances[i].Status = status
		_ = s.saveLocked()
		return s.Compliances[i], nil
	}
	return Compliance{}, fmt.Errorf("проверка не найдена")
}

func (s *Store) AnswerCompliance(id int64, key, answer string) (Compliance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Compliances {
		if s.Compliances[i].ID != id {
			continue
		}
		done := 0
		for j := range s.Compliances[i].Questions {
			if s.Compliances[i].Questions[j].Key == key {
				s.Compliances[i].Questions[j].Answer = answer
			}
			if strings.TrimSpace(s.Compliances[i].Questions[j].Answer) != "" {
				done++
			}
		}
		if done == len(s.Compliances[i].Questions) {
			s.Compliances[i].Status = "done"
		}
		_ = s.saveLocked()
		return s.Compliances[i], nil
	}
	return Compliance{}, fmt.Errorf("проверка не найдена")
}

func (s *Store) CompliancesCopy() []Compliance {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Compliance(nil), s.Compliances...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func (s *Store) AddDispute(section, ref, text, byName string, by int64) (Dispute, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	text = strings.TrimSpace(text)
	if text == "" {
		return Dispute{}, fmt.Errorf("пустой текст")
	}
	if byName == "" {
		byName = fmt.Sprintf("%d", by)
	}
	var targetID int64
	var targetName string
	if section == "approvals" {
		if a, ok := s.ApprovalByUIDLocked(ref); ok {
			targetID = a.ManagerID
			targetName = a.ManagerName
		}
	}
	id := s.nextLocked()
	now := time.Now()
	msgID := s.nextLocked()
	d := Dispute{
		ID: id, UID: uid("dsp", id), Section: section, RefUID: ref, Text: text, Status: "open",
		CreatedBy: by, CreatedName: byName, TargetID: targetID, TargetName: targetName,
		CreatedAt: now, LastAt: now,
		Messages: []DisputeMsg{{ID: msgID, By: by, ByName: byName, Text: text, At: now}},
	}
	s.Disputes = append(s.Disputes, d)
	_ = s.saveLocked()
	return d, nil
}

func (s *Store) ReplyDispute(id int64, by int64, byName, text string, admin bool) (Dispute, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	text = strings.TrimSpace(text)
	if text == "" {
		return Dispute{}, fmt.Errorf("пустой текст")
	}
	if byName == "" {
		byName = fmt.Sprintf("%d", by)
	}
	for i := range s.Disputes {
		if s.Disputes[i].ID != id {
			continue
		}
		d := s.Disputes[i]
		if !disputeInvolves(d, by, admin) {
			return Dispute{}, fmt.Errorf("нет доступа к спору")
		}
		if d.Status != "open" {
			return Dispute{}, fmt.Errorf("спор закрыт")
		}
		msgID := s.nextLocked()
		now := time.Now()
		s.Disputes[i].Messages = append(s.Disputes[i].Messages, DisputeMsg{
			ID: msgID, By: by, ByName: byName, Text: text, At: now,
		})
		s.Disputes[i].LastAt = now
		s.Disputes[i].Text = text
		_ = s.saveLocked()
		return s.Disputes[i], nil
	}
	return Dispute{}, fmt.Errorf("спор не найден")
}

func (s *Store) SetDisputeStatus(id int64, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Disputes {
		if s.Disputes[i].ID == id {
			s.Disputes[i].Status = status
			s.Disputes[i].LastAt = time.Now()
			return s.saveLocked()
		}
	}
	return fmt.Errorf("спор не найден")
}

func (s *Store) DisputesCopy() []Dispute {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Dispute(nil), s.Disputes...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func (s *Store) DisputesFor(userID int64, admin bool) []Dispute {
	s.mu.Lock()
	defer s.mu.Unlock()
	hidden := s.hiddenLocked(userID)
	out := []Dispute{}
	for _, d := range s.Disputes {
		if !disputeInvolves(d, userID, admin) {
			continue
		}
		if hidden[fmt.Sprintf("dsp:%d", d.ID)] {
			continue
		}
		d.Messages = append([]DisputeMsg(nil), d.Messages...)
		if len(d.Messages) == 0 && d.Text != "" {
			d.Messages = []DisputeMsg{{By: d.CreatedBy, ByName: d.CreatedName, Text: d.Text, At: d.CreatedAt}}
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		ti, tj := out[i].LastAt, out[j].LastAt
		if ti.IsZero() {
			ti = out[i].CreatedAt
		}
		if tj.IsZero() {
			tj = out[j].CreatedAt
		}
		return ti.After(tj)
	})
	return out
}

func disputeInvolves(d Dispute, userID int64, admin bool) bool {
	if admin {
		return true
	}
	if d.CreatedBy == userID || d.TargetID == userID {
		return true
	}
	for _, m := range d.Messages {
		if m.By == userID {
			return true
		}
	}
	return false
}

func (s *Store) hiddenLocked(userID int64) map[string]bool {
	for _, u := range s.Users {
		if u.ID == userID {
			out := map[string]bool{}
			for k, v := range u.Hidden {
				if v {
					out[k] = true
				}
			}
			return out
		}
	}
	return map[string]bool{}
}

func (s *Store) MarkSeen(userID int64, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(key) == "" {
		return
	}
	for i := range s.Users {
		if s.Users[i].ID != userID {
			continue
		}
		if s.Users[i].Seen == nil {
			s.Users[i].Seen = map[string]time.Time{}
		}
		s.Users[i].Seen[key] = time.Now()
		if s.Users[i].Inbox == nil {
			s.Users[i].Inbox = map[string]int{}
		}
		s.Users[i].Inbox[key] = 0
		if key == "approvals" {
			s.Users[i].Inbox["disputes"] = 0
		}
		_ = s.saveLocked()
		return
	}
}

func (s *Store) userSeesSectionLocked(u User, section string) bool {
	if u.Role == RoleOwner || u.Role == RoleAdmin {
		return true
	}
	if section == "users" || section == "summary" || section == "activity" {
		return false
	}
	if u.CanSection(section) {
		return true
	}
	e, ok := s.employeeByTGLocked(u.ID)
	if !ok {
		return false
	}
	return s.unionPositionsLocked(e.PositionIDs).Has(section)
}

func (s *Store) bumpUserInboxLocked(i int, section string) {
	if i < 0 || i >= len(s.Users) || section == "" {
		return
	}
	if s.Users[i].Inbox == nil {
		s.Users[i].Inbox = map[string]int{}
	}
	s.Users[i].Inbox[section]++
	if section != "activity" && section != "users" && (s.Users[i].Role == RoleOwner || s.Users[i].Role == RoleAdmin) {
		s.Users[i].Inbox["activity"]++
	}
}

func (s *Store) BumpInbox(section string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Users {
		if !s.userSeesSectionLocked(s.Users[i], section) {
			continue
		}
		s.bumpUserInboxLocked(i, section)
	}
	_ = s.saveLocked()
}

func (s *Store) BumpInboxIDs(section string, ids []int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	want := map[int64]bool{}
	for _, id := range ids {
		if id != 0 {
			want[id] = true
		}
	}
	if len(want) == 0 {
		return
	}
	for i := range s.Users {
		if !want[s.Users[i].ID] {
			continue
		}
		s.bumpUserInboxLocked(i, section)
	}
	_ = s.saveLocked()
}

func (s *Store) HideClosed(userID int64, kind string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for i := range s.Users {
		if s.Users[i].ID != userID {
			continue
		}
		if s.Users[i].Hidden == nil {
			s.Users[i].Hidden = map[string]bool{}
		}
		if kind == "disputes" || kind == "dsp" {
			for _, d := range s.Disputes {
				if d.Status == "closed" {
					k := fmt.Sprintf("dsp:%d", d.ID)
					if !s.Users[i].Hidden[k] {
						s.Users[i].Hidden[k] = true
						n++
					}
				}
			}
		}
		if kind == "approvals" || kind == "apr" {
			for _, a := range s.Approvals {
				if a.Status != "pending" {
					k := fmt.Sprintf("apr:%d", a.ID)
					if !s.Users[i].Hidden[k] {
						s.Users[i].Hidden[k] = true
						n++
					}
				}
			}
		}
		_ = s.saveLocked()
		return n
	}
	return 0
}

func (s *Store) HideOne(userID int64, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("пусто")
	}
	for i := range s.Users {
		if s.Users[i].ID != userID {
			continue
		}
		ok := false
		if strings.HasPrefix(key, "apr:") {
			id, _ := strconv.ParseInt(strings.TrimPrefix(key, "apr:"), 10, 64)
			for _, a := range s.Approvals {
				if a.ID == id && a.Status != "pending" {
					ok = true
					break
				}
			}
		}
		if strings.HasPrefix(key, "dsp:") {
			id, _ := strconv.ParseInt(strings.TrimPrefix(key, "dsp:"), 10, 64)
			for _, d := range s.Disputes {
				if d.ID == id && d.Status == "closed" {
					ok = true
					break
				}
			}
		}
		if !ok {
			return fmt.Errorf("скрыть можно только закрытое")
		}
		if s.Users[i].Hidden == nil {
			s.Users[i].Hidden = map[string]bool{}
		}
		s.Users[i].Hidden[key] = true
		_ = s.saveLocked()
		return nil
	}
	return fmt.Errorf("пользователь не найден")
}

func (s *Store) Unread(userID int64, admin bool) map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = admin
	out := map[string]int{
		"requests": 0, "tasks": 0, "approvals": 0, "disputes": 0,
		"saldo": 0, "payments": 0, "compliance": 0, "directory": 0,
		"appeal_lawyer": 0, "appeal_docs": 0, "users": 0, "activity": 0,
		"balance": 0, "rates": 0, "holidays": 0, "total": 0,
	}
	for _, u := range s.Users {
		if u.ID != userID {
			continue
		}
		for k, n := range u.Inbox {
			if n > 0 {
				out[k] = n
			}
		}
		break
	}
	if out["total"] == 0 {
		out["total"] = out["approvals"] + out["disputes"]
	}
	return out
}

func (s *Store) DisputePeers(d Dispute) []int64 {
	seen := map[int64]bool{}
	var out []int64
	add := func(id int64) {
		if id == 0 || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	add(d.CreatedBy)
	add(d.TargetID)
	for _, m := range d.Messages {
		add(m.By)
	}
	return out
}

func (s *Store) Summary() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	openReq, pendApr, openCmp, openDsp := 0, 0, 0, 0
	for _, r := range s.Requests {
		if r.Status == "open" || r.Status == "in_progress" {
			openReq++
		}
	}
	for _, a := range s.Approvals {
		if a.Status == "pending" {
			pendApr++
		}
	}
	for _, c := range s.Compliances {
		if c.Status != "done" {
			openCmp++
		}
	}
	for _, d := range s.Disputes {
		if d.Status == "open" {
			openDsp++
		}
	}
	return map[string]any{
		"users":             len(s.Users),
		"requests_open":     openReq,
		"approvals_pending": pendApr,
		"saldo_ops":         len(s.SaldoOps),
		"accounts":          len(s.Accounts),
		"compliance_open":   openCmp,
		"disputes_open":     openDsp,
	}
}

type HomeAct struct {
	Kind  string    `json:"kind"`
	Title string    `json:"title"`
	Text  string    `json:"text"`
	At    time.Time `json:"at"`
	Go    string    `json:"go"`
}

func (s *Store) HomeDash() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	total, open, progress := 0, 0, 0
	for _, r := range s.Requests {
		if r.Status == "deleted" {
			continue
		}
		total++
		switch r.Status {
		case "in_progress":
			progress++
		case "open":
			open++
		}
	}
	pendApr, pendPay := 0, 0
	for _, a := range s.Approvals {
		if a.Status == "pending" {
			pendApr++
		}
	}
	for _, p := range s.Payments {
		if p.Status != "sent" {
			pendPay++
		}
	}
	tasksOpen, tasksNew, tasksProgress, tasksDocs := s.TasksOpenCountLocked()
	lawyerOpen, docsOpen := 0, 0
	for _, a := range s.Appeals {
		if a.Status == "cancelled" || a.Status == "closed" {
			continue
		}
		if a.Kind == "lawyer" {
			lawyerOpen++
		} else {
			docsOpen++
		}
	}
	type act struct {
		at time.Time
		HomeAct
	}
	var acts []act
	for _, r := range s.Requests {
		if r.Status == "deleted" {
			continue
		}
		title := r.Title
		if title == "" {
			title = r.UID
		}
		acts = append(acts, act{r.CreatedAt, HomeAct{
			Kind: "request", Title: r.CreatedName, Text: "заявка «" + title + "»", At: r.CreatedAt, Go: "requests",
		}})
	}
	for _, p := range s.Payments {
		st := "оплата в отправку"
		if p.Status == "sent" {
			st = "оплата отправлена"
		}
		acts = append(acts, act{p.CreatedAt, HomeAct{
			Kind: "pay", Title: p.CreatedName, Text: st, At: p.CreatedAt, Go: "payments",
		}})
	}
	for _, a := range s.Approvals {
		acts = append(acts, act{a.CreatedAt, HomeAct{
			Kind: "apr", Title: a.ManagerName, Text: "согласование: " + a.Preview, At: a.CreatedAt, Go: "approvals",
		}})
	}
	for _, t := range s.Tasks {
		st := "задача: " + t.Title
		acts = append(acts, act{t.CreatedAt, HomeAct{
			Kind: "task", Title: t.CreatedName, Text: st, At: t.CreatedAt, Go: "tasks",
		}})
	}
	for _, o := range s.SaldoOps {
		who := o.Manager
		if who == "" {
			who = "Сальдо"
		}
		txt := strings.TrimSpace(o.Action + " " + o.Currency)
		if o.CP != "" {
			txt += " · " + o.CP
		}
		acts = append(acts, act{o.CreatedAt, HomeAct{
			Kind: "saldo", Title: who, Text: txt, At: o.CreatedAt, Go: "saldo",
		}})
	}
	sort.Slice(acts, func(i, j int) bool { return acts[i].at.After(acts[j].at) })
	if len(acts) > 8 {
		acts = acts[:8]
	}
	out := make([]HomeAct, 0, len(acts))
	for _, a := range acts {
		a.HomeAct.Text = clipRunes(a.Text, 90)
		out = append(out, a.HomeAct)
	}
	return map[string]any{
		"requests_total":    total,
		"requests_open":     open,
		"requests_progress": progress,
		"approvals_pending": pendApr,
		"payments_pending":  pendPay,
		"tasks_open":        tasksOpen,
		"tasks_new":         tasksNew,
		"tasks_progress":    tasksProgress,
		"tasks_docs":        tasksDocs,
		"activity":          out,
		"dir_warn":          s.orphanPositionWarnLocked(),
		"lawyer_open":       lawyerOpen,
		"docs_open":         docsOpen,
	}
}

func clipRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}
