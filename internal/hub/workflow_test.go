package hub

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"tg-bot-orh3/MINI_APP/internal/appdb"
	"tg-bot-orh3/MINI_APP/internal/secrets"
	"time"
)

func signedTestUser(id int64) string {
	user := fmt.Sprintf(`{"id":%d,"first_name":"Test"}`, id)
	date := strconv.FormatInt(time.Now().Unix(), 10)
	data := "auth_date=" + date + "\nuser=" + user
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(secrets.BotToken))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(data))
	return url.Values{"user": {user}, "auth_date": {date}, "hash": {hex.EncodeToString(mac.Sum(nil))}}.Encode()
}
func TestForeignRequestMutationsHTTP(t *testing.T) {
	db, err := appdb.Load(filepath.Join(t.TempDir(), "miniapp.json"))
	if err != nil {
		t.Fatal(err)
	}
	db.Users = []appdb.User{{ID: 80001, Role: appdb.RoleOperator}}
	db.Employees = []appdb.Employee{{ID: 1, TelegramID: 80001, PositionIDs: []int64{9}}, {ID: 2, TelegramID: 80002}}
	db.Positions = []appdb.Position{{ID: 9, Access: appdb.AllAccess()}}
	db.Requests = []appdb.Request{{ID: 100, UID: "FOREIGN", EmployeeID: 2, CreatedBy: 80002, Status: "in_progress", WorkflowStage: appdb.StageApproval}, {ID: 101, UID: "OWN", EmployeeID: 1, Status: "in_progress", WorkflowStage: appdb.StageApproval}}
	db.Payments = []appdb.Payment{{ID: 200, RequestID: 100}}
	db.Tasks = []appdb.Task{{ID: 201, RequestID: 100}}
	db.DealFiles = []appdb.DealFile{{ID: 202, RequestID: 100}}
	a := &API{db: db}
	handler := a.Handler()
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "requests", `{"id":100,"status":"failed","close_reason":"test"}`},
		{"POST", "requests", `{"id":100,"action":"edit_comment","comment":"changed"}`},
		{"DELETE", "requests?id=100", ``},
		{"POST", "approvals", `{"request_id":100}`}, {"POST", "payments", `{"request_id":100,"text":"pay"}`},
		{"POST", "payments", `{"id":200,"status":"sent"}`}, {"POST", "tasks", `{"id":201,"request_id":101,"title":"relink"}`},
		{"POST", "tasks", `{"request_id":100,"title":"task"}`}, {"POST", "appeals", `{"request_id":100,"text":"appeal"}`},
		{"POST", "disputes", `{"section":"requests","ref_uid":"FOREIGN","text":"dispute"}`},
		{"DELETE", "deal-files?id=202", ``}, {"POST", "attach", `{"request_id":100}`}, {"POST", "compliance", `{"request_id":100}`},
	} {
		t.Run(tc.method+tc.path+tc.body, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "/api/"+tc.path, strings.NewReader(tc.body))
			r.Header.Set("X-Telegram-Init-Data", signedTestUser(80001))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("want 403 got %d: %s", w.Code, w.Body.String())
			}
		})
	}
	for _, path := range []string{"payments", "tasks", "appeals"} {
		r := httptest.NewRequest("POST", "/api/"+path, strings.NewReader(`{"request_id":101,"text":"test","title":"test"}`))
		r.Header.Set("X-Telegram-Init-Data", signedTestUser(80001))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 409 {
			t.Fatalf("workflow %s: %d %s", path, w.Code, w.Body.String())
		}
	}
	for _, scope := range []string{"mine", "all"} {
		r := httptest.NewRequest("GET", "/api/requests?scope="+scope, nil)
		r.Header.Set("X-Telegram-Init-Data", signedTestUser(80001))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		var rows []appdb.RequestView
		if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
			t.Fatal(err)
		}
		want := 1
		if scope == "all" {
			want = 2
		}
		if len(rows) != want {
			t.Fatalf("scope %s: %+v", scope, rows)
		}
		for _, v := range rows {
			if v.ID == 100 && v.CanEdit {
				t.Fatal("foreign editable")
			}
		}
	}
}

