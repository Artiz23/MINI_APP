package appdb

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func archiveFixture(t *testing.T) *Store {
	t.Helper()
	s := &Store{path: filepath.Join(t.TempDir(), "miniapp.json"), NextID: 100,
		Companies: []Company{{ID: 20, Name: "Acme"}},
		VaultTabs: []CompanyVaultTab{{ID: 21, CompanyID: 20, Contour: VaultExternal, Name: "Invoices"}},
		Requests:  []Request{{ID: 1, UID: "req-1", Title: "Test request", Notes: "Request notes", EmployeeID: 11}},
		DealFiles: []DealFile{
			{ID: 2, RequestID: 1, Kind: "file", Name: "invoice.txt", Rel: "source.txt", Mime: "text/plain", Text: "File caption"},
			{ID: 3, RequestID: 1, Kind: "note", Text: "Test note", CreatedName: "Alice"},
			{ID: 4, RequestID: 99, Kind: "note", Text: "Other request"},
			{ID: 5, RequestID: 1, Kind: "file", Name: "invoice.txt", Rel: "source2.txt", Mime: "text/plain"},
		},
		Payments:    []Payment{{ID: 6, UID: "pay-6", RequestID: 1, Text: "Pay text"}},
		Approvals:   []Approval{{ID: 7, UID: "apr-7", RequestUID: "req-1", Preview: "Approval text"}},
		SaldoOps:    []SaldoOp{{ID: 8, UID: "saldo-8", RequestID: 1, Amount: 123, Currency: "USD"}},
		Appeals:     []Appeal{{ID: 9, UID: "appeal-9", RequestID: 1, Text: "Appeal text"}},
		Tasks:       []Task{{ID: 10, UID: "task-10", RequestID: 1, Text: "Task text"}},
		Compliances: []Compliance{{ID: 11, UID: "cmp-11", RequestUID: "req-1", Text: "Compliance text"}},
		Disputes:    []Dispute{{ID: 12, RefUID: "pay-6", Text: "Dispute text", Messages: []DisputeMsg{{Text: "Reply text"}}}, {ID: 13, RefUID: "req-99", Text: "Unrelated dispute"}},
	}
	if err := os.MkdirAll(s.FilesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"source.txt": "invoice content", "source2.txt": "second content"} {
		if err := os.WriteFile(filepath.Join(s.FilesDir(), name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestArchiveRequestCompleteIndependentSnapshot(t *testing.T) {
	s := archiveFixture(t)
	done := make(chan error, 1)
	go func() { done <- s.ArchiveRequest(1, VaultCompany, 20, "external:21", 7, "Alice") }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("archive deadlocked")
	}
	loaded, err := Load(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.VaultFiles) != 1 {
		t.Fatalf("expected one complete archive, got %d", len(loaded.VaultFiles))
	}
	f := loaded.VaultFiles[0]
	if f.RequestID != 1 || f.CreatedBy != 7 || f.Folder != "external:21" {
		t.Fatalf("wrong metadata: %+v", f)
	}
	view, err := loaded.RequestBundle(1)
	if err != nil {
		t.Fatal(err)
	}
	if view.DocumentID != f.ID || view.DocumentBy != 7 || view.DocumentPlace != "Acme · внешний контур · Invoices" {
		t.Fatalf("missing document link: %+v", view.RequestView)
	}
	if err := os.Remove(filepath.Join(s.FilesDir(), "source.txt")); err != nil {
		t.Fatal(err)
	}
	s.Requests[0].Notes = "Changed after saving"
	_, x, err := loaded.RequestArchivePreview(f.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.Request.Files) != 3 || len(x.Request.Payments) != 1 || len(x.Request.Approvals) != 1 || len(x.Request.Saldo) != 1 || len(x.Request.Appeals) != 1 || len(x.Tasks) != 1 || len(x.Compliances) != 1 || len(x.Disputes) != 1 || len(x.Entries) != 2 {
		t.Fatalf("incomplete snapshot: %+v", x)
	}
	for _, want := range []string{"Request notes", "Test note", "File caption", "Pay text", "Approval text", "123 USD", "Appeal text", "Task text", "Compliance text", "Reply text"} {
		if !strings.Contains(x.Report(), want) {
			t.Errorf("report missing %q", want)
		}
	}
	if strings.Contains(x.Report(), "Other request") || strings.Contains(x.Report(), "Unrelated dispute") {
		t.Fatal("unrelated data leaked into archive")
	}
	if x.Entries[0].Path == x.Entries[1].Path {
		t.Fatal("duplicate attachment names collided")
	}
	_, data, err := loaded.RequestArchiveEntry(f.ID, "files/2_invoice.txt")
	if err != nil || string(data) != "invoice content" {
		t.Fatalf("snapshot attachment lost: %q %v", data, err)
	}
	if _, _, err := loaded.RequestArchiveEntry(f.ID, "../miniapp.json"); err == nil {
		t.Fatal("unexpected archive entry accepted")
	}
	_, data, err = loaded.VaultFileBytes(f.ID)
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(z.File) != 4 {
		t.Fatalf("want manifest, report and two attachments, got %d", len(z.File))
	}
	if err := loaded.ArchiveRequest(1, VaultMisc, 0, VaultOther, 7, "Alice"); err == nil {
		t.Fatal("duplicate archive accepted")
	}
}

func TestArchiveRequestDeletionPreservesOriginals(t *testing.T) {
	s := archiveFixture(t)
	if err := s.ArchiveRequest(1, VaultMisc, 0, VaultOther, 7, "Alice"); err != nil {
		t.Fatal(err)
	}
	f := s.VaultFiles[0]
	if err := s.DeleteVaultFile(f.ID, 8, false); err == nil {
		t.Fatal("other user deleted archive")
	}
	if err := s.DeleteVaultFile(f.ID, 7, false); err != nil {
		t.Fatal(err)
	}
	if len(s.VaultFiles) != 0 || len(s.Requests) != 1 || len(s.DealFiles) != 4 {
		t.Fatal("delete touched source records")
	}
	if _, err := os.Stat(filepath.Join(s.FilesDir(), "source.txt")); err != nil {
		t.Fatal("delete touched source file", err)
	}
	if _, err := os.Stat(filepath.Join(s.FilesDir(), f.Rel)); !os.IsNotExist(err) {
		t.Fatal("archive bytes were not removed", err)
	}
	b, _ := s.RequestBundle(1)
	if b.DocumentID != 0 {
		t.Fatal("stale saved state")
	}
	if err := s.ArchiveRequest(1, VaultMisc, 0, VaultOther, 7, "Alice"); err != nil {
		t.Fatal("resave failed", err)
	}
	if err := s.DeleteVaultFile(s.VaultFiles[0].ID, 9, true); err != nil {
		t.Fatal("admin delete failed", err)
	}
}

func TestArchiveRequestFailuresLeaveNoPartialArchive(t *testing.T) {
	for _, scenario := range []string{"missing-file", "missing-tab", "wrong-contour", "save-error"} {
		t.Run(scenario, func(t *testing.T) {
			s := archiveFixture(t)
			folder := "external:21"
			switch scenario {
			case "missing-file":
				s.DealFiles[0].Rel = "missing.txt"
			case "missing-tab":
				folder = "external:999"
			case "wrong-contour":
				folder = "internal:21"
			case "save-error":
				if err := os.Mkdir(s.path, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.ArchiveRequest(1, VaultCompany, 20, folder, 7, "Alice"); err == nil {
				t.Fatal("expected error")
			}
			if len(s.VaultFiles) != 0 || s.NextID != 100 {
				t.Fatal("partial archive committed")
			}
			files, _ := filepath.Glob(filepath.Join(s.FilesDir(), "request-*.zip"))
			if len(files) != 0 {
				t.Fatalf("partial ZIP left: %v", files)
			}
		})
	}
}

func TestArchiveDestinationsIncludeExistingTabs(t *testing.T) {
	s := archiveFixture(t)
	found := false
	for _, d := range s.VaultDestinations() {
		if d.Scope == VaultCompany && d.OwnerID == 20 && d.Folder == "external:21" && d.Label == "Acme · внешний контур · Invoices" {
			found = true
		}
	}
	if !found {
		t.Fatal("existing company tab missing")
	}
}
