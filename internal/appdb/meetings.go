package appdb

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Meeting struct {
	ID          int64     `json:"id"`
	UID         string    `json:"uid"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	StartsAt    time.Time `json:"starts_at"`
	CreatedBy   int64     `json:"created_by"`
	CreatedName string    `json:"created_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (s *Store) MeetingsList(filter string, now time.Time) []Meeting {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Meeting{}
	for _, m := range s.Meetings {
		if filter == "upcoming" && m.StartsAt.Before(now) || filter == "past" && !m.StartsAt.Before(now) {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartsAt.Before(out[j].StartsAt) })
	return out
}
func (s *Store) SaveMeeting(in Meeting, by int64, name string, admin bool) (Meeting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	in.Title = strings.TrimSpace(in.Title)
	in.URL = strings.TrimSpace(in.URL)
	u, err := url.Parse(in.URL)
	if in.Title == "" || in.StartsAt.IsZero() || err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return Meeting{}, fmt.Errorf("укажите название, дату и ссылку http/https")
	}
	old := append([]Meeting(nil), s.Meetings...)
	previousID := s.NextID
	now := time.Now()
	if in.ID == 0 {
		in.ID = s.nextLocked()
		in.UID = uid("meeting", in.ID)
		in.CreatedBy = by
		in.CreatedName = name
		in.CreatedAt = now
		in.UpdatedAt = now
		s.Meetings = append(s.Meetings, in)
	} else {
		found := false
		for i, m := range s.Meetings {
			if m.ID != in.ID {
				continue
			}
			if !admin && m.CreatedBy != by {
				return Meeting{}, fmt.Errorf("нет права изменять встречу")
			}
			m.Title = in.Title
			m.URL = in.URL
			m.StartsAt = in.StartsAt
			m.UpdatedAt = now
			s.Meetings[i] = m
			in = m
			found = true
			break
		}
		if !found {
			return Meeting{}, fmt.Errorf("встреча не найдена")
		}
	}
	if err := s.saveLocked(); err != nil {
		s.Meetings = old
		s.NextID = previousID
		return Meeting{}, err
	}
	return in, nil
}
func (s *Store) DeleteMeeting(id, by int64, admin bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, m := range s.Meetings {
		if m.ID != id {
			continue
		}
		if !admin && m.CreatedBy != by {
			return fmt.Errorf("нет права удалять встречу")
		}
		old := s.Meetings
		s.Meetings = append(append([]Meeting(nil), old[:i]...), old[i+1:]...)
		if err := s.saveLocked(); err != nil {
			s.Meetings = old
			return err
		}
		return nil
	}
	return fmt.Errorf("встреча не найдена")
}
