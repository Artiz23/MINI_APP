package appdb

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Employee struct {
	ID          int64     `json:"id"`
	UID         string    `json:"uid"`
	Name        string    `json:"name"`
	Number      string    `json:"number,omitempty"`
	Title       string    `json:"title,omitempty"`
	TelegramID  int64     `json:"telegram_id,omitempty"`
	PositionIDs []int64   `json:"position_ids,omitempty"`
	ManagerIDs  []int64   `json:"manager_ids,omitempty"`
	ClientIDs   []int64   `json:"client_ids,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Manager struct {
	ID          int64     `json:"id"`
	UID         string    `json:"uid"`
	Name        string    `json:"name"`
	Abbrev      string    `json:"abbrev,omitempty"`
	EmployeeIDs []int64   `json:"employee_ids,omitempty"`
	CreatedBy   int64     `json:"created_by,omitempty"`
	CreatedName string    `json:"created_name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Client struct {
	ID          int64     `json:"id"`
	UID         string    `json:"uid"`
	Name        string    `json:"name"`
	WorkID      string    `json:"work_id,omitempty"`
	EmployeeIDs []int64   `json:"employee_ids,omitempty"`
	CreatedBy   int64     `json:"created_by,omitempty"`
	CreatedName string    `json:"created_name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Counterparty struct {
	ID          int64     `json:"id"`
	UID         string    `json:"uid"`
	Name        string    `json:"name"`
	WorkID      string    `json:"work_id,omitempty"`
	CreatedBy   int64     `json:"created_by,omitempty"`
	CreatedName string    `json:"created_name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type RequestView struct {
	DocumentID    int64  `json:"document_id,omitempty"`
	DocumentBy    int64  `json:"document_by,omitempty"`
	DocumentPlace string `json:"document_place,omitempty"`
	Request
	EmployeeName       string `json:"employee_name,omitempty"`
	ManagerName        string `json:"manager_name,omitempty"`
	ManagerAbbrev      string `json:"manager_abbrev,omitempty"`
	ClientName         string `json:"client_name,omitempty"`
	ClientWorkID       string `json:"client_work_id,omitempty"`
	CounterpartyName   string `json:"counterparty_name,omitempty"`
	CounterpartyWorkID string `json:"counterparty_work_id,omitempty"`
	PaymentStatus      string `json:"payment_status,omitempty"`
	FilesCount         int    `json:"files_count"`
	MessagesCount      int    `json:"messages_count"`
}

type CatalogIn struct {
	Kind             string  `json:"kind"`
	ID               int64   `json:"id,omitempty"`
	Name             string  `json:"name"`
	Title            string  `json:"title,omitempty"`
	TelegramID       int64   `json:"telegram_id,omitempty"`
	Abbrev           string  `json:"abbrev,omitempty"`
	WorkID           string  `json:"work_id,omitempty"`
	Number           string  `json:"number,omitempty"`
	LinkManagerID    int64   `json:"link_manager_id,omitempty"`
	UnlinkManagerID  int64   `json:"unlink_manager_id,omitempty"`
	LinkClientID     int64   `json:"link_client_id,omitempty"`
	UnlinkClientID   int64   `json:"unlink_client_id,omitempty"`
	LinkEmployeeID   int64   `json:"link_employee_id,omitempty"`
	UnlinkEmployeeID int64   `json:"unlink_employee_id,omitempty"`
	LinkPositionID   int64   `json:"link_position_id,omitempty"`
	UnlinkPositionID int64   `json:"unlink_position_id,omitempty"`
	PositionIDs      []int64 `json:"position_ids,omitempty"`
	Access           Access  `json:"access,omitempty"`
	RevokeAccess     bool    `json:"revoke_access,omitempty"`
	CreatedBy        int64   `json:"-"`
	CreatedName      string  `json:"-"`
}

