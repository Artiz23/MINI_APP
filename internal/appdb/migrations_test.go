package appdb

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSchemaMigrationBackupAndRestart(t *testing.T) {
	p := filepath.Join(t.TempDir(), "miniapp.json")
	original := []byte(`{"next_id":50,"requests":[{"id":1,"uid":"R1","title":"Old comment","status":"done"},{"id":2,"uid":"R2","title":"R2","status":"in_progress"},{"id":3,"status":"unrecognized"}],"payments":[{"request_id":2,"status":"sent"}]}`)
	if err := os.WriteFile(p, original, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if s.SchemaVersion != CurrentSchemaVersion || s.Requests[0].Comment != "Old comment" || s.Requests[0].Status != "success_closed" || s.Requests[0].WorkflowStage != StageComplete {
		t.Fatalf("migration: %+v", s.Requests)
	}
	if s.Requests[1].WorkflowStage != StageTask || s.Requests[1].Comment != "" {
		t.Fatalf("stage: %+v", s.Requests[1])
	}
	if s.Requests[2].Status != "unrecognized" || len(s.MigrationWarnings) != 1 {
		t.Fatal("unknown status silently changed")
	}
	backups, _ := filepath.Glob(p + ".backup-*")
	if len(backups) != 1 {
		t.Fatal(backups)
	}
	b, _ := os.ReadFile(backups[0])
	if !bytes.Equal(b, original) {
		t.Fatal("backup differs")
	}
	if _, err = Load(p); err != nil {
		t.Fatal(err)
	}
	backups, _ = filepath.Glob(p + ".backup-*")
	if len(backups) != 1 {
		t.Fatal("migration repeated")
	}
}

func TestFutureSchemaIsNotModified(t *testing.T) {
	p := filepath.Join(t.TempDir(), "miniapp.json")
	b := []byte(`{"schema_version":999}`)
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("accepted future schema")
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(b, after) {
		t.Fatal("modified future schema")
	}
}

func TestRequestPermissionsAndStatePersistence(t *testing.T) {
	p := filepath.Join(t.TempDir(), "miniapp.json")
	s, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	s.Employees = []Employee{{ID: 10, TelegramID: 7}, {ID: 11, TelegramID: 8}}
	s.Requests = []Request{{ID: 100, CreatedBy: 8, EmployeeID: 10, Status: "open", WorkflowStage: StageApproval}, {ID: 101, CreatedBy: 7, Status: "open"}}
	for _, tc := range []struct {
		id, by            int64
		admin, mine, edit bool
	}{{100, 7, false, true, true}, {100, 8, false, false, false}, {100, 8, true, false, true}, {101, 7, false, true, true}} {
		p, err := s.RequestPermissions(tc.id, tc.by, tc.admin)
		if err != nil || p.IsMine != tc.mine || p.CanEdit != tc.edit || !p.CanView {
			t.Fatalf("permissions %+v %+v %v", tc, p, err)
		}
	}
	if _, err = s.UpdateRequestState(100, "in_progress", "", 8, false); err == nil {
		t.Fatal("foreign mutation")
	}
	if _, err = s.EditRequestComment(100, "early", 7, false); err == nil {
		t.Fatal("comment while open")
	}
	if _, err = s.UpdateRequestState(100, "in_progress", "", 7, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EditRequestComment(100, "comment", 7, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateRequestState(100, "failed", " ", 7, false); err == nil {
		t.Fatal("empty reason")
	}
	if _, err = s.UpdateRequestState(100, "invented", "", 7, false); err == nil {
		t.Fatal("unknown state")
	}
	if _, err = s.UpdateRequestState(100, "failed", "reason", 7, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EditRequestComment(100, "late", 7, false); err == nil {
		t.Fatal("closed comment")
	}
	reloaded, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Requests[0].Comment != "comment" || reloaded.Requests[0].CloseReason != "reason" || len(reloaded.AuditEvents) != 3 {
		t.Fatal("state not persisted")
	}
}

func TestRequestWriteFailureRollsBack(t *testing.T) {
	s := &Store{path: filepath.Join(t.TempDir(), "missing", "blocked", "data.json"), NextID: 5, Requests: []Request{{ID: 1, CreatedBy: 7, Status: "open"}}}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(s.path+".tmp", 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateRequestState(1, "in_progress", "", 7, false); err == nil {
		t.Fatal("expected failure")
	}
	if s.Requests[0].Status != "open" || s.NextID != 5 || len(s.AuditEvents) != 0 {
		t.Fatal("partial update")
	}
}
