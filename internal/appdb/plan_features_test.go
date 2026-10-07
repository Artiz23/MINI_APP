package appdb

import (
	"archive/zip"
	"bytes"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func featureStore(t *testing.T) *Store {
	t.Helper()
	s, err := Load(filepath.Join(t.TempDir(), "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestSaldoCanonicalAndRollback(t *testing.T) {
	s := featureStore(t)
	s.Counterparties = []Counterparty{{ID: 90, Name: "CP"}}
	s.SaldoCPs = []string{"CP"}
	s.SaldoCurrencies = []string{"RUB"}
	for _, amount := range []float64{1000000, 50000} {
		if _, err := s.AddSaldo("CP", "swift", "RUB", "plus", "Test", amount, 7, 0, 90); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.SaldoTotals()["CP|swift|RUB"]; got != 1050000 {
		t.Fatal(got)
	}
	if s.SaldoBalances[0].Amount != "1050000" {
		t.Fatal(s.SaldoBalances)
	}
	if !reflect.DeepEqual(s.rebuildBalancesLocked(), s.SaldoBalances) {
		t.Fatal("journal differs")
	}
	if _, _, err := s.DeleteSaldoCP("CP"); err == nil {
		t.Fatal("deleted financial history")
	}
	if err := s.CatalogDelete("counterparties", 90, 7, true); err == nil {
		t.Fatal("directory deleted financial identity")
	}
	if _, _, err := s.RenameSaldoCP("CP", "Renamed"); err != nil {
		t.Fatal(err)
	}
	if s.SaldoTotals()["Renamed|swift|RUB"] != 1050000 {
		t.Fatal("rename lost balance")
	}
	if _, err := s.CloseSaldoLot(s.SaldoOps[1].ID, 7, "Test"); err != nil {
		t.Fatal(err)
	}
	if s.SaldoTotals()["Renamed|swift|RUB"] != 1000000 {
		t.Fatal("lot close missed canonical balance")
	}
	if _, err := s.AddSaldo("Renamed", "swift", "RUB", "invalid", "Test", 1, 7, 0, 90); err == nil {
		t.Fatal("invalid action")
	}
	if _, err := s.AddSaldo("Renamed", "swift", "RUB", "plus", "Test", 0.1, 7, 0, 90); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddSaldo("Renamed", "swift", "RUB", "plus", "Test", 0.2, 7, 0, 90); err != nil {
		t.Fatal(err)
	}
	if s.SaldoBalances[0].Amount != "1000000.3" {
		t.Fatal(s.SaldoBalances)
	}
	reloaded, err := Load(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.SaldoBalances) != len(s.SaldoBalances) {
		t.Fatal("balances not persisted")
	}
	for i := range s.SaldoBalances {
		want, got := s.SaldoBalances[i], reloaded.SaldoBalances[i]
		if balanceKey(want.CounterpartyID, want.CP, want.Kind, want.Currency) != balanceKey(got.CounterpartyID, got.CP, got.Kind, got.Currency) || want.Amount != got.Amount || want.UpdatedBy != got.UpdatedBy {
			t.Fatalf("balance changed after restart: want %+v, got %+v", want, got)
		}
	}
}
func TestClientCommissionIdempotence(t *testing.T) {
	s := featureStore(t)
	s.Settings.PaymentChatID = -200
	s.Clients = []Client{{ID: 9, CommissionEnabled: true}}
	s.Requests = []Request{{ID: 100, ClientID: 9, CreatedBy: 7, Status: "in_progress", WorkflowStage: StageComplete}}
	if _, err := s.UpdateRequestState(100, "success_closed", "", 7, false); err != nil {
		t.Fatal(err)
	}
	if len(s.Payments) != 1 || s.Payments[0].Kind != "client_commission" || s.Payments[0].ChatID != -200 {
		t.Fatal(s.Payments)
	}
	if _, err := s.UpdateRequestState(100, "reopen", "correction", 7, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateRequestState(100, "success_closed", "", 7, true); err != nil {
		t.Fatal(err)
	}
	if len(s.Payments) != 1 {
		t.Fatal("duplicate commission")
	}
}
func TestMonthlyArchiveScheduleAndSelection(t *testing.T) {
	s := featureStore(t)
	now := time.Date(2026, 2, 28, 23, 0, 0, 0, Moscow())
	v := MonthlyArchiveSettings{Enabled: true, Day: 31, TimeHHMM: "23:00", Sections: []string{"requests", "meetings"}}
	if ArchiveDue(v, now.Add(-time.Minute)) || !ArchiveDue(v, now) {
		t.Fatal("short month schedule")
	}
	if _, err := s.SetArchiveSettings(v, 7); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := s.MonthlySnapshot(now, true, 7); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(s.VaultFiles) != 1 || s.Settings.MonthlyArchive.LastRunMonth != "2026-02" {
		t.Fatal("duplicate scheduled archive")
	}
	f, data, err := s.VaultFileBytes(s.VaultFiles[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !f.SystemArchive {
		t.Fatal("unrestricted snapshot")
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range z.File {
		names[f.Name] = true
	}
	if len(names) != 3 || !names["manifest.json"] || !names["requests.json"] || !names["meetings.json"] {
		t.Fatal(names)
	}
	if _, created, err := s.MonthlySnapshot(now, false, 7); err != nil || !created {
		t.Fatal("manual archive", err)
	}
	if len(s.VaultFilesCopy(VaultMisc, 0, VaultOther, 8)) != 0 {
		t.Fatal("snapshot leaked to ordinary user")
	}
}
func TestMeetingsAndSettings(t *testing.T) {
	s := featureStore(t)
	m, err := s.SaveMeeting(Meeting{Title: "Call", URL: "https://example.com", StartsAt: time.Now().Add(time.Hour)}, 7, "Test", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveMeeting(m, 8, "Other", false); err == nil {
		t.Fatal("foreign meeting edit")
	}
	if _, err := s.SaveMeeting(Meeting{Title: "Bad", URL: "javascript:alert(1)", StartsAt: time.Now()}, 7, "Test", false); err == nil {
		t.Fatal("unsafe URL")
	}
	if _, err := s.SetChat("approval", -999, 7); err == nil {
		t.Fatal("unregistered chat accepted")
	}
	approvalChat, err := s.AddManagedChat("Approval", -100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddManagedChat("Payment", -200); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetChat("approval", -100, 7); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetChat("payment", -100, 7); err == nil {
		t.Fatal("same chats")
	}
	if _, err := s.SetChat("payment", -200, 7); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateManagedChat(approvalChat.ID, "Approval", -101); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Meetings) != 1 || reloaded.Settings.ApprovalChatID != -101 || reloaded.Settings.PaymentChatID != -200 {
		t.Fatal("settings not persisted")
	}
}
func TestActivityBucketsAndExactAnalytics(t *testing.T) {
	s := featureStore(t)
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, Moscow())
	s.Employees = []Employee{{ID: 1, TelegramID: 7, Name: "Test"}}
	for i := 0; i < 5; i++ {
		if err := s.RecordActivity(7, now.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.ActivitySlices) != 1 {
		t.Fatal("duplicate heartbeat")
	}
	s.Requests = []Request{{ID: 1, EmployeeID: 1, CreatedAt: now, Status: "success_closed", Economics: RequestEconomics{ClientSale: SaleTerm{Value: "0.1", Currency: "RUB"}, Profit: MoneyTerm{Value: "100.1", Currency: "RUB"}}}, {ID: 2, EmployeeID: 1, CreatedAt: now, Status: "success_closed", Economics: RequestEconomics{ClientSale: SaleTerm{Value: "0.2", Currency: "RUB"}, Profit: MoneyTerm{Value: "-0.1", Currency: "RUB"}}}}
	rows, err := s.Analytics("employee", 0, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Sales["RUB"] != "0.3" || rows[0].Profit["RUB"] != "100" || rows[0].ActivityHours != 5.0/60 || rows[0].Conversion != 100 {
		t.Fatal(rows)
	}
}
