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

type VaultDestination struct {
	Scope   string `json:"scope"`
	OwnerID int64  `json:"owner_id"`
	Folder  string `json:"folder"`
	Label   string `json:"label"`
}

func (s *Store) VaultDestinations() []VaultDestination {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []VaultDestination{}
	add := func(scope string, owner int64, folder string) {
		out = append(out, VaultDestination{scope, owner, folder, s.vaultLabelLocked(scope, owner, folder)})
	}
	for _, c := range s.Companies {
		for _, contour := range []string{VaultInternal, VaultExternal, VaultOther} {
			add(VaultCompany, c.ID, contour)
			for _, tab := range s.VaultTabs {
				if tab.CompanyID == c.ID && tab.Contour == contour {
					add(VaultCompany, c.ID, tab.FolderKey())
				}
			}
		}
	}
	for _, c := range s.Counterparties {
		add(VaultCounterparty, c.ID, VaultOther)
	}
	add(VaultMisc, 0, VaultOther)
	return out
}

type ArchiveEntry struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Mime string `json:"mime"`
	Size int64  `json:"size"`
}

type RequestSnapshot struct {
	Version     int            `json:"version"`
	SavedAt     time.Time      `json:"saved_at"`
	SavedBy     string         `json:"saved_by"`
	Request     RequestBundle  `json:"request"`
	Tasks       []Task         `json:"tasks"`
	Compliances []Compliance   `json:"compliances"`
	Disputes    []Dispute      `json:"disputes"`
	Entries     []ArchiveEntry `json:"entries"`
}

func (s *Store) archiveDestinationLocked(scope string, owner int64, folder string) error {
	if scope != VaultCompany && scope != VaultCounterparty && scope != VaultMisc {
		return fmt.Errorf("выберите раздел документов")
	}
	if err := s.vaultPlaceOKLocked(scope, owner); err != nil {
		return err
	}
	if scope != VaultCompany {
		if folder != VaultOther || (scope == VaultMisc && owner != 0) {
			return fmt.Errorf("выберите существующую папку")
		}
		return nil
	}
	contour, tabID, ok := ParseVaultContour(folder)
	if !ok {
		return fmt.Errorf("выберите существующую вкладку")
	}
	if tabID == 0 {
		return nil
	}
	for _, t := range s.VaultTabs {
		if t.ID == tabID && t.CompanyID == owner && t.Contour == contour {
			return nil
		}
	}
	return fmt.Errorf("вкладка не найдена; выберите другую")
}

