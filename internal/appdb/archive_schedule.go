package appdb

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ArchiveSections = []string{"requests", "approvals", "saldo", "payments", "tasks", "appeals", "directory", "meetings", "balances", "compliance", "disputes", "documents", "activity", "audit"}

func Moscow() *time.Location {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return time.FixedZone("MSK", 10800)
	}
	return loc
}
func ArchiveDue(v MonthlyArchiveSettings, now time.Time) bool {
	now = now.In(Moscow())
	if !v.Enabled || v.LastRunMonth == now.Format("2006-01") || v.Day < 1 || v.Day > 31 {
		return false
	}
	t, err := time.Parse("15:04", v.TimeHHMM)
	if err != nil {
		return false
	}
	last := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
	day := v.Day
	if day > last {
		day = last
	}
	due := time.Date(now.Year(), now.Month(), day, t.Hour(), t.Minute(), 0, 0, now.Location())
	return !now.Before(due)
}
func (s *Store) SetArchiveSettings(in MonthlyArchiveSettings, by int64) (MiniAppSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if in.Day < 1 || in.Day > 31 {
		return s.Settings, fmt.Errorf("день: 1–31")
	}
	if _, err := time.Parse("15:04", in.TimeHHMM); err != nil {
		return s.Settings, fmt.Errorf("время: HH:MM по Москве")
	}
	if len(in.Sections) == 0 {
		return s.Settings, fmt.Errorf("выберите разделы")
	}
	for _, sec := range in.Sections {
		ok := false
		for _, allowed := range ArchiveSections {
			if sec == allowed {
				ok = true
			}
		}
		if !ok {
			return s.Settings, fmt.Errorf("неизвестный раздел %s", sec)
		}
	}
	in.Sections = append([]string(nil), in.Sections...)
	in.LastRunMonth = s.Settings.MonthlyArchive.LastRunMonth
	in.UpdatedBy = by
	in.UpdatedAt = time.Now()
	next := s.Settings
	next.MonthlyArchive = in
	return s.saveSettingsLocked(next, by, "archive")
}
func (s *Store) IsSystemArchive(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.VaultFiles {
		if f.ID == id {
			return f.SystemArchive
		}
	}
	return false
}
func (s *Store) MonthlySnapshot(now time.Time, scheduled bool, by int64) (VaultFile, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings := s.Settings.MonthlyArchive
	if scheduled && !ArchiveDue(settings, now) {
		return VaultFile{}, false, nil
	}
	if len(settings.Sections) == 0 {
		return VaultFile{}, false, fmt.Errorf("сначала выберите разделы архива")
	}
	if err := os.MkdirAll(s.FilesDir(), 0700); err != nil {
		return VaultFile{}, false, err
	}
	f, err := os.CreateTemp(s.FilesDir(), "snapshot-*.zip")
	if err != nil {
		return VaultFile{}, false, err
	}
	complete := false
	defer func() {
		f.Close()
		if !complete {
			os.Remove(f.Name())
		}
	}()
	z := zip.NewWriter(f)
	writeJSON := func(name string, v any) error {
		w, e := z.Create(name)
		if e != nil {
			return e
		}
		return json.NewEncoder(w).Encode(v)
	}
	if err = writeJSON("manifest.json", map[string]any{"schema_version": s.SchemaVersion, "saved_at": now, "sections": settings.Sections, "include_files": settings.IncludeFiles}); err != nil {
		return VaultFile{}, false, err
	}
	selected := map[string]bool{}
	for _, sec := range settings.Sections {
		if selected[sec] {
			continue
		}
		selected[sec] = true
		var data any
		switch sec {
		case "requests":
			data = s.Requests
		case "approvals":
			data = s.Approvals
		case "saldo":
			data = map[string]any{"operations": s.SaldoOps, "balances": s.SaldoBalances}
		case "payments":
			data = s.Payments
		case "tasks":
			data = s.Tasks
		case "appeals":
			data = s.Appeals
		case "directory":
			data = map[string]any{"employees": s.Employees, "managers": s.Managers, "clients": s.Clients, "counterparties": s.Counterparties, "positions": s.Positions}
		case "meetings":
			data = s.Meetings
		case "balances":
			data = map[string]any{"companies": s.Companies, "accounts": s.Accounts}
		case "compliance":
			data = s.Compliances
		case "disputes":
			data = s.Disputes
		case "audit":
			data = s.AuditEvents
		case "activity":
			data = s.ActivitySlices
		case "documents":
			files := []VaultFile{}
			for _, v := range s.VaultFiles {
				if !v.SystemArchive {
					files = append(files, v)
				}
			}
			data = map[string]any{"vault": files, "deal_files": s.DealFiles, "tabs": s.VaultTabs}
		default:
			return VaultFile{}, false, fmt.Errorf("неизвестный раздел %s", sec)
		}
		if err = writeJSON(sec+".json", data); err != nil {
			return VaultFile{}, false, err
		}
	}
	if settings.IncludeFiles {
		paths := map[string]bool{}
		if selected["requests"] || selected["documents"] {
			for _, v := range s.DealFiles {
				if v.Rel != "" {
					paths[v.Rel] = true
				}
			}
		}
		if selected["documents"] {
			for _, v := range s.VaultFiles {
				if !v.SystemArchive && v.Rel != "" {
					paths[v.Rel] = true
				}
			}
		}
		for rel := range paths {
			clean := filepath.Clean(rel)
			if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return VaultFile{}, false, fmt.Errorf("небезопасный путь вложения")
			}
			src, e := os.Open(filepath.Join(s.FilesDir(), clean))
			if e != nil {
				return VaultFile{}, false, e
			}
			w, e := z.Create("files/" + filepath.ToSlash(clean))
			if e == nil {
				_, e = io.Copy(w, src)
			}
			src.Close()
			if e != nil {
				return VaultFile{}, false, e
			}
		}
	}
	if err = z.Close(); err != nil {
		return VaultFile{}, false, err
	}
	if err = f.Sync(); err != nil {
		return VaultFile{}, false, err
	}
	info, err := f.Stat()
	if err != nil {
		return VaultFile{}, false, err
	}
	if err = f.Close(); err != nil {
		return VaultFile{}, false, err
	}
	oldID := s.NextID
	id := s.nextLocked()
	v := VaultFile{ID: id, UID: uid("vf", id), Scope: VaultMisc, Folder: VaultOther, Name: "miniapp_" + now.In(Moscow()).Format("2006-01_20060102_150405") + "_MSK.zip", Mime: "application/zip", Size: info.Size(), Rel: filepath.Base(f.Name()), CreatedBy: by, CreatedName: "Автоархив Mini App", CreatedAt: now, SystemArchive: true}
	s.VaultFiles = append(s.VaultFiles, v)
	if scheduled {
		s.Settings.MonthlyArchive.LastRunMonth = now.In(Moscow()).Format("2006-01")
	}
	if err = s.saveLocked(); err != nil {
		s.VaultFiles = s.VaultFiles[:len(s.VaultFiles)-1]
		s.NextID = oldID
		s.Settings.MonthlyArchive = settings
		return VaultFile{}, false, err
	}
	complete = true
	return v, true, nil
}
