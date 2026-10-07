package appdb

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	VaultCompany      = "company"
	VaultCounterparty = "counterparty"
	VaultMisc         = "misc"
	VaultInternal     = "internal"
	VaultExternal     = "external"
	VaultOther        = "other"
)

type VaultFile struct {
	SystemArchive bool      `json:"system_archive,omitempty"`
	RequestID     int64     `json:"request_id,omitempty"`
	RequestUID    string    `json:"request_uid,omitempty"`
	RequestTitle  string    `json:"request_title,omitempty"`
	ID            int64     `json:"id"`
	UID           string    `json:"uid"`
	Scope         string    `json:"scope"`
	OwnerID       int64     `json:"owner_id,omitempty"`
	Folder        string    `json:"folder"`
	Name          string    `json:"name"`
	Mime          string    `json:"mime,omitempty"`
	Size          int64     `json:"size,omitempty"`
	Rel           string    `json:"rel,omitempty"`
	CreatedBy     int64     `json:"created_by"`
	CreatedName   string    `json:"created_name"`
	CreatedAt     time.Time `json:"created_at"`
	Mine          bool      `json:"mine,omitempty"`
}

// CompanyVaultTab — доп. вкладка внутри внутреннего или внешнего контура компании.
type CompanyVaultTab struct {
	ID        int64     `json:"id"`
	CompanyID int64     `json:"company_id"`
	Contour   string    `json:"contour"` // internal | external
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func (t CompanyVaultTab) FolderKey() string {
	return t.Contour + ":" + fmt.Sprintf("%d", t.ID)
}

func normVaultScope(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case VaultCompany:
		return VaultCompany
	case VaultCounterparty:
		return VaultCounterparty
	default:
		return VaultMisc
	}
}

func ParseVaultContour(folder string) (contour string, tabID int64, ok bool) {
	folder = strings.ToLower(strings.TrimSpace(folder))
	switch folder {
	case VaultInternal, VaultExternal, VaultOther:
		return folder, 0, true
	case "":
		return VaultInternal, 0, true
	}
	part, rest, cut := strings.Cut(folder, ":")
	if !cut {
		return "", 0, false
	}
	if part != VaultInternal && part != VaultExternal {
		return "", 0, false
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	if err != nil || id == 0 {
		return "", 0, false
	}
	return part, id, true
}

func normVaultFolder(scope, folder string) string {
	folder = strings.TrimSpace(folder)
	if scope != VaultCompany {
		return VaultOther
	}
	contour, tabID, ok := ParseVaultContour(folder)
	if !ok {
		return VaultInternal
	}
	if tabID != 0 {
		return contour + ":" + strconv.FormatInt(tabID, 10)
	}
	return contour
}

func (s *Store) companyNameTakenLocked(name string, except int64) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	for _, c := range s.Companies {
		if c.ID == except {
			continue
		}
		if strings.ToLower(strings.TrimSpace(c.Name)) == name {
			return true
		}
	}
	return false
}

func (s *Store) AddCompany(name string) (Company, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" {
		return Company{}, fmt.Errorf("введите название компании")
	}
	if s.companyNameTakenLocked(name, 0) {
		return Company{}, fmt.Errorf("такая компания уже есть")
	}
	id := s.nextLocked()
	c := Company{ID: id, UID: uid("cmp", id), Name: name}
	s.Companies = append(s.Companies, c)
	return c, s.saveLocked()
}

func (s *Store) RenameCompany(id int64, name string) (Company, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" {
		return Company{}, fmt.Errorf("введите название компании")
	}
	if s.companyNameTakenLocked(name, id) {
		return Company{}, fmt.Errorf("такая компания уже есть")
	}
	for i := range s.Companies {
		if s.Companies[i].ID != id {
			continue
		}
		s.Companies[i].Name = name
		return s.Companies[i], s.saveLocked()
	}
	return Company{}, fmt.Errorf("компания не найдена")
}