func (s *Store) migrateCatalogLocked() bool {
	changed := false
	if s.Employees == nil {
		s.Employees = []Employee{}
	}
	if s.Managers == nil {
		s.Managers = []Manager{}
	}
	if s.Clients == nil {
		s.Clients = []Client{}
	}
	if s.Counterparties == nil {
		s.Counterparties = []Counterparty{}
	}
	if s.seedPositionsLocked() {
		changed = true
	}
	if s.stripOrphanPositionsLocked() {
		changed = true
	}
	if len(s.Counterparties) == 0 && len(s.SaldoCPs) > 0 {
		for _, name := range s.SaldoCPs {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			id := s.nextLocked()
			s.Counterparties = append(s.Counterparties, Counterparty{
				ID: id, UID: uid("cp", id), Name: name, CreatedAt: time.Now(),
			})
			changed = true
		}
	}
	if s.linkAllUsersToEmployeesLocked() {
		changed = true
	}
	return changed
}

func normPersonName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func (s *Store) linkAllUsersToEmployeesLocked() bool {
	changed := false
	for _, u := range s.Users {
		if u.ID == 0 {
			continue
		}
		if s.linkUserToEmployeeLocked(u.ID, u.Name) {
			changed = true
		}
	}
	return changed
}

func (s *Store) linkUserToEmployeeLocked(tgID int64, name string) bool {
	if tgID == 0 {
		return false
	}
	if _, ok := s.employeeByTGLocked(tgID); ok {
		return false
	}
	name = normPersonName(name)
	if name == "" {
		return false
	}
	var byName []int
	for i, e := range s.Employees {
		if e.TelegramID != 0 {
			continue
		}
		if normPersonName(e.Name) == name {
			byName = append(byName, i)
		}
	}
	if len(byName) != 1 {
		return false
	}
	s.Employees[byName[0]].TelegramID = tgID
	return true
}

func (s *Store) userIDByNameLocked(name string) int64 {
	name = normPersonName(name)
	if name == "" {
		return 0
	}
	var id int64
	n := 0
	for _, u := range s.Users {
		if normPersonName(u.Name) == name {
			if _, ok := s.employeeByTGLocked(u.ID); ok {
				continue
			}
			id = u.ID
			n++
		}
	}
	if n == 1 {
		return id
	}
	return 0
}

func (s *Store) employeeForUserLocked(tgID int64, name string) (Employee, bool) {
	if e, ok := s.employeeByTGLocked(tgID); ok {
		return e, true
	}
	if s.linkUserToEmployeeLocked(tgID, name) {
		return s.employeeByTGLocked(tgID)
	}
	return Employee{}, false
}

func (s *Store) actorEmployeeLocked(by int64, name string, staff bool) (Employee, bool) {
	if e, ok := s.employeeByTGLocked(by); ok {
		return e, true
	}
	if !staff {
		if u, ok := s.userLocked(by); ok && u.IsAdmin() {
			staff = true
		}
	}
	if staff {
		if strings.TrimSpace(name) == "" {
			name = "админ"
		}
		return Employee{Name: name, Number: "00"}, true
	}
	if s.linkUserToEmployeeLocked(by, name) {
		return s.employeeByTGLocked(by)
	}
	return Employee{}, false
}

