package appdb

import (
	"fmt"
	"strings"
	"time"
)

func stageRank(stage RequestStage) int {
	switch stage {
	case StageApproval:
		return 0
	case StagePayment:
		return 1
	case StageTask:
		return 2
	case StageAppeal:
		return 3
	case StageComplete:
		return 4
	}
	return -1
}
func (s *Store) syncWorkflowLocked() {
	for i, r := range s.Requests {
		inferred := s.inferStageLocked(r)
		if stageRank(inferred) > stageRank(r.WorkflowStage) {
			s.Requests[i].WorkflowStage = inferred
		}
	}
}
func (s *Store) RequireStage(id int64, stage RequestStage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requireStageLocked(id, stage)
}
func (s *Store) requireStageLocked(id int64, stage RequestStage) error {
	r, ok := s.requestByIDLocked(id)
	if !ok {
		return fmt.Errorf("заявка не найдена")
	}
	if r.Status != "in_progress" {
		return fmt.Errorf("сначала переведите заявку в работу")
	}
	if r.WorkflowStage != stage {
		return fmt.Errorf("действие недоступно на текущем этапе: %s", r.WorkflowStage)
	}
	return nil
}
func (s *Store) SkipRequestStage(id, by int64, reason string, admin bool) (Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !admin || strings.TrimSpace(reason) == "" {
		return Request{}, fmt.Errorf("пропуск доступен администратору с причиной")
	}
	for i, old := range s.Requests {
		if old.ID != id {
			continue
		}
		if old.Status != "in_progress" {
			return Request{}, fmt.Errorf("заявка должна быть в работе")
		}
		next := old
		switch old.WorkflowStage {
		case StageApproval:
			next.WorkflowStage = StagePayment
		case StagePayment:
			next.WorkflowStage = StageTask
		case StageTask:
			next.WorkflowStage = StageAppeal
		case StageAppeal:
			next.WorkflowStage = StageComplete
		default:
			return Request{}, fmt.Errorf("нет этапа для пропуска")
		}
		return s.commitRequestLocked(i, old, next, by, "skip_stage", string(old.WorkflowStage), string(next.WorkflowStage)+": "+reason)
	}
	return Request{}, fmt.Errorf("заявка не найдена")
}

func (s *Store) UpdateRequestState(id int64, status, reason string, by int64, admin bool) (Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, old := range s.Requests {
		if old.ID != id {
			continue
		}
		if !s.requestPermissionLocked(old, by, admin).CanEdit {
			return Request{}, fmt.Errorf("нет права изменять заявку")
		}
		next := old
		switch status {
		case "in_progress":
			if old.Status != "open" {
				return Request{}, fmt.Errorf("недопустимый переход статуса")
			}
		case "success_closed", "failed":
			if old.Status != "in_progress" {
				return Request{}, fmt.Errorf("закрыть можно только заявку в работе")
			}
			if status == "failed" && strings.TrimSpace(reason) == "" {
				return Request{}, fmt.Errorf("укажите причину: сделка не состоялась")
			}
			if status == "success_closed" && old.WorkflowStage != StageComplete {
				return Request{}, fmt.Errorf("сначала завершите этапы заявки")
			}
			next.CloseReason = strings.TrimSpace(reason)
			if status == "success_closed" {
				if c, ok := s.clientLocked(old.ClientID); ok && c.CommissionEnabled && s.Settings.PaymentChatID == 0 {
					return Request{}, fmt.Errorf("настройте чат оплаты для комиссии клиента")
				}
			}
		case "reopen":
			if !admin || strings.TrimSpace(reason) == "" {
				return Request{}, fmt.Errorf("повторное открытие доступно администратору с причиной")
			}
			if old.Status != "success_closed" && old.Status != "failed" {
				return Request{}, fmt.Errorf("заявка не закрыта")
			}
			status = "in_progress"
		case "deleted":
		default:
			return Request{}, fmt.Errorf("неизвестный статус")
		}
		next.Status = status
		if status == "success_closed" || status == "failed" {
			next.ClosedAt = time.Now()
		} else if status == "in_progress" {
			next.ClosedAt = time.Time{}
		}
		if status == "failed" {
			next.WorkflowStage = StageComplete
		}
		return s.commitRequestLocked(i, old, next, by, "status", old.Status, status+" "+reason)
	}
	return Request{}, fmt.Errorf("заявка не найдена")
}

func (s *Store) EditRequestComment(id int64, comment string, by int64, admin bool) (Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, old := range s.Requests {
		if old.ID != id {
			continue
		}
		if !s.requestPermissionLocked(old, by, admin).CanEdit {
			return Request{}, fmt.Errorf("нет права изменять заявку")
		}
		if old.Status != "in_progress" {
			return Request{}, fmt.Errorf("комментарий можно менять только в работе")
		}
		next := old
		next.Comment = strings.TrimSpace(comment)
		next.CommentUpdatedAt = time.Now()
		next.CommentUpdatedBy = by
		return s.commitRequestLocked(i, old, next, by, "comment", old.Comment, next.Comment)
	}
	return Request{}, fmt.Errorf("заявка не найдена")
}

func (s *Store) commitRequestLocked(i int, old, next Request, by int64, action, before, after string) (Request, error) {
	previousID := s.NextID
	previousPayments := len(s.Payments)
	if old.Status != "success_closed" && next.Status == "success_closed" {
		s.ensureClientCommissionLocked(next, by)
	}
	s.Requests[i] = next
	s.AuditEvents = append(s.AuditEvents, AuditEvent{ID: s.nextLocked(), ActorID: by, EntityType: "request", EntityID: next.ID, Action: action, Before: before, After: after, At: time.Now()})
	if err := s.saveLocked(); err != nil {
		s.Requests[i] = old
		s.NextID = previousID
		s.AuditEvents = s.AuditEvents[:len(s.AuditEvents)-1]
		s.Payments = s.Payments[:previousPayments]
		return Request{}, err
	}
	return next, nil
}