// ArchiveRequest saves an independent snapshot. The source request stays intact.
// The ZIP is completed before publishing its single record in miniapp.json.
func (s *Store) ArchiveRequest(reqID int64, scope string, ownerID int64, folder string, by int64, byName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.archiveDestinationLocked(scope, ownerID, folder); err != nil {
		return err
	}
	for _, f := range s.VaultFiles {
		if f.RequestID == reqID {
			return fmt.Errorf("заявка уже сохранена в документы")
		}
	}
	b, err := s.requestBundleLocked(reqID)
	if err != nil {
		return err
	}
	x := RequestSnapshot{Version: 1, SavedAt: time.Now(), SavedBy: byName, Request: b,
		Tasks: []Task{}, Compliances: []Compliance{}, Disputes: []Dispute{}, Entries: []ArchiveEntry{}}
	refs := map[string]bool{}
	addRef := func(uid string) {
		if uid != "" {
			refs[uid] = true
		}
	}
	addRef(b.UID)
	for _, p := range b.Payments {
		addRef(p.UID)
	}
	for _, p := range b.Approvals {
		addRef(p.UID)
	}
	for _, p := range b.Saldo {
		addRef(p.UID)
	}
	for _, p := range b.Appeals {
		addRef(p.UID)
	}
	for _, t := range s.Tasks {
		if t.RequestID == reqID {
			x.Tasks = append(x.Tasks, t)
			addRef(t.UID)
		}
	}
	for _, c := range s.Compliances {
		if c.RequestID == reqID || (c.RequestUID != "" && c.RequestUID == b.UID) {
			x.Compliances = append(x.Compliances, c)
			addRef(c.UID)
		}
	}
	for _, d := range s.Disputes {
		if refs[d.RefUID] {
			x.Disputes = append(x.Disputes, d)
		}
	}
	if err := os.MkdirAll(s.FilesDir(), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(s.FilesDir(), "request-*.zip")
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = os.Remove(file.Name())
		}
	}()
	zw := zip.NewWriter(file)
	write := func(name string, data []byte) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	for _, f := range b.Files {
		if f.Kind != "file" {
			continue
		}
		if f.Rel == "" {
			return fmt.Errorf("нет содержимого файла %s", f.Name)
		}
		entry := fmt.Sprintf("files/%d_%s", f.ID, sanitizeFileName(f.Name))
		src, err := os.Open(filepath.Join(s.FilesDir(), f.Rel))
		if err != nil {
			return fmt.Errorf("не удалось прочитать %s: %w", f.Name, err)
		}
		w, err := zw.Create(entry)
		if err != nil {
			_ = src.Close()
			return err
		}
		n, err := io.Copy(w, src)
		_ = src.Close()
		if err != nil {
			return fmt.Errorf("не удалось скопировать %s: %w", f.Name, err)
		}
		x.Entries = append(x.Entries, ArchiveEntry{entry, f.Name, f.Mime, n})
	}
	data, err := json.MarshalIndent(x, "", "  ")
	if err != nil {
		return err
	}
	if err := write("request.json", data); err != nil {
		return err
	}
	if err := write("report.txt", []byte(x.Report())); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	info, err := os.Stat(file.Name())
	if err != nil {
		return err
	}
	oldID := s.NextID
	id := s.nextLocked()
	f := VaultFile{ID: id, UID: uid("vf", id), Scope: scope, OwnerID: ownerID, Folder: normVaultFolder(scope, folder),
		Name: sanitizeFileName("Заявка_"+b.UID) + ".zip", Mime: "application/zip", Size: info.Size(), Rel: filepath.Base(file.Name()),
		CreatedBy: by, CreatedName: byName, CreatedAt: x.SavedAt, RequestID: reqID, RequestUID: b.UID, RequestTitle: b.Title}
	s.VaultFiles = append(s.VaultFiles, f)
	if err := s.saveLocked(); err != nil {
		s.VaultFiles = s.VaultFiles[:len(s.VaultFiles)-1]
		s.NextID = oldID
		return err
	}
	committed = true
	return nil
}