func (s *Store) EmployeeByTelegram(tgID int64) (Employee, bool) {
	if tgID == 0 {
		return Employee{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.Employees {
		if e.TelegramID == tgID {
			return e, true
		}
	}
	return Employee{}, false
}

func (s *Store) EmployeesCopy() []Employee {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Employee(nil), s.Employees...)
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

func (s *Store) ManagersCopy() []Manager {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Manager(nil), s.Managers...)
	for i := range out {
		out[i].EmployeeIDs = s.employeesOfManagerLocked(out[i].ID)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

func (s *Store) ClientsCopy() []Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Client(nil), s.Clients...)
	for i := range out {
		out[i].EmployeeIDs = s.employeesOfClientLocked(out[i].ID)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

func (s *Store) CounterpartiesCopy() []Counterparty {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Counterparty(nil), s.Counterparties...)
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

func (s *Store) CatalogGet(kind string) (any, error) {
	switch strings.TrimSpace(kind) {
	case "employees":
		return s.EmployeesCopy(), nil
	case "managers":
		return s.ManagersCopy(), nil
	case "clients":
		return s.ClientsCopy(), nil
	case "counterparties":
		return s.CounterpartiesCopy(), nil
	case "positions":
		return s.PositionsCopy(), nil
	default:
		return nil, fmt.Errorf("неизвестный справочник")
	}
}

func (s *Store) CatalogAdd(in CatalogIn) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if in.ID != 0 {
		return s.catalogUpdateLocked(in)
	}
	name := strings.TrimSpace(in.Name)
	if name == "" && strings.TrimSpace(in.Kind) != "employees" {
		return nil, fmt.Errorf("нужно имя")
	}
	now := time.Now()
	switch strings.TrimSpace(in.Kind) {
	case "employees":
		u, err := s.requirePoolUserLocked(in.TelegramID, 0)
		if err != nil {
			return nil, err
		}
		if name == "" {
			name = strings.TrimSpace(u.Name)
		}
		if name == "" {
			return nil, fmt.Errorf("нужно имя")
		}
		name, err = s.uniqueKindNameLocked("employees", name, 0)
		if err != nil {
			return nil, err
		}
		num, err := s.uniqueEmpNumberLocked(0, in.Number)
		if err != nil {
			return nil, err
		}
		id := s.nextLocked()
		e := Employee{ID: id, UID: uid("emp", id), Name: name, Number: num, Title: strings.TrimSpace(in.Title), TelegramID: u.ID, PositionIDs: s.cleanPositionIDsLocked(in.PositionIDs), CreatedAt: now}
		s.Employees = append(s.Employees, e)
		_ = s.saveLocked()
		return e, nil
	case "managers":
		name, err := s.uniqueKindNameLocked("managers", name, 0)
		if err != nil {
			return nil, err
		}
		ab, err := s.uniqueMgrAbbrevLocked(0, in.Abbrev)
		if err != nil {
			return nil, err
		}
		id := s.nextLocked()
		m := Manager{ID: id, UID: uid("mgr", id), Name: name, Abbrev: ab, CreatedBy: in.CreatedBy, CreatedName: strings.TrimSpace(in.CreatedName), CreatedAt: now}
		s.Managers = append(s.Managers, m)
		_ = s.saveLocked()
		return m, nil
	case "clients":
		name, err := s.uniqueKindNameLocked("clients", name, 0)
		if err != nil {
			return nil, err
		}
		id := s.nextLocked()
		c := Client{ID: id, UID: uid("cl", id), Name: name, WorkID: strings.TrimSpace(in.WorkID), CreatedBy: in.CreatedBy, CreatedName: strings.TrimSpace(in.CreatedName), CreatedAt: now}
		s.Clients = append(s.Clients, c)
		_ = s.saveLocked()
		return c, nil
	case "positions":
		name, err := s.uniqueKindNameLocked("positions", name, 0)
		if err != nil {
			return nil, err
		}
		id := s.nextLocked()
		p := Position{ID: id, UID: uid("pos", id), Name: name, Access: NormalizeAccess(in.Access), CreatedAt: now}
		s.Positions = append(s.Positions, p)
		_ = s.saveLocked()
		return p, nil
	case "counterparties":
		name, err := s.uniqueKindNameLocked("counterparties", name, 0)
		if err != nil {
			return nil, err
		}
		id := s.nextLocked()
		c := Counterparty{ID: id, UID: uid("cp", id), Name: name, WorkID: strings.TrimSpace(in.WorkID), CreatedBy: in.CreatedBy, CreatedName: strings.TrimSpace(in.CreatedName), CreatedAt: now}
		s.Counterparties = append(s.Counterparties, c)
		found := false
		for _, n := range s.SaldoCPs {
			if strings.EqualFold(n, name) {
				found = true
				break
			}
		}
		if !found {
			s.SaldoCPs = append(s.SaldoCPs, name)
		}
		_ = s.saveLocked()
		return c, nil
	default:
		return nil, fmt.Errorf("неизвестный справочник")
	}
}

func (s *Store) requirePoolUserLocked(tgID, exceptEmpID int64) (User, error) {
	if tgID == 0 {
		return User{}, fmt.Errorf("выберите человека из доступов")
	}
	u, ok := s.userLocked(tgID)
	if !ok {
		return User{}, fmt.Errorf("сначала выдайте доступ в Доступах")
	}
	for _, e := range s.Employees {
		if e.ID != exceptEmpID && e.TelegramID == tgID {
			return User{}, fmt.Errorf("этот человек уже сотрудник")
		}
	}
	return u, nil
}

func (s *Store) uniqueEmpNumberLocked(exceptID int64, raw string) (string, error) {
	num := empDigits(raw)
	if num == "" {
		return "", fmt.Errorf("нужен номер сотрудника, 1–2 цифры")
	}
	for _, e := range s.Employees {
		if e.ID != exceptID && empDigits(e.Number) == num {
			return "", fmt.Errorf("номер сотрудника %s уже занят", num)
		}
	}
	return num, nil
}

func (s *Store) uniqueMgrAbbrevLocked(exceptID int64, raw string) (string, error) {
	ab := letters2(raw)
	if len([]rune(ab)) != 2 {
		return "", fmt.Errorf("аббревиатура — две буквы, например ЖЖ")
	}
	for _, m := range s.Managers {
		if m.ID != exceptID && letters2(m.Abbrev) == ab {
			return "", fmt.Errorf("аббревиатура %s уже занята", ab)
		}
	}
	return ab, nil
}

func uniqIDs(ids []int64) []int64 {
	seen := map[int64]bool{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func dropID(ids []int64, id int64) []int64 {
	out := ids[:0]
	for _, x := range ids {
		if x != id {
			out = append(out, x)
		}
	}
	return append([]int64(nil), out...)
}

func (s *Store) employeesOfManagerLocked(mgrID int64) []int64 {
	out := []int64{}
	for _, e := range s.Employees {
		for _, id := range e.ManagerIDs {
			if id == mgrID {
				out = append(out, e.ID)
				break
			}
		}
	}
	return out
}

func (s *Store) employeesOfClientLocked(clientID int64) []int64 {
	out := []int64{}
	for _, e := range s.Employees {
		for _, id := range e.ClientIDs {
			if id == clientID {
				out = append(out, e.ID)
				break
			}
		}
	}
	return out
}

func (s *Store) setEmpMgrLocked(empID, mgrID int64, on bool) error {
	if on {
		if _, ok := s.managerLocked(mgrID); !ok {
			return fmt.Errorf("менеджер не найден")
		}
	}
	for i := range s.Employees {
		if s.Employees[i].ID != empID {
			continue
		}
		if on {
			s.Employees[i].ManagerIDs = uniqIDs(append(s.Employees[i].ManagerIDs, mgrID))
		} else {
			s.Employees[i].ManagerIDs = dropID(s.Employees[i].ManagerIDs, mgrID)
		}
		return nil
	}
	return fmt.Errorf("сотрудник не найден")
}

func (s *Store) setEmpClientLocked(empID, clientID int64, on bool) error {
	if on {
		if _, ok := s.clientLocked(clientID); !ok {
			return fmt.Errorf("клиент не найден")
		}
	}
	for i := range s.Employees {
		if s.Employees[i].ID != empID {
			continue
		}
		if on {
			s.Employees[i].ClientIDs = uniqIDs(append(s.Employees[i].ClientIDs, clientID))
		} else {
			s.Employees[i].ClientIDs = dropID(s.Employees[i].ClientIDs, clientID)
		}
		return nil
	}
	return fmt.Errorf("сотрудник не найден")
}

func (s *Store) catalogUpdateLocked(in CatalogIn) (any, error) {
	kind := strings.TrimSpace(in.Kind)
	if kind == "managers" && (in.LinkEmployeeID != 0 || in.UnlinkEmployeeID != 0) {
		if _, ok := s.managerLocked(in.ID); !ok {
			return nil, fmt.Errorf("менеджер не найден")
		}
		if in.LinkEmployeeID != 0 {
			if err := s.setEmpMgrLocked(in.LinkEmployeeID, in.ID, true); err != nil {
				return nil, err
			}
		}
		if in.UnlinkEmployeeID != 0 {
			if err := s.setEmpMgrLocked(in.UnlinkEmployeeID, in.ID, false); err != nil {
				return nil, err
			}
		}
		_ = s.saveLocked()
		m, _ := s.managerLocked(in.ID)
		m.EmployeeIDs = s.employeesOfManagerLocked(in.ID)
		return m, nil
	}
	if kind == "clients" && (in.LinkEmployeeID != 0 || in.UnlinkEmployeeID != 0) {
		if _, ok := s.clientLocked(in.ID); !ok {
			return nil, fmt.Errorf("клиент не найден")
		}
		if in.LinkEmployeeID != 0 {
			if err := s.setEmpClientLocked(in.LinkEmployeeID, in.ID, true); err != nil {
				return nil, err
			}
		}
		if in.UnlinkEmployeeID != 0 {
			if err := s.setEmpClientLocked(in.UnlinkEmployeeID, in.ID, false); err != nil {
				return nil, err
			}
		}
		_ = s.saveLocked()
		c, _ := s.clientLocked(in.ID)
		c.EmployeeIDs = s.employeesOfClientLocked(in.ID)
		return c, nil
	}
	switch kind {
	case "employees":
		for i := range s.Employees {
			if s.Employees[i].ID != in.ID {
				continue
			}
			if name := strings.TrimSpace(in.Name); name != "" {
				name, err := s.uniqueKindNameLocked("employees", name, in.ID)
				if err != nil {
					return nil, err
				}
				s.Employees[i].Name = name
			}
			if strings.TrimSpace(in.Title) != "" {
				s.Employees[i].Title = strings.TrimSpace(in.Title)
			}
			if strings.TrimSpace(in.Number) != "" {
				num, err := s.uniqueEmpNumberLocked(in.ID, in.Number)
				if err != nil {
					return nil, err
				}
				s.Employees[i].Number = num
			}
			if in.TelegramID != 0 {
				u, err := s.requirePoolUserLocked(in.TelegramID, in.ID)
				if err != nil {
					return nil, err
				}
				s.Employees[i].TelegramID = u.ID
				if strings.TrimSpace(s.Employees[i].Name) == "" {
					s.Employees[i].Name = strings.TrimSpace(u.Name)
				}
			}
			if in.LinkManagerID != 0 {
				if err := s.setEmpMgrLocked(in.ID, in.LinkManagerID, true); err != nil {
					return nil, err
				}
			}
			if in.UnlinkManagerID != 0 {
				if err := s.setEmpMgrLocked(in.ID, in.UnlinkManagerID, false); err != nil {
					return nil, err
				}
			}
			if in.LinkClientID != 0 {
				if err := s.setEmpClientLocked(in.ID, in.LinkClientID, true); err != nil {
					return nil, err
				}
			}
			if in.UnlinkClientID != 0 {
				if err := s.setEmpClientLocked(in.ID, in.UnlinkClientID, false); err != nil {
					return nil, err
				}
			}
			if in.LinkPositionID != 0 {
				if err := s.setEmpPosLocked(in.ID, in.LinkPositionID, true); err != nil {
					return nil, err
				}
			}
			if in.UnlinkPositionID != 0 {
				if err := s.setEmpPosLocked(in.ID, in.UnlinkPositionID, false); err != nil {
					return nil, err
				}
			}
			if in.PositionIDs != nil {
				s.Employees[i].PositionIDs = s.cleanPositionIDsLocked(in.PositionIDs)
			}
			_ = s.saveLocked()
			return s.Employees[i], nil
		}
		return nil, fmt.Errorf("сотрудник не найден")
	case "managers":
		for i := range s.Managers {
			if s.Managers[i].ID != in.ID {
				continue
			}
			if name := strings.TrimSpace(in.Name); name != "" {
				name, err := s.uniqueKindNameLocked("managers", name, in.ID)
				if err != nil {
					return nil, err
				}
				s.Managers[i].Name = name
			}
			if strings.TrimSpace(in.Abbrev) != "" {
				ab, err := s.uniqueMgrAbbrevLocked(in.ID, in.Abbrev)
				if err != nil {
					return nil, err
				}
				s.Managers[i].Abbrev = ab
			}
			_ = s.saveLocked()
			s.Managers[i].EmployeeIDs = s.employeesOfManagerLocked(in.ID)
			return s.Managers[i], nil
		}
		return nil, fmt.Errorf("менеджер не найден")
	case "clients":
		for i := range s.Clients {
			if s.Clients[i].ID != in.ID {
				continue
			}
			if name := strings.TrimSpace(in.Name); name != "" {
				name, err := s.uniqueKindNameLocked("clients", name, in.ID)
				if err != nil {
					return nil, err
				}
				s.Clients[i].Name = name
			}
			if strings.TrimSpace(in.WorkID) != "" {
				s.Clients[i].WorkID = strings.TrimSpace(in.WorkID)
			}
			_ = s.saveLocked()
			s.Clients[i].EmployeeIDs = s.employeesOfClientLocked(in.ID)
			return s.Clients[i], nil
		}
		return nil, fmt.Errorf("клиент не найден")
	case "counterparties":
		for i := range s.Counterparties {
			if s.Counterparties[i].ID != in.ID {
				continue
			}
			if name := strings.TrimSpace(in.Name); name != "" {
				name, err := s.uniqueKindNameLocked("counterparties", name, in.ID)
				if err != nil {
					return nil, err
				}
				s.Counterparties[i].Name = name
			}
			if strings.TrimSpace(in.WorkID) != "" {
				s.Counterparties[i].WorkID = strings.TrimSpace(in.WorkID)
			}
			_ = s.saveLocked()
			return s.Counterparties[i], nil
		}
		return nil, fmt.Errorf("контрагент не найден")
	case "positions":
		for i := range s.Positions {
			if s.Positions[i].ID != in.ID {
				continue
			}
			if name := strings.TrimSpace(in.Name); name != "" {
				name, err := s.uniqueKindNameLocked("positions", name, in.ID)
				if err != nil {
					return nil, err
				}
				s.Positions[i].Name = name
			}
			if in.Access != nil {
				s.Positions[i].Access = NormalizeAccess(in.Access)
			}
			if in.LinkEmployeeID != 0 {
				if err := s.setEmpPosLocked(in.LinkEmployeeID, in.ID, true); err != nil {
					return nil, err
				}
			}
			if in.UnlinkEmployeeID != 0 {
				if err := s.setEmpPosLocked(in.UnlinkEmployeeID, in.ID, false); err != nil {
					return nil, err
				}
			}
			_ = s.saveLocked()
			s.Positions[i].EmployeeIDs = s.employeesOfPositionLocked(in.ID)
			s.Positions[i].InUse = len(s.Positions[i].EmployeeIDs) > 0
			s.Positions[i].Access = s.Positions[i].Access.Copy()
			return s.Positions[i], nil
		}
		return nil, fmt.Errorf("должность не найдена")
	default:
		return nil, fmt.Errorf("неизвестный справочник")
	}
}

func (s *Store) catalogOwnerLocked(kind string, id int64) (int64, bool) {
	switch strings.TrimSpace(kind) {
	case "managers":
		for _, m := range s.Managers {
			if m.ID == id {
				return m.CreatedBy, true
			}
		}
	case "clients":
		for _, c := range s.Clients {
			if c.ID == id {
				return c.CreatedBy, true
			}
		}
	case "counterparties":
		for _, c := range s.Counterparties {
			if c.ID == id {
				return c.CreatedBy, true
			}
		}
	}
	return 0, false
}

func (s *Store) CatalogDelete(kind string, id, by int64, admin bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == 0 {
		return fmt.Errorf("нужен id")
	}
	kind = strings.TrimSpace(kind)
	if kind == "managers" || kind == "clients" || kind == "counterparties" {
		owner, ok := s.catalogOwnerLocked(kind, id)
		if !ok {
			if kind == "managers" {
				return fmt.Errorf("менеджер не найден")
			}
			if kind == "clients" {
				return fmt.Errorf("клиент не найден")
			}
			return fmt.Errorf("контрагент не найден")
		}
		if !admin && (owner == 0 || owner != by) {
			return fmt.Errorf("удалить может админ или тот, кто создал")
		}
	} else if !admin {
		return fmt.Errorf("это удаляет администратор")
	}
	switch kind {
	case "employees":
		n := s.Employees[:0]
		ok := false
		for _, e := range s.Employees {
			if e.ID == id {
				ok = true
				continue
			}
			n = append(n, e)
		}
		if !ok {
			return fmt.Errorf("сотрудник не найден")
		}
		s.Employees = n
	case "managers":
		for i := range s.Employees {
			s.Employees[i].ManagerIDs = dropID(s.Employees[i].ManagerIDs, id)
		}
		n := s.Managers[:0]
		ok := false
		for _, e := range s.Managers {
			if e.ID == id {
				ok = true
				continue
			}
			n = append(n, e)
		}
		if !ok {
			return fmt.Errorf("менеджер не найден")
		}
		s.Managers = n
	case "clients":
		for i := range s.Employees {
			s.Employees[i].ClientIDs = dropID(s.Employees[i].ClientIDs, id)
		}
		n := s.Clients[:0]
		ok := false
		for _, e := range s.Clients {
			if e.ID == id {
				ok = true
				continue
			}
			n = append(n, e)
		}
		if !ok {
			return fmt.Errorf("клиент не найден")
		}
		s.Clients = n
	case "counterparties":
		n := s.Counterparties[:0]
		ok := false
		var name string
		for _, e := range s.Counterparties {
			if e.ID == id {
				ok = true
				name = e.Name
				continue
			}
			n = append(n, e)
		}
		if !ok {
			return fmt.Errorf("контрагент не найден")
		}
		s.Counterparties = n
		if name != "" {
			cps := s.SaldoCPs[:0]
			for _, x := range s.SaldoCPs {
				if !strings.EqualFold(x, name) {
					cps = append(cps, x)
				}
			}
			s.SaldoCPs = cps
		}
	case "positions":
		n := s.Positions[:0]
		ok := false
		for _, e := range s.Positions {
			if e.ID == id {
				ok = true
				continue
			}
			n = append(n, e)
		}
		if !ok {
			return fmt.Errorf("должность не найдена")
		}
		s.Positions = n
		for i := range s.Employees {
			s.Employees[i].PositionIDs = dropID(s.Employees[i].PositionIDs, id)
		}
	default:
		return fmt.Errorf("неизвестный справочник")
	}
	return s.saveLocked()
}

func (s *Store) managerLocked(id int64) (Manager, bool) {
	for _, m := range s.Managers {
		if m.ID == id {
			return m, true
		}
	}
	return Manager{}, false
}

func (s *Store) clientLocked(id int64) (Client, bool) {
	for _, m := range s.Clients {
		if m.ID == id {
			return m, true
		}
	}
	return Client{}, false
}

func (s *Store) counterpartyLocked(id int64) (Counterparty, bool) {
	for _, m := range s.Counterparties {
		if m.ID == id {
			return m, true
		}
	}
	return Counterparty{}, false
}

func (s *Store) employeeLocked(id int64) (Employee, bool) {
	for _, m := range s.Employees {
		if m.ID == id {
			return m, true
		}
	}
	return Employee{}, false
}

func (s *Store) RevokeEmployeeAccess(empID int64, protect func(int64) bool) (Employee, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.Employees {
		if s.Employees[i].ID == empID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Employee{}, fmt.Errorf("сотрудник не найден")
	}
	tgID := s.Employees[idx].TelegramID
	if tgID == 0 {
		return Employee{}, fmt.Errorf("у этой карточки нет Telegram — Mini App и так закрыт")
	}
	if protect != nil && protect(tgID) {
		return Employee{}, fmt.Errorf("у владельца нельзя забрать доступ")
	}
	out := s.Users[:0]
	found := false
	for _, u := range s.Users {
		if u.ID == tgID {
			if u.Role == RoleOwner {
				return Employee{}, fmt.Errorf("у владельца нельзя забрать доступ")
			}
			found = true
			continue
		}
		out = append(out, u)
	}
	if !found {
		return s.Employees[idx], fmt.Errorf("доступа в Доступах уже нет")
	}
	s.Users = out
	_ = s.saveLocked()
	return s.Employees[idx], nil
}

func (s *Store) UnbindEmployeeTelegram(tgID int64) error {
	if tgID == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for i := range s.Employees {
		if s.Employees[i].TelegramID == tgID {
			s.Employees[i].TelegramID = 0
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.saveLocked()
}

func (s *Store) BindEmployeeTelegram(empID, tgID int64) error {
	if empID == 0 || tgID == 0 {
		return fmt.Errorf("нужен сотрудник и Telegram ID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.userLocked(tgID); !ok {
		return fmt.Errorf("сначала выдайте доступ в Доступах")
	}
	found := false
	for i := range s.Employees {
		if s.Employees[i].ID == empID {
			found = true
			if s.Employees[i].TelegramID != 0 && s.Employees[i].TelegramID != tgID {
				return fmt.Errorf("эта карточка уже привязана к другому Telegram")
			}
		}
		if s.Employees[i].ID != empID && s.Employees[i].TelegramID == tgID {
			s.Employees[i].TelegramID = 0
		}
	}
	if !found {
		return fmt.Errorf("сотрудник не найден")
	}
	for i := range s.Employees {
		if s.Employees[i].ID == empID {
			s.Employees[i].TelegramID = tgID
			break
		}
	}
	return s.saveLocked()
}

func (s *Store) employeeByTGLocked(tgID int64) (Employee, bool) {
	if tgID == 0 {
		return Employee{}, false
	}
	for _, e := range s.Employees {
		if e.TelegramID == tgID {
			return e, true
		}
	}
	return Employee{}, false
}

func (s *Store) RequestViews() []RequestView {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]RequestView, 0, len(s.Requests))
	for _, r := range s.Requests {
		if r.Status != "deleted" {
			out = append(out, s.viewRequestLocked(r))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func (s *Store) RequestViewByID(id int64) (RequestView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.Requests {
		if r.ID == id {
			return s.viewRequestLocked(r), nil
		}
	}
	return RequestView{}, fmt.Errorf("заявка не найдена")
}

func (s *Store) viewRequestLocked(r Request) RequestView {
	v := RequestView{Request: r}
	if e, ok := s.employeeLocked(r.EmployeeID); ok {
		v.EmployeeName = e.Name
	}
	if m, ok := s.managerLocked(r.ManagerID); ok {
		v.ManagerName = m.Name
		v.ManagerAbbrev = m.Abbrev
	}
	if c, ok := s.clientLocked(r.ClientID); ok {
		v.ClientName = c.Name
		v.ClientWorkID = c.WorkID
	}
	if c, ok := s.counterpartyLocked(r.CounterpartyID); ok {
		v.CounterpartyName = c.Name
		v.CounterpartyWorkID = c.WorkID
	}
	pending, sent := false, false
	for _, p := range s.Payments {
		if p.RequestID != r.ID {
			continue
		}
		if p.Status == "sent" {
			sent = true
		} else {
			pending = true
		}
	}
	if pending {
		v.PaymentStatus = "pending"
	} else if sent {
		v.PaymentStatus = "sent"
	}
	for _, f := range s.DealFiles {
		if f.RequestID != r.ID {
			continue
		}
		if f.Kind == "file" {
			v.FilesCount++
		} else {
			v.MessagesCount++
		}
	}
	for _, f := range s.VaultFiles {
		if f.RequestID == r.ID && r.ID != 0 {
			v.DocumentID, v.DocumentBy = f.ID, f.CreatedBy
			v.DocumentPlace = s.vaultLabelLocked(f.Scope, f.OwnerID, f.Folder)
			break
		}
	}

	return v
}
