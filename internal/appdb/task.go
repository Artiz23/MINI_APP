package appdb

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	TaskNew        = "new"
	TaskInProgress = "in_progress"
	TaskDocs       = "docs"
	TaskDone       = "done"
)

type Task struct {
	ID          int64     `json:"id"`
	UID         string    `json:"uid"`
	Title       string    `json:"title"`
	Text        string    `json:"text,omitempty"`
	Status      string    `json:"status"`
	RequestID   int64     `json:"request_id,omitempty"`
	EmployeeID  int64     `json:"employee_id,omitempty"`
	ManagerID   int64     `json:"manager_id,omitempty"`
	CreatedBy   int64     `json:"created_by"`
	CreatedName string    `json:"created_name"`
	CreatedAt   time.Time `json:"created_at"`
	MovedAt     time.Time `json:"moved_at,omitempty"`
}

type TaskView struct {
	Task
	RequestTitle     string `json:"request_title,omitempty"`
	RequestUID       string `json:"request_uid,omitempty"`
	EmployeeName     string `json:"employee_name,omitempty"`
	ClientName       string `json:"client_name,omitempty"`
	ManagerName      string `json:"manager_name,omitempty"`
	CounterpartyName string `json:"counterparty_name,omitempty"`
}

func NormalizeTaskStatus(st string) string {
	switch strings.TrimSpace(st) {
	case TaskInProgress, TaskDocs, TaskDone:
		return st
	default:
		return TaskNew
	}
}

func (s *Store) taskViewLocked(t Task) TaskView {
	v := TaskView{Task: t}
	if e, ok := s.employeeLocked(t.EmployeeID); ok {
		v.EmployeeName = e.Name
	}
	if t.RequestID == 0 {
		return v
	}
	r, ok := s.requestByIDLocked(t.RequestID)
	if !ok {
		return v
	}
	rv := s.viewRequestLocked(r)
	v.RequestTitle = rv.Title
	v.RequestUID = rv.UID
	v.ClientName = rv.ClientName
	v.ManagerName = rv.ManagerName
	v.CounterpartyName = rv.CounterpartyName
	if v.EmployeeName == "" {
		v.EmployeeName = rv.EmployeeName
	}
	return v
}

func (s *Store) TasksViews() []TaskView {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TaskView, 0, len(s.Tasks))
	for _, t := range s.Tasks {
		out = append(out, s.taskViewLocked(t))
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].MovedAt.Equal(out[j].MovedAt) {
			return out[i].MovedAt.After(out[j].MovedAt)
		}
		return out[i].ID > out[j].ID
	})
	return out
}

func (s *Store) AddTask(title, text, status string, requestID, employeeID, managerID, by int64, byName string, staff bool) (TaskView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	title = strings.TrimSpace(title)
	if requestID != 0 {
		r, ok := s.requestByIDLocked(requestID)
		if !ok {
			return TaskView{}, fmt.Errorf("заявка не найдена")
		}
		if managerID == 0 {
			managerID = r.ManagerID
		}
		if employeeID == 0 {
			employeeID = r.EmployeeID
		}
	}
	if employeeID != 0 {
		if _, ok := s.employeeLocked(employeeID); !ok {
			return TaskView{}, fmt.Errorf("сотрудник не найден")
		}
	}
	emp, ok := s.employeeLocked(employeeID)
	if !ok {
		emp, ok = s.actorEmployeeLocked(by, byName, staff)
		if ok {
			employeeID = emp.ID
		}
	}
	if !ok {
		return TaskView{}, fmt.Errorf("вас нет в справочнике сотрудников. Админ добавит карточку с номером — Telegram ID подтянется из Доступов сам")
	}
	mgr, ok := s.managerLocked(managerID)
	if !ok {
		return TaskView{}, fmt.Errorf("выберите заявку или менеджера — нужна аббревиатура для номера")
	}
	now := time.Now()
	code, err := s.nextDealCodeLocked(mgr, emp, now)
	if err != nil {
		return TaskView{}, err
	}
	if title == "" {
		title = code
	}
	id := s.nextLocked()
	t := Task{
		ID: id, UID: code, Title: title, Text: strings.TrimSpace(text),
		Status: NormalizeTaskStatus(status), RequestID: requestID, EmployeeID: employeeID, ManagerID: mgr.ID,
		CreatedBy: by, CreatedName: byName, CreatedAt: now, MovedAt: now,
	}
	s.Tasks = append(s.Tasks, t)
	_ = s.saveLocked()
	return s.taskViewLocked(t), nil
}

func (s *Store) SetTaskStatus(id int64, status string) (TaskView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Tasks {
		if s.Tasks[i].ID != id {
			continue
		}
		s.Tasks[i].Status = NormalizeTaskStatus(status)
		s.Tasks[i].MovedAt = time.Now()
		_ = s.saveLocked()
		return s.taskViewLocked(s.Tasks[i]), nil
	}
	return TaskView{}, fmt.Errorf("задача не найдена")
}

func (s *Store) UpdateTask(id int64, title, text string, requestID, employeeID, by int64, _ bool) (TaskView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Tasks {
		if s.Tasks[i].ID != id {
			continue
		}
		if s.Tasks[i].CreatedBy != by {
			return TaskView{}, fmt.Errorf("править может только тот, кто создал")
		}
		if title = strings.TrimSpace(title); title != "" {
			s.Tasks[i].Title = title
		}
		s.Tasks[i].Text = strings.TrimSpace(text)
		if requestID != 0 {
			if _, ok := s.requestByIDLocked(requestID); !ok {
				return TaskView{}, fmt.Errorf("заявка не найдена")
			}
			s.Tasks[i].RequestID = requestID
		}
		s.Tasks[i].EmployeeID = employeeID
		_ = s.saveLocked()
		return s.taskViewLocked(s.Tasks[i]), nil
	}
	return TaskView{}, fmt.Errorf("задача не найдена")
}

func (s *Store) DeleteTask(id, by int64, _ bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("задача не найдена")
	}
	if s.Tasks[idx].CreatedBy != by {
		return fmt.Errorf("удалить может только тот, кто создал")
	}
	s.Tasks = append(s.Tasks[:idx], s.Tasks[idx+1:]...)
	return s.saveLocked()
}

func (s *Store) TasksOpenCountLocked() (open, nw, progress, docs int) {
	for _, t := range s.Tasks {
		switch t.Status {
		case TaskDone:
			continue
		case TaskInProgress:
			progress++
		case TaskDocs:
			docs++
		default:
			nw++
		}
		open++
	}
	return
}
