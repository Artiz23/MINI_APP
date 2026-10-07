package appdb

import (
	"fmt"
	"os"
	"time"
)

const CurrentSchemaVersion = 2

type RequestStage string

const (
	StageApproval RequestStage = "approval"
	StagePayment  RequestStage = "payment"
	StageTask     RequestStage = "task"
	StageAppeal   RequestStage = "appeal"
	StageComplete RequestStage = "complete"
)

type AuditEvent struct {
	ID         int64     `json:"id"`
	ActorID    int64     `json:"actor_id"`
	EntityType string    `json:"entity_type"`
	EntityID   int64     `json:"entity_id"`
	Action     string    `json:"action"`
	Before     string    `json:"before,omitempty"`
	After      string    `json:"after,omitempty"`
	At         time.Time `json:"at"`
}

// The exact original bytes are backed up before any normalization or write.
func backupBeforeMigration(path string, data []byte) error {
	f, err := os.OpenFile(path+".backup-"+time.Now().UTC().Format("20060102-150405.000000000"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("migration backup: %w", err)
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("migration backup: %w", err)
	}
	return nil
}

func (s *Store) migrateSchemaLocked() {
	for i := range s.Requests {
		r := &s.Requests[i]
		if s.SchemaVersion < 1 && r.Comment == "" && r.Title != r.UID {
			r.Comment = r.Title
		}
		switch r.Status {
		case "done", "closed":
			r.Status = "success_closed"
		case "open", "in_progress", "success_closed", "failed", "deleted":
		default:
			s.MigrationWarnings = append(s.MigrationWarnings, fmt.Sprintf("request %d: unknown status %q", r.ID, r.Status))
		}
		if r.WorkflowStage == "" {
			r.WorkflowStage = s.inferStageLocked(*r)
		}
	}
	s.SaldoBalances = s.rebuildBalancesLocked()
	s.SchemaVersion = CurrentSchemaVersion
}

func (s *Store) inferStageLocked(r Request) RequestStage {
	if r.Status == "success_closed" || r.Status == "failed" || r.Status == "deleted" {
		return StageComplete
	}
	stage := StageApproval
	for _, a := range s.Approvals {
		if a.RequestID == r.ID && a.Status == "approved" {
			stage = StagePayment
		}
	}
	for _, p := range s.Payments {
		if p.RequestID == r.ID && p.Status == "sent" {
			stage = StageTask
			break
		}
		if p.RequestID == r.ID && p.Status == "pending" {
			stage = StagePayment
		}
	}
	for _, t := range s.Tasks {
		if t.RequestID == r.ID && t.Status != "deleted" {
			stage = StageTask
			if t.Status == TaskDone {
				stage = StageAppeal
				break
			}
		}
	}
	for _, a := range s.Appeals {
		if a.RequestID == r.ID && a.Status != "cancelled" {
			stage = StageAppeal
			if a.Status == "closed" {
				return StageComplete
			}
		}
	}
	return stage
}