func (s *Store) DeleteCompany(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.Companies {
		if s.Companies[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("компания не найдена")
	}
	for _, a := range s.Accounts {
		if a.CompanyID == id {
			return fmt.Errorf("у компании есть счета в Балансе — сначала уберите их")
		}
	}
	kept := []VaultFile{}
	for _, f := range s.VaultFiles {

		if f.Scope == VaultCompany && f.OwnerID == id {
			if f.Rel != "" {
				_ = os.Remove(filepath.Join(s.FilesDir(), f.Rel))
			}
			continue
		}
		kept = append(kept, f)
	}
	s.VaultFiles = kept
	tabs := []CompanyVaultTab{}
	for _, t := range s.VaultTabs {
		if t.CompanyID == id {
			continue
		}
		tabs = append(tabs, t)
	}
	s.VaultTabs = tabs
	s.Companies = append(s.Companies[:idx], s.Companies[idx+1:]...)
	return s.saveLocked()
}

func (s *Store) VaultFilesCopy(scope string, ownerID int64, folder string, viewer int64) []VaultFile {
	s.mu.Lock()
	defer s.mu.Unlock()
	scope = normVaultScope(scope)
	folder = normVaultFolder(scope, folder)
	out := []VaultFile{}
	for _, f := range s.VaultFiles {
		if f.SystemArchive {
			u, _ := s.userLocked(viewer)
			if u.Role != RoleAdmin && u.Role != RoleOwner {
				continue
			}
		}
		if f.Scope != scope {
			continue
		}
		if scope != VaultMisc && f.OwnerID != ownerID {
			continue
		}
		if f.Folder != folder {
			continue
		}
		f.Mine = f.CreatedBy == viewer
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (s *Store) AddVaultFile(scope string, ownerID int64, folder, name, mime string, data []byte, byName string, by int64) (VaultFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addVaultFileLocked(scope, ownerID, folder, name, mime, data, byName, by)
}

// addVaultFileLocked requires s.mu to be held by the caller.
func (s *Store) addVaultFileLocked(scope string, ownerID int64, folder, name, mime string, data []byte, byName string, by int64) (VaultFile, error) {
	if len(data) == 0 {
		return VaultFile{}, fmt.Errorf("пустой файл")
	}
	scope = normVaultScope(scope)
	folder = normVaultFolder(scope, folder)
	if scope == VaultCompany {
		ok := false
		for _, c := range s.Companies {
			if c.ID == ownerID {
				ok = true
				break
			}
		}
		if !ok {
			return VaultFile{}, fmt.Errorf("компания не найдена")
		}
		if _, tabID, ok := ParseVaultContour(folder); ok && tabID != 0 {
			found := false
			for _, t := range s.VaultTabs {
				if t.ID == tabID && t.CompanyID == ownerID {
					found = true
					break
				}
			}
			if !found {
				return VaultFile{}, fmt.Errorf("вкладка не найдена")
			}
		}
	}
	if scope == VaultCounterparty {
		ok := false
		for _, c := range s.Counterparties {
			if c.ID == ownerID {
				ok = true
				break
			}
		}
		if !ok {
			return VaultFile{}, fmt.Errorf("контрагент не найден")
		}
	}
	if scope == VaultMisc {
		ownerID = 0
	}
	id := s.nextLocked()
	safe := sanitizeFileName(name)
	rel := fmt.Sprintf("vault_%d_%s", id, safe)
	dir := s.FilesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return VaultFile{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, rel), data, 0o600); err != nil {
		return VaultFile{}, err
	}
	f := VaultFile{
		ID: id, UID: uid("vf", id), Scope: scope, OwnerID: ownerID, Folder: folder,
		Name: name, Mime: mime, Size: int64(len(data)), Rel: rel,
		CreatedBy: by, CreatedName: byName, CreatedAt: time.Now(), Mine: true,
	}
	s.VaultFiles = append(s.VaultFiles, f)
	return f, s.saveLocked()
}

func (s *Store) VaultFileBytes(id int64) (VaultFile, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.VaultFiles {
		if f.ID != id {
			continue
		}
		if f.Rel == "" {
			return f, nil, fmt.Errorf("это не файл")
		}
		data, err := os.ReadFile(filepath.Join(s.FilesDir(), f.Rel))
		return f, data, err
	}
	return VaultFile{}, nil, fmt.Errorf("файл не найден")
}

func (s *Store) vaultPlaceOKLocked(scope string, ownerID int64) error {
	switch scope {
	case VaultCompany:
		for _, c := range s.Companies {
			if c.ID == ownerID {
				return nil
			}
		}
		return fmt.Errorf("компания не найдена")
	case VaultCounterparty:
		for _, c := range s.Counterparties {
			if c.ID == ownerID {
				return nil
			}
		}
		return fmt.Errorf("контрагент не найден")
	default:
		return nil
	}
}

func (s *Store) vaultLabelLocked(scope string, ownerID int64, folder string) string {
	switch scope {
	case VaultCompany:
		name := "компания"
		for _, c := range s.Companies {
			if c.ID == ownerID {
				name = c.Name
				break
			}
		}
		fold := "внутренний контур"
		contour, tabID, ok := ParseVaultContour(folder)
		if ok {
			switch contour {
			case VaultExternal:
				fold = "внешний контур"
			case VaultOther:
				fold = "прочее"
			default:
				fold = "внутренний контур"
			}
			if tabID != 0 {
				tabName := "вкладка"
				for _, t := range s.VaultTabs {
					if t.ID == tabID {
						tabName = t.Name
						break
					}
				}
				fold = fold + " · " + tabName
			}
		}
		return name + " · " + fold
	case VaultCounterparty:
		name := "контрагент"
		for _, c := range s.Counterparties {
			if c.ID == ownerID {
				name = c.Name
				break
			}
		}
		return name
	default:
		return "Прочее"
	}
}

func (s *Store) VaultPlaceLabel(scope string, ownerID int64, folder string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	scope = normVaultScope(scope)
	folder = normVaultFolder(scope, folder)
	return s.vaultLabelLocked(scope, ownerID, folder)
}

func (s *Store) DeleteOwnVault(scope string, ownerID int64, folder string, userID int64) (int, error) {
	if userID == 0 {
		return 0, fmt.Errorf("нет пользователя")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.deleteOwnVaultLocked(scope, ownerID, folder, userID)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, fmt.Errorf("нет ваших файлов здесь")
	}
	return n, s.saveLocked()
}

func (s *Store) deleteOwnVaultLocked(scope string, ownerID int64, folder string, userID int64) (int, error) {
	scope = normVaultScope(scope)
	folder = normVaultFolder(scope, folder)
	kept := []VaultFile{}
	n := 0
	for _, f := range s.VaultFiles {
		match := f.Scope == scope && f.Folder == folder && f.CreatedBy == userID
		if scope != VaultMisc {
			match = match && f.OwnerID == ownerID
		}
		if match {
			n++
			if f.Rel != "" {
				_ = os.Remove(filepath.Join(s.FilesDir(), f.Rel))
			}
			continue
		}
		kept = append(kept, f)
	}
	s.VaultFiles = kept
	return n, nil
}

func (s *Store) DeleteVaultFile(id, by int64, staff bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.VaultFiles {
		if s.VaultFiles[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("файл не найден")
	}
	if !staff && s.VaultFiles[idx].CreatedBy != by {
		return fmt.Errorf("удалить может только тот, кто загрузил")
	}
	rel := s.VaultFiles[idx].Rel
	oldFiles := s.VaultFiles
	kept := append([]VaultFile{}, oldFiles[:idx]...)
	s.VaultFiles = append(kept, oldFiles[idx+1:]...)
	if err := s.saveLocked(); err != nil {
		s.VaultFiles = oldFiles
		return err
	}
	if rel != "" {
		_ = os.Remove(filepath.Join(s.FilesDir(), rel))
	}
	return nil
}

func (s *Store) VaultTabsCopy(companyID int64, contour string) []CompanyVaultTab {
	s.mu.Lock()
	defer s.mu.Unlock()
	contour = strings.ToLower(strings.TrimSpace(contour))
	out := []CompanyVaultTab{}
	for _, t := range s.VaultTabs {
		if t.CompanyID != companyID {
			continue
		}
		if contour != "" && t.Contour != contour {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func (s *Store) AddCompanyVaultTab(companyID int64, contour, name string) (CompanyVaultTab, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" {
		return CompanyVaultTab{}, fmt.Errorf("введите название вкладки")
	}
	contour = strings.ToLower(strings.TrimSpace(contour))
	if contour != VaultInternal && contour != VaultExternal {
		return CompanyVaultTab{}, fmt.Errorf("вкладки только во внутреннем или внешнем контуре")
	}
	ok := false
	for _, c := range s.Companies {
		if c.ID == companyID {
			ok = true
			break
		}
	}
	if !ok {
		return CompanyVaultTab{}, fmt.Errorf("компания не найдена")
	}
	low := strings.ToLower(name)
	for _, t := range s.VaultTabs {
		if t.CompanyID == companyID && t.Contour == contour && strings.ToLower(t.Name) == low {
			return CompanyVaultTab{}, fmt.Errorf("такая вкладка уже есть")
		}
	}
	id := s.nextLocked()
	t := CompanyVaultTab{ID: id, CompanyID: companyID, Contour: contour, Name: name, CreatedAt: time.Now()}
	s.VaultTabs = append(s.VaultTabs, t)
	return t, s.saveLocked()
}

func (s *Store) RenameCompanyVaultTab(id int64, name string) (CompanyVaultTab, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" {
		return CompanyVaultTab{}, fmt.Errorf("введите название вкладки")
	}
	low := strings.ToLower(name)
	for i := range s.VaultTabs {
		if s.VaultTabs[i].ID != id {
			continue
		}
		for _, t := range s.VaultTabs {
			if t.ID == id {
				continue
			}
			if t.CompanyID == s.VaultTabs[i].CompanyID && t.Contour == s.VaultTabs[i].Contour && strings.ToLower(t.Name) == low {
				return CompanyVaultTab{}, fmt.Errorf("такая вкладка уже есть")
			}
		}
		s.VaultTabs[i].Name = name
		return s.VaultTabs[i], s.saveLocked()
	}
	return CompanyVaultTab{}, fmt.Errorf("вкладка не найдена")
}

func (s *Store) DeleteCompanyVaultTab(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	var gone CompanyVaultTab
	for i := range s.VaultTabs {
		if s.VaultTabs[i].ID == id {
			idx = i
			gone = s.VaultTabs[i]
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("вкладка не найдена")
	}
	key := gone.FolderKey()
	kept := []VaultFile{}
	for _, f := range s.VaultFiles {
		if f.Scope == VaultCompany && f.OwnerID == gone.CompanyID && f.Folder == key {
			if f.Rel != "" {
				_ = os.Remove(filepath.Join(s.FilesDir(), f.Rel))
			}
			continue
		}
		kept = append(kept, f)
	}
	s.VaultFiles = kept
	s.VaultTabs = append(s.VaultTabs[:idx], s.VaultTabs[idx+1:]...)
	return s.saveLocked()
}
