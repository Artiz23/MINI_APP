package appdb

import "fmt"

type RequestPermission struct {
	CanView   bool `json:"can_view"`
	CanEdit   bool `json:"can_edit"`
	CanDelete bool `json:"can_delete"`
	IsMine    bool `json:"is_mine"`
	IsAdmin   bool `json:"is_admin"`
}

func (s *Store) actorCanEditRequestLocked(id, by int64) bool {
	r, ok := s.requestByIDLocked(id)
	if !ok {
		return false
	}
	u, _ := s.userLocked(by)
	return s.requestPermissionLocked(r, by, u.Role == RoleOwner || u.Role == RoleAdmin).CanEdit
}

func (s *Store) requestPermissionLocked(r Request, by int64, admin bool) RequestPermission {
	mine := false
	if r.EmployeeID != 0 {
		e, ok := s.employeeByTGLocked(by)
		mine = ok && e.ID == r.EmployeeID
	} else {
		mine = r.CreatedBy == by
	}
	visible := r.Status != "deleted"
	return RequestPermission{CanView: visible, CanEdit: visible && (mine || admin), CanDelete: visible && (mine || admin), IsMine: mine, IsAdmin: admin}
}

func (s *Store) RequestPermissions(id, by int64, admin bool) (RequestPermission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requestByIDLocked(id)
	if !ok {
		return RequestPermission{}, fmt.Errorf("заявка не найдена")
	}
	return s.requestPermissionLocked(r, by, admin), nil
}

// Resolve existing references as well as submitted references: changing a task's
// request_id must authorize both the old request and its new destination.
func (s *Store) MutationRequestIDs(kind string, id, requestID int64, ref string, by int64) []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := []int64{}
	add := func(v int64) {
		if v > 0 {
			ids = append(ids, v)
		}
	}
	add(requestID)
	if ref != "" {
		for _, r := range s.Requests {
			if r.UID == ref {
				add(r.ID)
			}
		}
	}
	if id > 0 {
		switch kind {
		case "requests":
			add(id)
		case "approvals":
			for _, v := range s.Approvals {
				if v.ID == id {
					add(v.RequestID)
				}
			}
		case "payments":
			for _, v := range s.Payments {
				if v.ID == id {
					add(v.RequestID)
				}
			}
		case "tasks":
			for _, v := range s.Tasks {
				if v.ID == id {
					add(v.RequestID)
				}
			}
		case "appeals":
			for _, v := range s.Appeals {
				if v.ID == id {
					add(v.RequestID)
				}
			}
		case "compliance":
			for _, v := range s.Compliances {
				if v.ID == id {
					add(v.RequestID)
				}
			}
		case "deal-files":
			for _, v := range s.DealFiles {
				if v.ID == id {
					add(v.RequestID)
				}
			}
		case "saldo":
			for _, v := range s.SaldoOps {
				if v.ID == id {
					add(v.RequestID)
				}
			}
		case "disputes":
			for _, v := range s.Disputes {
				if v.ID == id {
					for _, r := range s.Requests {
						if r.UID == v.RefUID {
							add(r.ID)
						}
					}
				}
			}
		}
	}
	if kind == "attach" {
		for _, w := range s.AttachWaits {
			if w.UserID == by {
				add(w.RequestID)
			}
		}
	}
	return ids
}
