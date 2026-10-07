package appdb

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func letters2(s string) string {
	var out []rune
	for _, r := range []rune(strings.ToUpper(strings.TrimSpace(s))) {
		if unicode.IsLetter(r) {
			out = append(out, r)
			if len(out) == 2 {
				break
			}
		}
	}
	return string(out)
}

func empDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	d := b.String()
	if d == "" {
		return ""
	}
	if len(d) == 1 {
		return "0" + d
	}
	if len(d) > 2 {
		return d[len(d)-2:]
	}
	return d
}

func (s *Store) nextDealCodeLocked(mgr Manager, emp Employee, at time.Time) (string, error) {
	ab := letters2(mgr.Abbrev)
	if len([]rune(ab)) != 2 {
		return "", fmt.Errorf("у менеджера нужна аббревиатура из двух букв, например ЖЖ")
	}
	en := empDigits(emp.Number)
	if en == "" {
		return "", fmt.Errorf("в карточке сотрудника должен быть номер (1–2 цифры)")
	}
	month := strconv.Itoa(int(at.Month()))
	prefix := ab + en + month
	max := 0
	year, mo := at.Year(), at.Month()
	take := func(code string, created time.Time) {
		if created.IsZero() || created.Year() != year || created.Month() != mo {
			return
		}
		code = lettersKeep(strings.ToUpper(strings.TrimSpace(code)))
		if !strings.HasPrefix(code, prefix) {
			return
		}
		rest := strings.TrimPrefix(code, prefix)
		n, err := strconv.Atoi(rest)
		if err != nil || n <= 0 {
			return
		}
		if n > max {
			max = n
		}
	}
	for _, r := range s.Requests {
		take(r.UID, r.CreatedAt)
	}
	for _, t := range s.Tasks {
		take(t.UID, t.CreatedAt)
	}
	return prefix + strconv.Itoa(max+1), nil
}

func lettersKeep(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizeManualUID(s string) string {
	return lettersKeep(strings.ToUpper(strings.TrimSpace(s)))
}

func (s *Store) dealCodeTakenLocked(code string) bool {
	code = normalizeManualUID(code)
	if code == "" {
		return false
	}
	for _, r := range s.Requests {
		if r.Status == "deleted" {
			continue
		}
		if normalizeManualUID(r.UID) == code {
			return true
		}
	}
	for _, t := range s.Tasks {
		if normalizeManualUID(t.UID) == code {
			return true
		}
	}
	return false
}
