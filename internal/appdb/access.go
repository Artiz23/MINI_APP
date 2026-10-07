package appdb

import "strings"

const (
	SecMeetings     = "meetings"
	SecAnalytics    = "analytics"
	SecRequests     = "requests"
	SecApprovals    = "approvals"
	SecSaldo        = "saldo"
	SecRates        = "rates"
	SecHolidays     = "holidays"
	SecBalance      = "balance"
	SecCompliance   = "compliance"
	SecDirectory    = "directory"
	SecPayments     = "payments"
	SecTasks        = "tasks"
	SecAppeals      = "appeals"
	SecAppealLawyer = "appeal_lawyer"
	SecAppealDocs   = "appeal_docs"
	SecDocuments    = "documents"
)

var SectionKeys = []string{
	SecMeetings, SecAnalytics,
	SecRequests, SecApprovals, SecSaldo, SecRates, SecHolidays, SecBalance, SecCompliance, SecDirectory, SecPayments, SecTasks,
	SecAppeals, SecAppealLawyer, SecAppealDocs, SecDocuments,
}

var PositionAccessKeys = []string{
	SecMeetings, SecAnalytics,
	SecTasks, SecRequests, SecSaldo, SecApprovals, SecPayments, SecDirectory, SecRates, SecHolidays, SecBalance, SecCompliance,
	SecAppeals, SecAppealLawyer, SecAppealDocs, SecDocuments,
}

const (
	RoleAdmin Role = "admin"
)

type Access map[string]bool

func DefaultAccess() Access {
	return Access{
		SecRequests:  true,
		SecApprovals: true,
		SecSaldo:     true,
		SecDirectory: true,
		SecTasks:     true,
		SecMeetings:  true,
	}
}

func AllAccess() Access {
	a := Access{}
	for _, k := range SectionKeys {
		a[k] = true
	}
	return a
}

func (a Access) Has(sec string) bool {
	if a == nil {
		return false
	}
	return a[sec]
}

func (a Access) Copy() Access {
	out := Access{}
	if a == nil {
		return out
	}
	for k, v := range a {
		out[k] = v
	}
	return out
}

func NormalizeAccess(a Access) Access {
	out := Access{}
	for _, k := range SectionKeys {
		if a != nil && a[k] {
			out[k] = true
		}
	}
	return out
}

func (u User) IsOwner() bool {
	return u.Role == RoleOwner
}

func (u User) IsAdmin() bool {
	return u.Role == RoleOwner || u.Role == RoleAdmin
}

func (u User) CanSection(sec string) bool {
	if u.IsOwner() {
		return true
	}
	return u.Access.Has(sec)
}

const (
	NtfRates     = "rates"
	NtfHolidays  = "holidays"
	NtfSaldo     = "saldo"
	NtfRequests  = "requests"
	NtfApprovals = "approvals"
	NtfBalance   = "balance"
	NtfPayments  = "payments"
)

var NotifyKeys = []string{NtfRates, NtfHolidays, NtfSaldo, NtfRequests, NtfApprovals, NtfBalance, NtfPayments}

type Notify map[string]bool

func DefaultNotify() Notify {
	n := Notify{}
	for _, k := range NotifyKeys {
		n[k] = true
	}
	return n
}

func NormalizeNotify(n Notify) Notify {
	out := DefaultNotify()
	if n == nil {
		return out
	}
	for _, k := range NotifyKeys {
		if v, ok := n[k]; ok {
			out[k] = v
		}
	}
	return out
}

func (n Notify) On(key string) bool {
	if n == nil {
		return true
	}
	v, ok := n[key]
	if !ok {
		return true
	}
	return v
}

const (
	DirManagers       = "managers"
	DirClients        = "clients"
	DirCounterparties = "counterparties"
)

var DirKeys = []string{DirManagers, DirClients, DirCounterparties}

func NormalizeDirs(in []string) []string {
	allow := map[string]bool{}
	for _, raw := range in {
		k := strings.ToLower(strings.TrimSpace(raw))
		for _, ok := range DirKeys {
			if k == ok {
				allow[k] = true
			}
		}
	}
	out := make([]string, 0, len(DirKeys))
	for _, k := range DirKeys {
		if allow[k] {
			out = append(out, k)
		}
	}
	return out
}

func HasDir(dirs []string, kind string) bool {
	for _, d := range dirs {
		if d == kind {
			return true
		}
	}
	return false
}
