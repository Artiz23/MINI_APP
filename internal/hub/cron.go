package hub

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"tg-bot-orh3/MINI_APP/internal/appdb"
	"tg-bot-orh3/MINI_APP/internal/secrets"
	"tg-bot-orh3/internal/rates"
)

func StartCron(api *API) *cron.Cron {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		loc = time.FixedZone("MSK", 3*3600)
	}
	c := cron.New(cron.WithLocation(loc), cron.WithSeconds())
	db, bot := api.db, api.bot
	_, _ = c.AddFunc("0 0 10 * * *", func() { sendOwnerSummary(db, bot, loc) })
	_, _ = c.AddFunc("0 0 10 * * 1-5", func() { sendRatesDM(api, loc) })
	_, _ = c.AddFunc("0 0 13 * * 1-5", func() { sendRatesDM(api, loc) })
	_, _ = c.AddFunc("0 0 16 * * 1-5", func() { sendRatesDM(api, loc) })
	_, _ = c.AddFunc("0 5 10 * * 1-5", func() { sendHolidaysDM(api, loc) })
	_, _ = c.AddFunc("0 30 9 * * *", func() { remindBalance(db, bot, loc, "утро", "10:00") })
	_, _ = c.AddFunc("0 30 18 * * *", func() {
		remindBalance(db, bot, loc, "вечер", "19:00")
		sendSaldoDigest(db, bot, loc)
	})
	c.Start()
	log.Printf("miniapp cron: курсы 10/13/16, праздники 10:05, сводка 10:00, баланс 09:30/18:30, сальдо 18:30 МСК")
	return c
}

func weekdayMSK(loc *time.Location) bool {
	d := time.Now().In(loc).Weekday()
	return d != time.Saturday && d != time.Sunday
}

func sendOwnerSummary(db *appdb.Store, bot *Bot, loc *time.Location) {
	if bot == nil || !weekdayMSK(loc) {
		return
	}
	s := db.Summary()
	text := fmt.Sprintf(
		"Сводка на %s\n\nОткрытые заявки: %v\nОжидают согласования: %v\nОпераций сальдо: %v\nСчетов баланса: %v\nКомплаенс открыт: %v\nСпоры: %v\nПользователей: %v",
		time.Now().In(loc).Format("02.01.2006 15:04"),
		s["requests_open"], s["approvals_pending"], s["saldo_ops"], s["accounts"],
		s["compliance_open"], s["disputes_open"], s["users"],
	)
	if !db.BalanceSlotOK("morning", time.Now().In(loc)) {
		text += "\n\nБаланс на утро ещё не отмечен."
	}
	for _, id := range secrets.MainAdminIDs {
		if err := bot.Notify(id, text); err != nil {
			log.Printf("сводка %d: %v", id, err)
		}
	}
}

func remindBalance(db *appdb.Store, bot *Bot, loc *time.Location, slotName, until string) {
	if bot == nil || !weekdayMSK(loc) {
		return
	}
	ids := db.BalanceResponsibleIDs()
	if len(ids) == 0 {
		return
	}
	text := fmt.Sprintf("Напоминание: обновите баланс компаний (%s) до %s МСК. Откройте Mini App → Баланс.", slotName, until)
	var bump []int64
	for _, id := range ids {
		if !db.Wants(id, appdb.SecBalance, appdb.NtfBalance) {
			continue
		}
		bump = append(bump, id)
		if err := bot.Notify(id, text); err != nil {
			log.Printf("баланс %d: %v", id, err)
		}
	}
	if len(bump) > 0 {
		db.BumpInboxIDs(appdb.SecBalance, bump)
	}
}

func sendSaldoDigest(db *appdb.Store, bot *Bot, loc *time.Location) {
	if bot == nil {
		return
	}
	day := time.Now().In(loc).Format("2006-01-02")
	lots := db.OpenLotsAll()
	if len(lots) == 0 {
		return
	}
	byMgr := map[int64][]appdb.SaldoLot{}
	for _, l := range lots {
		if l.ManagerID == 0 {
			continue
		}
		byMgr[l.ManagerID] = append(byMgr[l.ManagerID], l)
	}
	stamp := time.Now().In(loc).Format("02.01 15:04 МСК")
	for mgrID, mgrLots := range byMgr {
		if !db.Wants(mgrID, appdb.SecSaldo, appdb.NtfSaldo) {
			continue
		}
		if db.EveningSent(mgrID, day) {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Твои сальдо на сейчас (%s) — <b>%d</b>. Напомню завтра в 18:30, если что-то останется.\n", stamp, len(mgrLots))
		for i, l := range mgrLots {
			fmt.Fprintf(&b, "\n<b>%d. %s</b>\n%s %s\n%s", i+1, escHTML(l.CP), formatAmt(l.Remaining), escHTML(l.Currency), escHTML(l.Kind))
		}
		b.WriteString("\n\nЗакрыть можно в Mini App → Сальдо.")
		if err := bot.Notify(mgrID, b.String()); err != nil {
			log.Printf("miniapp сальдо %d: %v", mgrID, err)
			continue
		}
		db.BumpInboxIDs(appdb.SecSaldo, []int64{mgrID})
		db.MarkEveningSent(mgrID, day)
	}
}

func sendRatesDM(api *API, loc *time.Location) {
	if api == nil || api.bot == nil {
		return
	}
	ids := api.db.NotifyIDs(appdb.SecRates, appdb.NtfRates)
	if len(ids) == 0 {
		return
	}
	api.db.BumpInboxIDs(appdb.SecRates, ids)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	text := rates.FormatSnapshot(api.fetchRates(ctx), time.Now(), loc)
	text += "\n\nВыключить: Mini App → Уведомления"
	for _, id := range ids {
		if err := api.bot.NotifyPlain(id, text); err != nil {
			log.Printf("miniapp курсы %d: %v", id, err)
		}
	}
}

func sendHolidaysDM(api *API, loc *time.Location) {
	if api == nil || api.bot == nil || api.holidays == nil {
		return
	}
	ids := api.db.NotifyIDs(appdb.SecHolidays, appdb.NtfHolidays)
	if len(ids) == 0 {
		return
	}
	api.db.BumpInboxIDs(appdb.SecHolidays, ids)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	now := time.Now()
	alerts, err := api.holidays.Check(ctx, now)
	if err != nil {
		log.Printf("miniapp праздники: %v", err)
		return
	}
	day := now.In(loc).Format("2006-01-02")
	if len(alerts) == 0 {
		msg, err := api.holidays.FormatNearest(ctx, now)
		if err != nil || strings.TrimSpace(msg) == "" {
			return
		}
		msg += "\n\nВыключить: Mini App → Уведомления"
		for _, id := range ids {
			if api.db.HolidaySent(id, day) {
				continue
			}
			if err := api.bot.NotifyPlain(id, msg); err != nil {
				log.Printf("miniapp праздник %d: %v", id, err)
				continue
			}
			api.db.MarkHolidaySent(id, day)
		}
		return
	}
	for _, al := range alerts {
		for _, id := range ids {
			if err := api.bot.NotifyPlain(id, al.Text); err != nil {
				log.Printf("miniapp праздник %d: %v", id, err)
			}
		}
	}
	for _, id := range ids {
		api.db.MarkHolidaySent(id, day)
	}
}

func formatAmt(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return strings.Replace(fmt.Sprintf("%.2f", v), ".", ",", 1)
}

func escHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
