package hub

import (
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"tg-bot-orh3/MINI_APP/internal/appdb"
)

func TestArchiveAPIFlowAndPermissions(t *testing.T) {
	db, err := appdb.Load(filepath.Join(t.TempDir(), "miniapp.json"))
	if err != nil {
		t.Fatal(err)
	}
	db.Requests = []appdb.Request{{ID: 1, UID: "req-1", Title: "Test"}}
	db.DealFiles = []appdb.DealFile{{ID: 2, RequestID: 1, Kind: "note", Text: "A message"}}
	a := &API{db: db}
	user := session{ID: 7, Name: "Alice", Access: appdb.Access{appdb.SecRequests: true, appdb.SecDocuments: true}}
	body := `{"request_id":1,"scope":"misc","owner_id":0,"folder":"other"}`
	denied := httptest.NewRecorder()
	a.archiveRequest(denied, httptest.NewRequest("POST", "/api/requests/archive", strings.NewReader(body)), session{ID: 7, Access: appdb.Access{appdb.SecRequests: true}})
	if denied.Code != 403 || len(db.VaultFiles) != 0 {
		t.Fatalf("missing document permission accepted: %d", denied.Code)
	}
	w := httptest.NewRecorder()
	a.archiveRequest(w, httptest.NewRequest("POST", "/api/requests/archive", strings.NewReader(body)), user)
	if w.Code != 410 || len(db.VaultFiles) != 0 {
		t.Fatalf("retired archive endpoint accepted a write: %d %s", w.Code, w.Body.String())
	}
	// A copy saved before the feature was retired is still readable and deletable.
	if err := db.ArchiveRequest(1, appdb.VaultMisc, 0, appdb.VaultOther, user.ID, user.Name); err != nil { t.Fatal(err) }
	f := db.VaultFiles[0]
	query := "/api/documents?id=" + strconv.FormatInt(f.ID, 10)
	w = httptest.NewRecorder()
	a.documents(w, httptest.NewRequest("GET", query+"&archive=1", nil), user)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "A message") {
		t.Fatalf("preview failed: %s", w.Body.String())
	}
	w = httptest.NewRecorder()
	a.documents(w, httptest.NewRequest("GET", query+"&entry=report.txt", nil), user)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "A message") || !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("report download failed")
	}
	w = httptest.NewRecorder()
	a.documents(w, httptest.NewRequest("GET", query+"&archive=1", nil), session{ID: 8})
	if w.Code != 403 {
		t.Fatal("preview allowed without document access")
	}
	w = httptest.NewRecorder()
	a.documents(w, httptest.NewRequest("DELETE", query, nil), session{ID: 8, Access: user.Access})
	if w.Code == 200 || len(db.VaultFiles) != 1 {
		t.Fatal("other user removed snapshot")
	}
	w = httptest.NewRecorder()
	a.documents(w, httptest.NewRequest("DELETE", query, nil), user)
	if w.Code != 200 || len(db.VaultFiles) != 0 || len(db.Requests) != 1 {
		t.Fatal("archive delete failed")
	}
}