type fakeDeliverySender struct {
	calls           int
	failAt          int
	chats           []int64
	approvalText    string
	approvalButtons *tgbotapi.InlineKeyboardMarkup
}

func (f *fakeDeliverySender) send(chat int64) (int, error) {
	f.calls++
	f.chats = append(f.chats, chat)
	if f.calls == f.failAt {
		return 0, fmt.Errorf("fake failure")
	}
	return f.calls, nil
}
func (f *fakeDeliverySender) SendToGroup(chat int64, _ string) (int, error) { return f.send(chat) }
func (f *fakeDeliverySender) SendToGroupMarkup(chat int64, text string, buttons *tgbotapi.InlineKeyboardMarkup) (int, error) {
	f.approvalText = text
	f.approvalButtons = buttons
	return f.send(chat)
}
func (f *fakeDeliverySender) SendBytesToGroup(chat int64, _ string, _ []byte, _ string) (int, error) {
	return f.send(chat)
}
func TestDeliveryRetryKeepsBusinessRecord(t *testing.T) {
	db, err := appdb.Load(filepath.Join(t.TempDir(), "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	db.Settings.ApprovalChatID = -100
	db.Settings.PaymentChatID = -200
	db.Requests = []appdb.Request{{ID: 99, CreatedBy: 7, Status: "in_progress", WorkflowStage: appdb.StageApproval}}
	rec, err := db.CreateRequestApproval(99, 7, "Test", false)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeDeliverySender{failAt: 1}
	a := &API{db: db, sender: fake}
	if a.deliver("approval", rec.ID) == nil {
		t.Fatal("expected failure")
	}
	d, _ := db.DeliveryRecord("approval", rec.ID)
	if d.DeliveryStatus != "error" || len(d.DeliveryParts) != 0 {
		t.Fatalf("partial: %+v", d)
	}
	if err := a.deliver("approval", rec.ID); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 2 || len(db.Approvals) != 1 {
		t.Fatal("retry resent completed part or duplicated record")
	}
	if !strings.Contains(fake.approvalText, "Сальдо:") || !strings.Contains(fake.approvalText, "🆕 Заявка") || fake.approvalButtons == nil || len(fake.approvalButtons.InlineKeyboard[0]) != 3 {
		t.Fatal("approval must contain request, saldo and three inline buttons")
	}
	if err := a.deliver("approval", rec.ID); err != nil || fake.calls != 2 {
		t.Fatal("sent delivery repeated")
	}
	for _, chat := range fake.chats {
		if chat != -100 {
			t.Fatal("wrong chat")
		}
	}
	p, err := db.AddPayment(99, "pay", "Test", 7)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.deliver("payment", p.ID); err != nil {
		t.Fatal(err)
	}
	if fake.chats[len(fake.chats)-1] != -200 {
		t.Fatal("payment used approval chat")
	}
	if db.Payments[0].Status != "pending" {
		t.Fatal("Telegram delivery marked money paid")
	}
}

func TestSaldoFilesUseConfiguredChat(t *testing.T) {
	db, err := appdb.Load(filepath.Join(t.TempDir(), "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	db.Settings.SaldoChatID = -300
	fake := &fakeDeliverySender{}
	a := &API{db: db, sender: fake}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/saldo", strings.NewReader(`{"send_files":true}`))
	a.saldo(w, r, session{ID: 7, Access: appdb.Access{appdb.SecSaldo: true}})
	if w.Code != 200 || fake.calls != 3 {
		t.Fatalf("status=%d calls=%d body=%s", w.Code, fake.calls, w.Body.String())
	}
	for _, chat := range fake.chats {
		if chat != -300 {
			t.Fatalf("wrong chat %d", chat)
		}
	}
}
