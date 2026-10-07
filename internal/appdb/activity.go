package appdb

import "time"

type ActivitySlice struct {
	UserID     int64  `json:"user_id"`
	EmployeeID int64  `json:"employee_id"`
	Date       string `json:"date"`
	Bucket     string `json:"bucket"`
}

func (s *Store) RecordActivity(by int64, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, _ := s.employeeByTGLocked(by)
	now = now.In(Moscow())
	bucket := now.Truncate(5 * time.Minute).Format(time.RFC3339)
	for i := len(s.ActivitySlices) - 1; i >= 0; i-- {
		v := s.ActivitySlices[i]
		if v.UserID == by && v.Bucket == bucket {
			return nil
		}
	}
	s.ActivitySlices = append(s.ActivitySlices, ActivitySlice{UserID: by, EmployeeID: e.ID, Date: now.Format("2006-01-02"), Bucket: bucket})
	if err := s.saveLocked(); err != nil {
		s.ActivitySlices = s.ActivitySlices[:len(s.ActivitySlices)-1]
		return err
	}
	return nil
}