func (x RequestSnapshot) Report() string {
	var out strings.Builder
	r := x.Request
	fmt.Fprintf(&out, "ЗАЯВКА %s — %s\nСтатус: %s\nСоздана: %s\nСохранил: %s · %s\n\nСотрудник: %s (ID %d)\nМенеджер: %s (ID %d)\nКлиент: %s (ID %d)\nКонтрагент: %s (ID %d)\nТема Telegram: %s\nТаблица: %s\nЗаметки: %s\n",
		r.UID, r.Title, r.Status, r.CreatedAt.Format(time.RFC3339), x.SavedBy, x.SavedAt.Format(time.RFC3339),
		r.EmployeeName, r.EmployeeID, r.ManagerName, r.ManagerID, r.ClientName, r.ClientID, r.CounterpartyName, r.CounterpartyID, r.ThreadLink, r.TableRef, r.Notes)
	fmt.Fprintln(&out, "\nФАЙЛЫ И СООБЩЕНИЯ")
	for _, f := range r.Files {
		fmt.Fprintf(&out, "\n%s · %s · %s\n%s\n", f.CreatedAt.Format(time.RFC3339), f.CreatedName, f.Name, f.Text)
	}
	fmt.Fprintln(&out, "\nОПЛАТЫ")
	for _, p := range r.Payments {
		fmt.Fprintf(&out, "\n%s · %s · %s\n%s\nОтправлено: %s · %s\n", p.UID, p.Status, p.CreatedName, p.Text, p.SentName, p.SentAt.Format(time.RFC3339))
	}
	fmt.Fprintln(&out, "\nСОГЛАСОВАНИЯ")
	for _, a := range r.Approvals {
		fmt.Fprintf(&out, "\n%s · %s · %s\n%s\nРешение: %s · %s\n", a.UID, a.Status, a.ManagerName, a.Preview, a.DecidedBy, a.DecidedAt.Format(time.RFC3339))
	}
	fmt.Fprintln(&out, "\nСАЛЬДО")
	for _, p := range r.Saldo {
		fmt.Fprintf(&out, "\n%s · %s · %s · %s · %g %s · %s · %s\n", p.UID, p.CP, p.Kind, p.Action, p.Amount, p.Currency, p.Manager, p.CreatedAt.Format(time.RFC3339))
	}
	fmt.Fprintln(&out, "\nЗАДАЧИ")
	for _, t := range x.Tasks {
		fmt.Fprintf(&out, "\n%s · %s · %s · %s\n%s\n", t.UID, t.Title, t.Status, t.CreatedName, t.Text)
	}
	fmt.Fprintln(&out, "\nОБРАЩЕНИЯ")
	for _, a := range r.Appeals {
		fmt.Fprintf(&out, "\n%s · %s · %s · %s\n%s\nИсполнитель: %s\n", a.UID, a.Kind, a.Status, a.CreatedName, a.Text, a.TakenName)
	}
	fmt.Fprintln(&out, "\nКОМПЛАЕНС")
	for _, c := range x.Compliances {
		fmt.Fprintf(&out, "\n%s · %s · %s\n%s\n", c.UID, c.Subject, c.Status, c.Text)
		for _, q := range c.Questions {
			fmt.Fprintf(&out, "%s: %s\n", q.Text, q.Answer)
		}
	}
	fmt.Fprintln(&out, "\nСПОРЫ И ПЕРЕПИСКА")
	for _, d := range x.Disputes {
		fmt.Fprintf(&out, "\n%s · %s · %s\n%s\n", d.UID, d.Status, d.CreatedName, d.Text)
		for _, m := range d.Messages {
			fmt.Fprintf(&out, "%s · %s\n%s\n", m.At.Format(time.RFC3339), m.ByName, m.Text)
		}
	}
	return out.String()
}

func (s *Store) openRequestArchive(id int64) (VaultFile, *zip.ReadCloser, RequestSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x RequestSnapshot
	for _, f := range s.VaultFiles {
		if f.ID != id || f.RequestID == 0 {
			continue
		}
		z, err := zip.OpenReader(filepath.Join(s.FilesDir(), f.Rel))
		if err != nil {
			return f, nil, x, err
		}
		for _, entry := range z.File {
			if entry.Name != "request.json" {
				continue
			}
			r, err := entry.Open()
			if err == nil {
				err = json.NewDecoder(r).Decode(&x)
				_ = r.Close()
			}
			if err != nil {
				_ = z.Close()
				return f, nil, x, err
			}
			return f, z, x, nil
		}
		_ = z.Close()
		return f, nil, x, fmt.Errorf("содержимое заявки не найдено")
	}
	return VaultFile{}, nil, x, fmt.Errorf("заявка не сохранена в документы")
}

func (s *Store) RequestArchivePreview(id int64) (VaultFile, RequestSnapshot, error) {
	f, z, x, err := s.openRequestArchive(id)
	if z != nil {
		_ = z.Close()
	}
	return f, x, err
}

func (s *Store) RequestArchiveEntry(id int64, path string) (ArchiveEntry, []byte, error) {
	_, z, x, err := s.openRequestArchive(id)
	if err != nil {
		return ArchiveEntry{}, nil, err
	}
	defer z.Close()
	allowed := append(x.Entries, ArchiveEntry{"report.txt", "Заявка.txt", "text/plain; charset=utf-8", 0})
	for _, meta := range allowed {
		if meta.Path != path {
			continue
		}
		for _, f := range z.File {
			if f.Name != path {
				continue
			}
			r, err := f.Open()
			if err != nil {
				return meta, nil, err
			}
			data, err := io.ReadAll(r)
			_ = r.Close()
			return meta, data, err
		}
	}
	return ArchiveEntry{}, nil, fmt.Errorf("вложение не найдено")
}
