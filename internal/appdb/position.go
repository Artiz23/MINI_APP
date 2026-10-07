package appdb

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Position struct {
	ID          int64     `json:"id"`
	UID         string    `json:"uid"`
	Name        string    `json:"name"`
	Access      Access    `json:"access,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	EmployeeIDs []int64   `json:"employee_ids,omitempty"`
	InUse       bool      `json:"in_use,omitempty"`
}

func defaultPositions() []struct {
	Name   string
	Access Access
} {
	return []struct {
		Name   string
		Access Access
	}{
		{"Операционист", Access{
			SecTasks: true, SecRequests: true, SecSaldo: true, SecApprovals: true, SecPayments: true,
			SecDirectory: true, SecRates: true, SecHolidays: true, SecBalance: true, SecCompliance: true,
			SecAppeals: true, SecDocuments: true,
		}},
		{"Документалист", Access{
			SecTasks: true, SecDirectory: true, SecRates: true, SecHolidays: true, SecBalance: true, SecCompliance: true,
			SecAppealDocs: true, SecDocuments: true,
		}},
		{"Юрист", Access{
			SecTasks: true, SecRates: true, SecHolidays: true, SecCompliance: true, SecAppealLawyer: true,
		}},
	}
}

func (s *Store) seedPositionsLocked() bool {
	if s.Positions == nil {
		s.Positions = []Position{}
	}
	changed := false
	if len(s.Positions) == 0 {
		now := time.Now()
		for _, d := range defaultPositions() {
			id := s.nextLocked()
			s.Positions = append(s.Positions, Position{
				ID: id, UID: uid("pos", id), Name: d.Name, Access: d.Access.Copy(), CreatedAt: now,
			})
		}
		changed = true
	}
	return s.grantDocumentsAccessLocked() || changed
}

func (s *Store) grantDocumentsAccessLocked() bool {
	changed := false
	for i := range s.Positions {
		n := strings.ToLower(strings.TrimSpace(s.Positions[i].Name))
		if n != "операционист" && n != "документалист" {
			continue
		}
		if s.Positions[i].Access == nil {
			s.Positions[i].Access = Access{}
		}
		if !s.Positions[i].Access[SecDocuments] {
			s.Positions[i].Access[SecDocuments] = true
			changed = true
		}
	}
	return changed
}

func (s *Store) PositionsCopy() []Position {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Position(nil), s.Positions...)
	if out == nil {
		out = []Position{}
	}
	for i := range out {
		out[i].Access = out[i].Access.Copy()
		out[i].EmployeeIDs = s.employeesOfPositionLocked(out[i].ID)
		out[i].InUse = len(out[i].EmployeeIDs) > 0
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

func (s *Store) employeesOfPositionLocked(id int64) []int64 {
	var out []int64
	for _, e := range s.Employees {
		if hasID(e.PositionIDs, id) {
			out = append(out, e.ID)
		}
	}
	return out
}

func (s *Store) positionLocked(id int64) (Position, bool) {
	for _, p := range s.Positions {
		if p.ID == id {
			return p, true
		}
	}
	return Position{}, false
}

func (s *Store) unionPositionsLocked(ids []int64) Access {
	out := Access{}
	for _, id := range ids {
		p, ok := s.positionLocked(id)
		if !ok {
			continue
		}
		for k, v := range p.Access {
			if v {
				out[k] = true
			}
		}
	}
	return NormalizeAccess(out)
}

func (s *Store) AccessFromEmployee(tgID int64) Access {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.employeeByTGLocked(tgID)
	if !ok {
		return Access{}
	}
	return s.unionPositionsLocked(e.PositionIDs)
}

func (s *Store) OrphanPositionWarn() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.orphanPositionWarnLocked()
}

func (s *Store) orphanPositionWarnLocked() int {
	n := 0
	for _, e := range s.Employees {
		for _, id := range e.PositionIDs {
			if _, ok := s.positionLocked(id); !ok {
				n++
			}
		}
	}
	return n
}

func (s *Store) uniqueKindNameLocked(kind, name string, exceptID int64) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("нужно имя")
	}
	key := normPersonName(name)
	type row struct {
		id   int64
		name string
	}
	var rows []row
	switch kind {
	case "employees":
		for _, e := range s.Employees {
			rows = append(rows, row{e.ID, e.Name})
		}
	case "managers":
		for _, e := range s.Managers {
			rows = append(rows, row{e.ID, e.Name})
		}
	case "clients":
		for _, e := range s.Clients {
			rows = append(rows, row{e.ID, e.Name})
		}
	case "counterparties":
		for _, e := range s.Counterparties {
			rows = append(rows, row{e.ID, e.Name})
		}
	case "positions":
		for _, e := range s.Positions {
			rows = append(rows, row{e.ID, e.Name})
		}
	}
	for _, r := range rows {
		if r.id != exceptID && normPersonName(r.name) == key {
			return "", fmt.Errorf("такая запись уже есть в справочнике")
		}
	}
	return name, nil
}

func (s *Store) setEmpPosLocked(empID, posID int64, on bool) error {
	if _, ok := s.employeeLocked(empID); !ok {
		return fmt.Errorf("сотрудник не найден")
	}
	if _, ok := s.positionLocked(posID); !ok {
		return fmt.Errorf("должность не найдена")
	}
	for i := range s.Employees {
		if s.Employees[i].ID != empID {
			continue
		}
		if on {
			s.Employees[i].PositionIDs = uniqIDs(append(s.Employees[i].PositionIDs, posID))
		} else {
			s.Employees[i].PositionIDs = dropID(s.Employees[i].PositionIDs, posID)
		}
		return nil
	}
	return fmt.Errorf("сотрудник не найден")
}

func (s *Store) stripOrphanPositionsLocked() bool {
	changed := false
	for i := range s.Employees {
		cleaned := s.cleanPositionIDsLocked(s.Employees[i].PositionIDs)
		if len(cleaned) != len(s.Employees[i].PositionIDs) {
			s.Employees[i].PositionIDs = cleaned
			changed = true
		}
	}
	return changed
}

func (s *Store) cleanPositionIDsLocked(ids []int64) []int64 {
	var out []int64
	for _, id := range ids {
		if _, ok := s.positionLocked(id); ok {
			out = append(out, id)
		}
	}
	return uniqIDs(out)
}

func hasID(ids []int64, id int64) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}
