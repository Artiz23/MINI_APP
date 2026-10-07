# План реализации доработок MINI_APP

> Статус на 7 октября 2026: план реализован и проверен. По решению владельца прибыль вводится сотрудником вручную как сумма и валюта, без формулы и FX-пересчёта. Процентные комиссии сохраняются как условия сделки, но автоматически не пересчитываются. Актуальное состояние и проверки записаны в корневом `SESSION_HANDOFF.md`.

**Репозиторий:** `Artiz23/MINI_APP`  
**Ветка:** `main`  
**Базовый коммит:** `998ccbbb30bd1300de0ee3fa49e25fc4e52efabc` (`998ccbb`)  
**Дата анализа:** 2026-10-07

## 1. Цель

План подготовлен по фактическому состоянию репозитория на коммите `998ccbb` и покрывает требования № 1, 2, 3, 5, 6, 7, 11, 13, 15, 17, 18, 21, 22, 24 и 43. Ниже зафиксированы изменения модели данных, API, Telegram-интеграции, Mini App UI, прав доступа, миграции, тесты и рекомендуемый порядок внедрения.

## 2. Что уже есть в проекте

### 2.1. Архитектура

Проект состоит из Go backend, Telegram-бота, Telegram Mini App на vanilla JS/CSS, JSON-хранилища `data/miniapp.json`, файлового хранилища документов, cron-задач и интеграций с источниками курсов.

Ключевые файлы:

- `cmd/miniapp/main.go` — запуск Store, Bot, API и cron;
- `internal/appdb/store.go` — основное хранилище и часть бизнес-логики;
- `internal/appdb/catalog.go` — сотрудники, менеджеры, клиенты, контрагенты;
- `internal/appdb/deal.go` — оплаты, файлы, обращения по заявке;
- `internal/appdb/task.go` — Канбан;
- `internal/appdb/chats.go` — Telegram-чаты;
- `internal/appdb/access.go`, `position.go` — доступы;
- `internal/hub/api.go` — HTTP API;
- `internal/hub/bot.go` — Telegram;
- `internal/hub/cron.go` — фоновые задачи;
- `web/app.js`, `web/app.css` — Mini App.

`dist/linux` и `dist/windows` являются результатами сборки: их не следует редактировать вручную. Источник изменений — `internal/*`, `cmd/*`, `web/*`, затем пересборка.

### 2.2. Заявки

Текущий `Request` уже содержит `ID`, `UID`, `Title`, `Status`, Telegram thread/link, автора и дату, `TableRef`, `Notes`, `EmployeeID`, `ManagerID`, `ClientID`, `CounterpartyID`.

Новая заявка создаётся со статусом `open`. Поле, подписанное в UI как комментарий, сейчас фактически сохраняется в `Request.Title`. Отдельного canonical-поля комментария нет.

`SetRequestStatus(...)` принимает строку статуса без полноценной state machine. Для большинства переходов нет проверки владельца заявки. `Notes` обновляется только при непустом значении.

### 2.3. Видимость

`GET /api/requests` отдаёт общий список. `GET /api/requests?id=...` возвращает `RequestBundle` с оплатами, файлами, обращениями, сальдо и согласованиями.

Единой серверной политики «можно видеть, но нельзя менять» сейчас нет. Frontend также показывает действия на карточке без достаточного разделения own/foreign. Поэтому требование № 2 необходимо реализовывать на backend и frontend одновременно.

### 2.4. Сальдо

Сальдо — журнал `SaldoOps`. `SaldoTotals()` при чтении пересчитывает итог: `minus` вычитает, остальные операции прибавляют. Пример № 11 (`1 000 000 + 50 000 = 1 050 000`) математически уже соответствует текущей модели, но итог не хранится как отдельное persistent-состояние.

### 2.5. Согласование и оплата

`POST /api/approvals` создаёт `Approval` и уведомляет пользователей, но не отправляет полноценную заявку в настраиваемую Telegram-группу, не прикладывает автоматически сальдо/выгрузку и не хранит delivery metadata.

В `deal.go` уже есть `Payment` со статусами `pending`/`sent`, но отдельного configurable payment chat нет. Следует разделить «сообщение доставлено в Telegram» и «оплата реально выполнена».

### 2.6. Клиенты

`Client` содержит ID/UID, имя, WorkID, связи с сотрудниками и метаданные создания. Признака «выплачивается комиссия» нет. Редактирование существующей карточки клиента уже ограничивается admin/owner, что подходит под № 24.

### 2.7. Курсы и нижняя панель

Сейчас выводятся шесть источников/блоков: Rapira, ЦБ, ProFinance RUB, ProFinance Forex, XE, Investing. Их по № 18 не трогаем.

Dock сейчас: `Главная | Заявки | Канбан | Сальдо | Согл.`. Нужно заменить dock-кнопку Канбана на Курсы, оставив сам Канбан в общем меню.

### 2.8. Документы и cron

Документы уже имеют файловый vault. Старый endpoint архива отдельной заявки отключён. `cron.go` уже работает в `Europe/Moscow`, поэтому ежемесячный snapshot Mini App следует делать отдельным механизмом, не оживляя старый request archive.

## 3. Архитектурный принцип

Не наращивать без необходимости уже большие `store.go`, `api.go` и `app.js`. Новую доменную логику рекомендуется разнести:

```text
internal/appdb/
  request_access.go
  request_workflow.go
  settings.go
  saldo_balance.go
  meetings.go
  analytics.go
  activity.go
  archive_schedule.go
  rates_config.go

internal/hub/
  api_settings.go
  api_meetings.go
  api_analytics.go
  approval_delivery.go
  payment_delivery.go
  archive_job.go
```

Полный рефакторинг старого кода не нужен: цель — не создавать ещё один монолит.

## 4. Порядок реализации

**Этап A — фундамент:** schema version и backup, request permissions, статусы, комментарий, workflow, финансовые поля заявки, настройки.

**Этап B — финансовая целостность:** единый путь изменения сальдо, persistent saldo balances, клиентская комиссия, idempotent reminder.

**Этап C — Telegram workflow:** approval auto-send, сальдо+выгрузка, отдельный payment chat, настройки групп, delivery retry.

**Этап D — новые разделы:** Встречи, custom rates, dock, ежемесячный архив.

**Этап E — бухгалтерия/аналитика:** activity telemetry и отчёты уже по окончательной модели данных.

---

## 5. № 2 — видимость заявок для операциониста

### 5.1. Режимы

По умолчанию операционист открывает **«Мои заявки»**. Дополнительные фильтры:

- Открытые;
- В работе;
- Закрытые.

«Закрытые» объединяют `success_closed` и `failed`.

В режиме **«Все заявки»** чужие заявки открываются только для чтения. Операционист не может менять статус/комментарий, создавать согласование, оплату, задачу, обращение/спор, добавлять/удалять файлы или выполнять другое mutate-действие. Admin/owner может работать со всеми.

### 5.2. Как определить «свою»

Основной признак:

```text
Request.EmployeeID == session.Employee.ID
```

Fallback для legacy:

```text
Request.CreatedBy == session.TelegramUserID
```

`EmployeeID` устойчивее к перепривязке Telegram-пользователя.

### 5.3. Единый policy

Добавить:

```go
type RequestPermission struct {
    CanView   bool
    CanEdit   bool
    CanDelete bool
    IsMine    bool
    IsAdmin   bool
}
```

или эквивалентные `CanViewRequest/CanMutateRequest`.

Policy должен использоваться во всех request-linked endpoint: requests, approvals, payments, tasks, appeals, files/attach, disputes, status/comment и любых вызовах с `request_id`. Скрытия кнопок в JS недостаточно.

### 5.4. API

```http
GET /api/requests?scope=mine
GET /api/requests?scope=all
```

Можно добавить `status=open,in_progress,success_closed,failed`.

В view полезно возвращать:

```json
{
  "can_view": true,
  "can_edit": false,
  "is_mine": false
}
```

### 5.5. UI

```text
[ Мои заявки ] [ Все заявки ]

☑ Открытые
☑ В работе
☑ Закрытые
```

У чужой заявки показывать `Только просмотр`, не рендерить action-кнопки, но backend всё равно повторно проверяет право.

---

## 6. № 1 — редактирование комментария

Добавить в `Request`:

```go
Comment          string
CommentUpdatedAt time.Time
CommentUpdatedBy int64
```

`Title` пока оставить для обратной совместимости, существующих topic/title и старых данных.

Миграция:

```text
если Comment пуст
и Title не пуст
и Title != UID
→ Comment = Title
```

Рекомендуемое правило: комментарий можно изменить/добавить только при `Status == in_progress`; только владелец заявки либо admin/owner. Закрытая история не редактируется.

Изменение комментария не смешивать со status change. Например:

```http
POST /api/requests
{
  "action": "edit_comment",
  "id": 123,
  "comment": "..."
}
```

UI: кнопка `Изменить комментарий` отображается только когда сервер вернул соответствующую capability и статус разрешает редактирование.

---

## 7. № 7 — статусы заявки

Целевые значения:

```text
open            = Открыта
in_progress     = В работе
success_closed  = Успешно закрыта
failed          = Сделка не состоялась
deleted         = технический скрытый статус
```

Переходы:

```text
open → in_progress
in_progress → success_closed
in_progress → failed
```

`success_closed` и `failed` — terminal. Если нужно повторное открытие, сделать отдельный admin/owner action `reopen → in_progress` с причиной и audit event.

Для `failed` добавить:

```go
CloseReason string
```

Backend обязан отклонять `failed` с пустой причиной. HTML `required` не считается защитой.

Миграция legacy:

```text
open        → open
in_progress → in_progress
done        → success_closed
closed      → success_closed
deleted     → deleted
```

Неизвестный статус — warning и ручная диагностика, а не молчаливое преобразование.

---

## 8. № 3 — строгий порядок этапов

Целевой порядок:

```text
Согласование → Оплата → Еще задача → Обращение
```

Добавить:

```go
type RequestStage string

const (
    StageApproval RequestStage = "approval"
    StagePayment  RequestStage = "payment"
    StageTask     RequestStage = "task"
    StageAppeal   RequestStage = "appeal"
    StageComplete RequestStage = "complete"
)
```

В `Request`:

```go
WorkflowStage RequestStage
```

Новая заявка: `approval`.

Предлагаемые переходы:

- `approval → payment` только после `Approval.Status == approved`;
- при rejected остаёмся на approval;
- `payment → task` после реального `Payment.Status == sent`;
- `task → appeal` рекомендуется после `Task.Status == done`;
- `appeal → complete` — после согласованного бизнес-события.

Критичный вопрос: «строгий порядок» означает обязательность всех четырёх этапов или только последовательность доступных действий. Рекомендуется именно backend-gating. Если отдельные этапы бывают необязательны — admin/owner получает `Пропустить этап` с обязательной причиной, операционист — нет.


---

## 9. № 21 + № 17 — экономика заявки и фундамент аналитики

Требования № 21 и № 17 нужно проектировать вместе. Если комиссия контрагента, комиссия агента и условия продажи клиенту сохраняются только свободным текстом, достоверно считать прибыль и строить аналитику будет невозможно.

### 9.1. Структурированные поля заявки

Добавить в заявку блок `Экономика сделки`.

Минимальный вариант:

```go
type CommissionTerm struct {
    Type     string // fixed | percent
    Value    string // decimal-safe representation
    Currency string
}

type SaleTerm struct {
    Value       string
    Currency    string
    Rate        string
    Description string
}

type RequestEconomics struct {
    CounterpartyCommission CommissionTerm
    AgentCommission        CommissionTerm
    ClientSale             SaleTerm
}

type Request struct {
    // existing...
    AgentID   int64
    Economics RequestEconomics
}
```

На UI операционист заполняет:

```text
Комиссия контрагента
Тип: сумма / процент
Значение
Валюта

Агент: нет / выбрать агента
Комиссия агента
Тип: сумма / процент
Значение
Валюта

Продажа клиенту
Сумма / курс / валюта
Комментарий
```

Если в бизнесе «как продали клиенту» означает конкретно курс, а не сумму сделки, поле `Rate` должно стать обязательным, а не оставаться описательным.

### 9.2. Добавить сущность «Агент»

В текущем каталоге отдельной сущности агента нет. Для аналитики «по агенту» нельзя использовать имя свободным текстом.

Добавить:

```go
type Agent struct {
    ID          int64
    UID         string
    Name        string
    WorkID      string
    CreatedBy   int64
    CreatedName string
    CreatedAt   time.Time
}
```

И новый kind справочника:

```text
agents
```

Заявка должна ссылаться по `AgentID`, а не по имени.

### 9.3. Денежная точность

Текущий проект во многих местах использует `float64`. Для новых комиссий/прибыли предпочтительно decimal/fixed-point представление.

Если полный переход всего проекта слишком большой для этой итерации:

1. новые финансовые поля сделать decimal-safe;
2. legacy `SaldoOp.Amount float64` временно оставить для совместимости;
3. вынести переход старых денежных полей в отдельную миграцию.

Это особенно важно для процентов, суммирования прибыли и отчётов за период.

### 9.4. Новый раздел «Бухгалтерия и аналитика»

Добавить отдельный access key, например:

```go
SecAnalytics = "analytics"
```

UI:

```text
Бухгалтерия и аналитика

Объект:
[ Контрагент | Агент | Клиент | Сотрудник ]

Период:
[ Сегодня | 7 дней | Месяц | Произвольный ]

Анализ:
[ Активность | Заявки | Успешность | Оборот | Комиссии | Прибыль ]
```

Минимальные показатели.

**По сотруднику:**

- создано заявок;
- в работе;
- успешно закрыто;
- сделка не состоялась;
- conversion rate;
- сумма клиентских продаж;
- комиссия контрагента;
- комиссия агента;
- комиссия клиента;
- прибыль;
- средняя прибыль на успешную сделку;
- среднее время закрытия;
- активность в Mini App.

**По клиенту:**

- количество сделок;
- успешные / несостоявшиеся;
- оборот;
- клиентские комиссии;
- прибыль;
- сотрудники, которые работали с клиентом.

**По контрагенту:**

- количество сделок;
- текущее сальдо;
- оборот;
- комиссии;
- прибыль;
- среднее время согласования.

**По агенту:**

- количество сделок;
- общая агентская комиссия;
- оборот;
- прибыль после агентской комиссии.

### 9.5. Прибыль вводится вручную

По решению владельца сотрудник указывает готовую прибыль и валюту в экономике заявки. Отрицательная сумма означает убыток. Система не выводит прибыль из продажи и комиссий, не конвертирует валюты и не применяет FX. Аналитика суммирует введённые значения успешно закрытых заявок отдельно по валютам.

---

## 10. Аналитика активности сотрудников

Требование «обычно активен столько-то часов в день» невозможно достоверно получить только из `CreatedAt` заявок, оплат или сальдо.

### 10.1. Heartbeat

Добавить:

```http
POST /api/activity/ping
```

Frontend отправляет heartbeat, например раз в 5 минут, только когда:

- Mini App открыт;
- вкладка находится в foreground;
- пользователь недавно взаимодействовал с приложением.

Backend хранит агрегированные временные bucket, а не каждый mouse/touch event.

Например:

```go
type ActivitySlice struct {
    UserID     int64
    EmployeeID int64
    Date       string
    Bucket     string // 5-minute bucket
}
```

Один bucket записывается максимум один раз.

Расчёт:

```text
активные часы = уникальные 5-минутные bucket * 5 / 60
```

Показатель в UI лучше называть **«Активность в Mini App»**, а не «рабочие часы», поскольку сотрудник может работать вне приложения.

---

## 11. № 24 — комиссия клиента

### 11.1. Модель клиента

Минимально:

```go
type Client struct {
    // existing...
    CommissionEnabled bool
}
```

Если комиссия должна иметь заранее заданное правило, дополнительно:

```go
CommissionType     string // fixed | percent
CommissionValue    string
CommissionCurrency string
CommissionComment  string
```

Из исходного требования обязательным является checkbox; ставка/сумма должна быть уточнена отдельно.

### 11.2. UI

В карточке клиента для admin/owner:

```text
☑ Выплачивается комиссия
```

Для обычного пользователя:

```text
Комиссия клиента: Да
```

без возможности изменения.

### 11.3. Автоматический reminder при успешном закрытии

При переходе заявки в `success_closed` backend:

1. загружает привязанного клиента;
2. проверяет `CommissionEnabled`;
3. проверяет, что reminder по этой заявке ещё не создан;
4. создаёт обязательный элемент в существующем разделе «Отправка»;
5. тип элемента — `client_commission`;
6. текст содержит `Оплата комиссия клиента`;
7. добавляет UID заявки, клиента, сотрудника, контрагента и сумму/правило комиссии, если они известны;
8. отправляет его в настроенный payment chat;
9. сохраняет состояние Telegram-доставки.

Не создавать отдельную параллельную очередь. Расширить существующий `Payment`:

```go
Kind string // regular | client_commission
Auto bool
```

Идемпотентность:

```text
RequestID + Kind(client_commission) = уникальная бизнес-операция
```

Повторная доставка/повторный вызов закрытия не должен создать вторую комиссию.

---

## 12. № 22 + № 11 — единое сальдо

### 12.1. Оставить журнал операций

`SaldoOps` должен оставаться audit trail и источником истории.

Добавить материализованный текущий баланс:

```go
type SaldoBalance struct {
    CounterpartyID int64
    CP             string
    Kind           string
    Currency       string
    Amount         string // decimal-safe target
    UpdatedAt      time.Time
    UpdatedBy      int64
}
```

Предпочтительный ключ:

```text
CounterpartyID + Kind + Currency
```

Для старых операций без `CounterpartyID` на период миграции допустим fallback по нормализованному имени контрагента.

### 12.2. Единственная точка записи

Ввести внутренний метод:

```go
ApplySaldoDelta(...)
```

Он под одним lock:

1. валидирует контрагента;
2. валидирует валюту;
3. валидирует `plus/minus`;
4. создаёт `SaldoOp`;
5. пересчитывает `SaldoBalance`;
6. сохраняет JSON один раз;
7. возвращает созданную операцию и новый баланс.

После этого нельзя оставлять обходные места, где `SaldoOps` дописывается напрямую.

`AddSaldo` и `CloseSaldoLot` должны использовать одну и ту же внутреннюю primitive (`applySaldoDeltaLocked` или аналог).

### 12.3. Контрагенты с финансовой историей

Физическое удаление контрагента с сальдо опасно. Рекомендуется:

```text
Archived = true
```

Переименование меняет отображаемое имя, но финансовая идентичность остаётся по `CounterpartyID`.

### 12.4. Миграция и сверка

При первой загрузке новой схемы:

```text
SaldoBalances = rebuild(SaldoOps)
```

После этого выполнить reconciliation:

```text
rebuild(SaldoOps) == SaldoBalances
```

При расхождении:

- записать warning;
- не исправлять историю молча;
- дать owner/admin action `Пересчитать сальдо из журнала`.

### 12.5. Acceptance example № 11

Исходно:

```text
1 000 000 RUB
```

Операция:

```text
Принять +50 000 RUB
```

После одной бизнес-транзакции:

```text
SaldoOps: +50 000
SaldoBalance: 1 050 000
API total: 1 050 000
UI: 1 050 000
report: 1 050 000
counterparty export: 1 050 000
```

Все компоненты читают одну canonical-сумму.

---

## 13. № 5 — автоматическое сальдо и выгрузка при согласовании

### 13.1. Целевой сценарий

Операционист нажимает `Согласование`.

Backend:

1. проверяет право на заявку;
2. проверяет `WorkflowStage == approval`;
3. проверяет настройку `ApprovalChatID`;
4. создаёт `Approval`;
5. получает заявку и привязанного контрагента;
6. получает единый snapshot текущего сальдо;
7. формирует выгрузку только по этому контрагенту;
8. отправляет карточку заявки в approval chat;
9. отправляет сальдо;
10. отправляет файл выгрузки;
11. сохраняет Telegram delivery metadata;
12. дальнейший этап `payment` разрешается только после реального approve.

### 13.2. Убрать ручное сальдо из карточки заявки

Из `renderRequestCard()` убрать action:

```text
Сальдо
```

и связанную логику перехода из заявки.

Сам самостоятельный раздел «Сальдо» остаётся. Исторические `SaldoOps.RequestID` сохраняются.

### 13.3. Выгрузка контрагента

Добавить функцию уровня store/export:

```go
CounterpartySaldoExport(counterpartyID)
```

Например CSV:

```text
Дата
UID операции
Заявка
Тип
Валюта
Действие
Сумма
Остаток
Сотрудник
```

Имя:

```text
counterparty_<uid>_<YYYYMMDD_HHMM>.csv
```

### 13.4. Согласованный snapshot

Текст сальдо и CSV должны строиться из одного snapshot. Нельзя отдельно читать баланс до и после потенциальной параллельной операции.

Данные получить атомарно, затем отпустить Store mutex и только после этого выполнять сетевые Telegram-вызовы.

### 13.5. Delivery state

Расширить `Approval`:

```go
ChatID         int64
MessageID      int
DeliveryStatus string // pending | sent | error
DeliveryError  string
DeliveredAt    time.Time
```

Telegram-ошибка не должна удалять само согласование.

В UI admin/owner:

```text
Не доставлено в чат
[ Повторить отправку ]
```

Retry повторяет доставку существующей записи, но не создаёт новое согласование.

---

## 14. № 43 — настройки Telegram-чатов

Текущий общий `ManagedChat` можно сохранить как справочник, но бизнес-настройкам нужны явные роли.

Добавить:

```go
type MiniAppSettings struct {
    ApprovalChatID int64 `json:"approval_chat_id,omitempty"`
    PaymentChatID  int64 `json:"payment_chat_id,omitempty"`
    SaldoChatID    int64 `json:"saldo_chat_id,omitempty"`

    // archive + rate settings below
}
```

В `Store`:

```go
Settings MiniAppSettings `json:"settings,omitempty"`
```

### 14.1. Где показывать настройки

Только admin/owner.

**Заявки**

```text
⚙ Настройки согласования
```

меняет `ApprovalChatID`.

**Согласование**

Та же настройка и то же поле `ApprovalChatID`. Это не второй независимый ID.

**Сальдо**

```text
⚙ Настройки
```

меняет `SaldoChatID`.

**Отправка**

```text
⚙ Настройки
```

меняет `PaymentChatID`.

### 14.2. API

Например:

```http
GET  /api/app-settings
POST /api/app-settings
```

Изменение — только admin/owner.

Для обычного пользователя при необходимости можно отдавать только:

```json
{
  "approval_chat_configured": true,
  "payment_chat_configured": true,
  "saldo_chat_configured": true
}
```

### 14.3. Валидация

При сохранении:

- `0` запрещён;
- корректно обработать отрицательный Telegram group/supergroup ID;
- если возможно, проверить чат через Telegram API;
- показать найденное имя;
- добавить кнопку `Отправить тест`.

---

## 15. № 6 — оплата в отдельном чате

Жёсткое правило backend:

```text
ApprovalChatID != PaymentChatID
```

если оба настроены.

Если admin/owner пытается сохранить одинаковые ID:

```text
Чат оплаты должен отличаться от чата согласования
```

### 15.1. Разделить business state и Telegram delivery

Расширить `Payment`:

```go
ChatID         int64
MessageID      int
DeliveryStatus string // pending | sent | error
DeliveryError  string
DeliveredAt    time.Time
Kind           string // regular | client_commission
Auto           bool
```

При этом существующий `Payment.Status` (`pending`/`sent`) означает состояние самой оплаты, а не факт отправки сообщения в Telegram.

### 15.2. Создание оплаты

1. Проверить request permission.
2. Проверить stage `payment`.
3. Проверить `PaymentChatID`.
4. Сохранить `Payment{Status:"pending"}`.
5. Отправить карточку в payment chat.
6. Сохранить delivery metadata.

Пример сообщения:

```text
ОПЛАТА

Заявка: ABC-123
Клиент: ...
Контрагент: ...
Сотрудник: ...
Инструкция: ...
```

Для автоматической клиентской комиссии:

```text
ОПЛАТА КОМИССИЯ КЛИЕНТА

Заявка: ...
Клиент: ...
Комиссия: ...
```

---

## 16. № 13 — раздел «Встречи»

### 16.1. Access

В `access.go`:

```go
SecMeetings = "meetings"
```

Добавить в `AllAccess()`, `NormalizeAccess()`, frontend `POS_ACCESS` и нужные default positions.

### 16.2. Модель

```go
type Meeting struct {
    ID          int64
    UID         string
    Title       string
    URL         string
    StartsAt    time.Time
    CreatedBy   int64
    CreatedName string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

Если ввод считается временем Москвы, парсить как `Europe/Moscow`; в данных хранить полноценный ISO timestamp.

### 16.3. API

```http
GET    /api/meetings
POST   /api/meetings
DELETE /api/meetings?id=...
```

Фильтры:

```text
upcoming
past
all
```

### 16.4. UI

В общем/боковом меню:

```text
📅 Встречи
```

Карточка:

```text
Название встречи
08.10.2026 · 14:00 МСК
[ Открыть встречу ]
```

Ссылку открывать безопасным WebApp/openLink способом.

Рекомендуемые права:

- видеть — пользователи с `SecMeetings`;
- создавать/изменять свою — пользователи с соответствующим доступом;
- изменять/удалять любую — admin/owner.

Если по бизнесу создавать встречи должны только admin/owner, это лучше ужесточить до начала разработки.

---

## 17. № 15 — ежемесячное сохранение Mini App в «Документы»

### 17.1. Настройки

В разделе «Документы», только admin/owner:

```text
[ ⚙ Автосохранение Mini App ]

Включено: Да/Нет
День месяца: 1–31
Время: HH:MM
Часовой пояс: Москва

Что сохранять:
☑ Заявки
☑ Согласования
☑ Сальдо
☑ Отправка
☑ Задачи
☑ Обращения
☑ Справочники
☑ Встречи
☑ Балансы
☑ Комплаенс
☑ Аналитика/activity
...
```

И обязательная кнопка:

```text
[ Сохранить сейчас ]
```

### 17.2. Модель

```go
type MonthlyArchiveSettings struct {
    Enabled      bool
    Day          int
    TimeHHMM     string
    Sections     []string
    IncludeFiles bool
    LastRunMonth string
    UpdatedBy    int64
    UpdatedAt    time.Time
}
```

Хранить внутри `MiniAppSettings`.

### 17.3. Cron

Надёжный вариант — системная проверка раз в минуту:

```text
ArchiveDue(now in Europe/Moscow)
```

Идемпотентность:

```text
LastRunMonth != YYYY-MM
```

Для дня 29/30/31 в коротком месяце рекомендуемое правило:

```text
если указанного дня нет → выполнить в последний календарный день месяца
```

### 17.4. Формат snapshot

Предпочтительно ZIP:

```text
miniapp_2026-10_20261031_2300_MSK.zip
```

Внутри, в зависимости от выбранных секций:

```text
manifest.json
requests.json
requests.csv
approvals.json
payments.json
saldo_balances.csv
saldo_operations.csv
tasks.json
appeals.json
directory.json
meetings.json
activity.json
...
```

Если `IncludeFiles=true`:

```text
files/
```

### 17.5. Размещение

Не восстанавливать старый disabled request archive. Создавать системный файл в существующем Documents vault, например:

```text
Документы → Прочее → Автоархив Mini App
```

Если текущая folder-модель не позволяет отдельный подпункт, добавить системный folder key `miniapp_archive`.

### 17.6. Безопасность архива

Не включать:

- bot token;
- init data;
- environment secrets;
- runtime credentials.

Архив предназначен для бизнес-данных, а не для конфигурационных секретов.



---

## 18. № 18 — три настраиваемых курса и Курсы вместо Канбана в dock

### 18.1. Шесть существующих карточек не менять

Оставить текущий блок:

1. Rapira USDT/RUB;
2. ЦБ;
3. ProFinance руб;
4. ProFinance forex;
5. XE EUR/USD;
6. Investing USD/RUB.

Их существующую логику получения и отображения не переписывать в рамках этого требования, кроме общего рефакторинга, необходимого для новых слотов.

### 18.2. Три пользовательских слота сверху

Добавить в settings:

```go
type RateSlot struct {
    Enabled bool
    Source  string
    Symbol  string
    Label   string
}

type MiniAppSettings struct {
    // ...
    CustomRateSlots [3]RateSlot
}
```

Все пользователи видят выбранные три значения. Изменять их могут только admin/owner.

### 18.3. Не давать вводить название parser-функции вручную

Фраза «покажет, что мы можем взять из этого сайта» должна быть реализована как backend capabilities.

Добавить:

```http
GET /api/rates/options
```

Ответ формируется только из реально поддерживаемых адаптеров, например:

```json
[
  {
    "source": "cbr",
    "pairs": ["USD/RUB", "EUR/RUB", "CNY/RUB", "AED/RUB"]
  },
  {
    "source": "profinance",
    "pairs": ["EUR/USD", "USD/RUB", "EUR/RUB", "CNY/RUB"]
  },
  {
    "source": "xe",
    "pairs": ["EUR/USD"]
  },
  {
    "source": "investing",
    "pairs": ["USD/RUB"]
  },
  {
    "source": "rapira",
    "pairs": ["USDT/RUB"]
  }
]
```

Точный список должен генерироваться из фактически реализованных возможностей, а не быть захардкожен только в frontend.

### 18.4. Provider abstraction

Для новых слотов желательно небольшой слой:

```go
type RateProvider interface {
    Name() string
    Capabilities() []RateInstrument
    Fetch(ctx context.Context, instrument string) (RateValue, error)
}
```

Это не требует немедленно переписывать шесть старых карточек. Существующий `fetchRates()` можно сохранить, а новый adapter layer использовать сначала только для custom slots.

### 18.5. UI настройки

В разделе Курсы, только admin/owner:

```text
⚙ Настроить верхние курсы
```

Каждый из трёх слотов:

```text
Источник:   [ ProFinance ]
Инструмент: [ USD/RUB ]
Название:   [ Мой USD ]
Включён:    [ Да ]
```

Ниже остаются шесть текущих карточек.

### 18.6. Нижняя панель

Было:

```text
Главная | Заявки | Канбан | Сальдо | Согл.
```

Стало:

```text
Главная | Заявки | Курсы | Сальдо | Согл.
```

Канбан не удаляется. Он остаётся в общем меню/«Еще».

---

## 19. Единый frontend-компонент настроек разделов

Не делать четыре независимых реализации кнопки настроек.

Добавить условный:

```js
openSectionSettings(scope)
```

Scopes:

```text
approval
saldo
payment
```

Разделы `Заявки` и `Согласование` используют один `approval` scope и один `ApprovalChatID`.

Это исключит расхождение конфигурации, когда в одном экране показывается один ID, а в другом другой.

---

## 20. Рекомендуемые изменения API — сводка

```text
GET  /api/requests?scope=mine|all&status=...
POST /api/requests
     action=create
     action=edit_comment
     action=set_status
     action=reopen
     action=skip_stage        # только если бизнес разрешит, admin/owner

GET/POST /api/approvals
POST     /api/approvals?action=retry_delivery

GET/POST /api/payments
POST     /api/payments?action=retry_delivery

GET/POST /api/saldo
POST     /api/saldo?action=rebuild_balances

GET/POST /api/app-settings

GET/POST/DELETE /api/meetings

GET  /api/rates
GET  /api/rates/options
POST /api/rates/config

GET  /api/analytics
POST /api/activity/ping

GET/POST /api/archive-settings
POST     /api/archive-now
```

Не обязательно создавать физически отдельный route для каждой строки: можно сохранить текущий стиль action-based API. Критично, чтобы каждое изменение данных проходило серверную авторизацию и бизнес-валидацию.

---

## 21. Целевая модель данных — сводка

Концептуальный вариант:

```go
type Request struct {
    // existing...

    Comment          string
    CommentUpdatedAt time.Time
    CommentUpdatedBy int64

    CloseReason string

    WorkflowStage RequestStage

    AgentID   int64
    Economics RequestEconomics
}

type Client struct {
    // existing...
    CommissionEnabled bool
}

type Payment struct {
    // existing...
    Kind           string
    Auto           bool
    ChatID         int64
    MessageID      int
    DeliveryStatus string
    DeliveryError  string
    DeliveredAt    time.Time
}

type Approval struct {
    // existing...
    ChatID         int64
    MessageID      int
    DeliveryStatus string
    DeliveryError  string
    DeliveredAt    time.Time
}

type SaldoBalance struct {
    CounterpartyID int64
    CP             string
    Kind           string
    Currency       string
    Amount         string
    UpdatedAt      time.Time
    UpdatedBy      int64
}

type MiniAppSettings struct {
    ApprovalChatID  int64
    PaymentChatID   int64
    SaldoChatID     int64
    MonthlyArchive  MonthlyArchiveSettings
    CustomRateSlots [3]RateSlot
}

type Meeting struct {
    ID          int64
    UID         string
    Title       string
    URL         string
    StartsAt    time.Time
    CreatedBy   int64
    CreatedName string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

---

## 22. Миграция `data/miniapp.json`

Из-за JSON-сериализации новые поля сами по себе backward-compatible по zero values, но статусы, workflow и materialized saldo требуют явной миграции.

### 22.1. Schema version

Добавить:

```go
SchemaVersion int `json:"schema_version,omitempty"`
```

При startup новой версии:

1. прочитать старый файл;
2. создать backup исходного `miniapp.json`;
3. выполнить последовательные migrations;
4. сохранить новую схему;
5. только после успешной записи продолжать запуск приложения.

Backup должен быть отдельным файлом, например:

```text
miniapp.json.backup-20261007-155400
```

### 22.2. Миграция в новую схему

Для первого релиза:

- добавить `Settings`;
- перенести legacy-комментарий из `Title` в `Comment`, когда применимо;
- нормализовать старые статусы;
- назначить `WorkflowStage`;
- rebuild `SaldoBalances` из `SaldoOps`;
- `Client.CommissionEnabled = false` по умолчанию;
- инициализировать Meetings/Agents/Activity пустыми;
- обновить access keys через `NormalizeAccess()`.

### 22.3. Не ставить всем старым заявкам stage=approval

Реконструировать stage из связанных данных:

```text
закрытая заявка                     → complete
есть обращение                      → appeal
есть связанная задача               → task
есть Payment pending/sent           → payment/task по состоянию
есть approved Approval              → payment
иначе                               → approval
```

Если состояние неоднозначно, записать migration warning.

### 22.4. Rollback

В первой версии не удалять legacy-поля после переноса. Это уменьшит риск при откате binary.

---

## 23. Telegram-интеграция

В `bot.go` уже есть базовая отправка в группу и fallback для supergroup. Для требования № 5 нужен helper отправки файла именно в group chat:

```go
SendBytesToGroup(chatID, name, data, caption)
```

### 23.1. Нельзя выполнять Telegram I/O под Store mutex

Правильный шаблон:

```text
1. lock Store
2. validate
3. persist business record as pending
4. unlock
5. Telegram API request
6. lock
7. save delivery result
8. unlock
```

Иначе зависший Telegram API способен блокировать операции всего Mini App.

### 23.2. Retry

Повторная Telegram-доставка не создаёт заново:

- Approval;
- Payment;
- client commission reminder.

Retry работает с существующим ID бизнес-объекта и обновляет только delivery metadata.

---

## 24. Audit log

Для финансовых и административных изменений рекомендуется добавить:

```go
type AuditEvent struct {
    ID         int64
    ActorID    int64
    ActorName  string
    EntityType string
    EntityID   int64
    Action     string
    Before     string
    After      string
    At         time.Time
}
```

Минимально фиксировать:

- изменение комментария;
- изменение статуса;
- reopen;
- skip workflow stage;
- изменение client commission flag;
- saldo delta;
- saldo rebuild;
- изменение ID Telegram-чата;
- изменение monthly archive settings;
- изменение custom rate slot;
- отметку оплаты `sent`.

Audit особенно важен, потому что требования затрагивают деньги и расширяют административные настройки.

---

## 25. Frontend — перечень изменений

### Навигация

- добавить `Встречи`;
- добавить `Бухгалтерия и аналитика`;
- заменить Канбан на Курсы в нижнем dock;
- Канбан оставить в общем меню.

### Заявки

- switch `Мои / Все`;
- чекбоксы Открытые / В работе / Закрытые;
- read-only чужих заявок;
- новые статусы;
- modal причины «Сделка не состоялась»;
- edit comment;
- поля экономики сделки;
- отображение workflow stage;
- последовательные action-кнопки;
- убрать `Сальдо`;
- admin settings для approval chat.

### Справочник

- client commission checkbox;
- сущность Agent;
- изменение комиссии только admin/owner.

### Согласование

- settings;
- delivery state;
- retry;
- индикация автоматического saldo/export.

### Сальдо

- settings;
- persistent current balance;
- diagnostic/rebuild только admin/owner;
- убрать ручной переход из request card.

### Отправка

- settings;
- отдельный payment chat;
- `regular` / `client_commission`;
- Telegram delivery status;
- retry.

### Курсы

- 3 custom slots сверху;
- настройки admin/owner;
- 6 текущих карточек без функциональных изменений.

### Документы

- monthly archive settings;
- `Сохранить сейчас`;
- список созданных автоархивов.

### Встречи

- список;
- дата;
- время;
- URL;
- create/edit/delete по правам.

### Аналитика

- dimension;
- entity;
- date range;
- metric;
- summary/table.

---

## 26. Backend security checklist

До завершения задачи проверить прямыми HTTP-вызовами:

```text
[ ] operator не меняет чужой request
[ ] operator не создаёт approval по чужому request_id
[ ] operator не создаёт payment по чужому request_id
[ ] operator не создаёт task по чужому request_id
[ ] operator не создаёт appeal/dispute по чужому request_id
[ ] operator не загружает/удаляет файл чужой заявки
[ ] operator не меняет status чужой заявки
[ ] operator не меняет comment чужой заявки
[ ] operator не перескакивает workflow ручным POST
[ ] только admin/owner меняет chat IDs
[ ] только admin/owner меняет Client.CommissionEnabled
[ ] только admin/owner меняет custom rate slots
[ ] только admin/owner меняет monthly archive settings
[ ] финансовая analytics недоступна роли без соответствующего access
```

Это обязательная часть реализации: UI-ограничения сами по себе не являются системой прав.

---

## 27. Тестирование

### 27.1. Unit tests — заявки

```text
create → open
open → in_progress
in_progress → success_closed
in_progress → failed без причины = ошибка
in_progress → failed с причиной = OK
unknown status = ошибка
invalid transition = ошибка
edit comment в in_progress = OK
edit comment после закрытия = ошибка
```

### 27.2. Unit tests — permissions

```text
operator + own request + mutate = allow
operator + foreign request + view = allow
operator + foreign request + mutate = deny
admin + any request = allow
owner + any request = allow
```

### 27.3. Unit tests — workflow

```text
payment before approval = deny
task before payment = deny
appeal before task = deny
valid sequence = allow
```

Если будет `skip_stage`, отдельно проверить admin/owner + обязательную причину.

### 27.4. Unit tests — client commission

```text
commission disabled + success close → 0 reminder
commission enabled + success close → 1 reminder
повторный close/retry → всё ещё 1 reminder
failed deal → 0 client commission reminder
```

### 27.5. Unit tests — saldo

```text
1 000 000 + 50 000 = 1 050 000
minus уменьшает баланс
CloseSaldoLot меняет тот же canonical balance
rebuild(SaldoOps) == materialized balance
rename counterparty не теряет историю
```

### 27.6. Unit tests — archive

```text
due schedule → один archive
повторный cron tick в том же месяце → без дубля
31-е в коротком месяце → последний день
архив содержит только выбранные sections
manual "Сохранить сейчас" работает независимо от schedule
```

### 27.7. API tests

Проверить:

```text
200/201 — success
400 — validation
403 — permission
404 — missing object
409 — invalid workflow/duplicate, если этот код будет принят
```

Критичные сценарии:

- mutate foreign request → `403`;
- failed без reason → `400`;
- PaymentChatID == ApprovalChatID → `400`;
- отсутствующий configured chat → понятная ошибка;
- retry delivery не создаёт duplicate record.

### 27.8. Telegram delivery tests

Желательно ввести interface:

```go
type TelegramSender interface {
    SendToGroup(...)
    SendBytesToGroup(...)
}
```

В unit/integration tests использовать fake sender.

Проверить:

- approval отправляет request + saldo + export;
- используется именно ApprovalChatID;
- payment использует только PaymentChatID;
- Telegram error сохраняется;
- retry не создаёт duplicate Approval/Payment.

### 27.9. Frontend tests

Расширить существующий JS test suite:

- `mine/all`;
- три статусных checkbox;
- foreign request read-only;
- own request actions;
- failed modal + mandatory reason;
- comment edit только в work;
- client commission checkbox только admin/owner;
- dock содержит Rates вместо Kanban;
- Канбан остаётся в More;
- старые 6 rate cards продолжают отображаться;
- новые 3 slots находятся сверху;
- settings не показывается обычному пользователю;
- Meetings route;
- Archive settings;
- Analytics access.



---

## 28. Acceptance criteria по каждому требованию

### № 1. Редактирование комментария

Готово, если:

- после создания заявки можно добавить комментарий, которого раньше не было;
- существующий комментарий можно изменить;
- backend запрещает редактирование в недопустимом статусе;
- операционист не может редактировать чужую заявку;
- изменение переживает restart приложения.

### № 2. Видимость заявок

Готово, если:

- операционист по умолчанию открывает `Мои заявки`;
- есть режим `Все заявки`;
- есть фильтры `Открытые / В работе / Закрытые`;
- чужая заявка открывается на просмотр;
- любые mutate-запросы по чужой заявке блокируются на backend;
- admin/owner может работать с любой заявкой.

### № 3. Порядок этапов

Готово, если последовательность:

```text
Согласование → Оплата → Еще задача → Обращение
```

проверяется на backend и её нельзя обойти прямым HTTP-вызовом.

### № 5. Автоотправка при согласовании

Готово, если одним действием:

- создаётся Approval;
- approval chat получает заявку;
- получает актуальное сальдо;
- получает выгрузку по привязанному контрагенту;
- ручное действие `Сальдо` отсутствует в карточке заявки;
- Telegram failure виден и может быть повторён без дубля.

### № 6. Отдельный чат оплаты

Готово, если:

- существует отдельный configurable `PaymentChatID`;
- сообщения оплаты не уходят в approval chat;
- одинаковый approval/payment ID запрещён;
- delivery retry не создаёт вторую оплату.

### № 7. Статусы

Готово, если приложение использует:

```text
В работе
Успешно закрыта
Сделка не состоялась
```

а `Сделка не состоялась` невозможно сохранить без обязательной причины.

### № 11. Пример сальдо

Готово, если сценарий:

```text
1 000 000 + принять 50 000 = 1 050 000
```

даёт один и тот же результат в Store, API, UI, отчёте и выгрузке.

### № 13. Встречи

Готово, если в меню есть `Встречи`, а запись содержит как минимум:

- название;
- ссылку;
- дату;
- время.

### № 15. Ежемесячный архив

Готово, если admin/owner может:

- включить/выключить расписание;
- выбрать день;
- выбрать время по Москве;
- выбрать разделы;
- нажать `Сохранить сейчас`;
- получить snapshot в Документах;
- и cron не создаёт повторный архив за тот же месяц.

### № 17. Бухгалтерия и аналитика

Готово, если авторизованный пользователь может выбрать:

- сущность;
- период;
- тип анализа;

и получить результат из структурированных данных, а не из парсинга свободного текста.

### № 18. Курсы

Готово, если:

- шесть существующих карточек продолжают работать;
- сверху отображаются 3 configurable slots;
- admin/owner выбирает только реально поддерживаемый source/instrument;
- обычный пользователь только смотрит;
- Курсы находятся в dock вместо Канбана;
- Канбан остаётся доступен из общего меню.

### № 21. Комиссии/продажа в заявке

Готово, если операционист может структурированно сохранить:

- комиссию контрагента;
- агента при наличии;
- комиссию агента;
- параметры продажи клиенту;

и эти значения можно использовать в аналитике.

### № 22. Единое сальдо

Готово, если любая операция, меняющая сальдо, в одной бизнес-транзакции обновляет:

- журнал операций;
- canonical текущий баланс.

Никакой endpoint не должен изменять одно без второго.

### № 24. Комиссия клиента

Готово, если:

- admin/owner ставит checkbox в клиенте;
- обычный пользователь не может его изменить;
- при `success_closed` создаётся ровно одно обязательное напоминание `Оплата комиссия клиента`;
- оно появляется в разделе Отправка;
- оно отправляется в payment chat.

### № 43. Настройки чатов

Готово, если admin/owner видит settings:

- в Заявках;
- в Согласовании;
- в Сальдо;
- в Отправке;

и фактическая Telegram-доставка использует сохранённые значения.

---

## 29. Оценка объёма работ

Оценка — чистые инженерные дни для одного разработчика, знакомого с кодовой базой. Это не календарный срок.

| Блок | Оценка |
|---|---:|
| Request access + statuses + comment + migration | 2–3 дня |
| Strict workflow | 2–3 дня |
| Request economics + Agent | 2–3 дня |
| Saldo centralization + materialized balance | 2–3 дня |
| Client commission reminder | 1–2 дня |
| Chat settings | 1–2 дня |
| Approval auto-send saldo/export | 2–3 дня |
| Separate payment chat + retry | 1–2 дня |
| Meetings | 1–2 дня |
| Custom rates + dock | 2–3 дня |
| Monthly archive | 2–4 дня |
| Activity telemetry | 2–3 дня |
| Accounting/analytics | 4–7 дней |
| Migrations + integration/regression/release | 3–5 дней |

Ориентир на весь объём: **27–45 инженерных дней**.

Наибольшая неопределённость:

- точная формула прибыли;
- обязательность каждого workflow-этапа;
- семантика «как продали клиенту»;
- правила комиссий;
- необходимость включать документы/вложения в ежемесячный snapshot.

---

## 30. Рекомендуемое разбиение на релизы

### Release 1 — безопасность и lifecycle заявки

Включить:

- № 1;
- № 2;
- № 3;
- № 7;
- базовые поля № 21;
- SchemaVersion/migration/audit foundation.

Цель — сначала закрыть возможность неверных действий и сформировать стабильную модель заявки.

### Release 2 — финансы и Telegram workflow

Включить:

- № 22;
- № 11;
- № 24;
- № 5;
- № 6;
- № 43.

Цель — canonical saldo, approval/payment delivery и комиссия клиента.

### Release 3 — дополнительные разделы и автоматизация

Включить:

- № 13;
- № 15;
- № 18.

### Release 4 — аналитика

Включить:

- № 17;
- activity telemetry;
- отчёты на данных № 21/24.

Так аналитика строится уже на стабильной экономической модели, а не переделывается после изменения структуры комиссий.

---

## 31. Вопросы, которые необходимо зафиксировать до кодирования соответствующих блоков

### Workflow

1. `Еще задача` обязательна для каждой заявки или это дополнительное действие?
2. Для перехода к `Обращение` задача должна только существовать или быть закрыта?
3. Обращение обязательно для успешного закрытия сделки?
4. Может ли admin/owner пропускать этап, и нужна ли обязательная причина?

### Комиссии и прибыль

5. Комиссия контрагента — фиксированная сумма, процент или оба варианта?
6. Комиссия агента — фиксированная сумма, процент или оба?
7. Что точно означает «как продали клиенту»: сумма, курс, маржа, валютная пара, описание или комбинация?
8. Какова утверждённая формула прибыли?
9. В какой валюте сводить прибыль?
10. По какому FX rate нормализовать исторические сделки?

### Комиссия клиента

11. Checkbox означает только «нужно напомнить об оплате» или у клиента также хранится ставка/сумма?
12. Кто отмечает клиентскую комиссию оплаченной?
13. Нужно ли блокировать финальное закрытие сделки до выплаты комиссии, или reminder достаточно?

### Встречи

14. Кто может создавать встречи — любой сотрудник с доступом или только admin/owner?
15. Нужны ли Telegram-напоминания о встречах?

### Архив

16. Включать ли физические вложения в ежемесячный ZIP или сохранять только бизнес-данные и manifest?
17. Сколько месяцев/лет хранить архивы?

---

## 32. Что можно реализовывать без дополнительных уточнений

Следующие фундаментальные изменения можно начинать сразу:

- `SchemaVersion` + backup/migration framework;
- server-side request ownership policy;
- `mine/all` filters;
- read-only чужих заявок;
- новые status constants и transition validation;
- обязательный `CloseReason` для failed;
- отдельный `Comment`;
- `WorkflowStage` framework;
- `MiniAppSettings`;
- разные approval/payment chat IDs;
- `Client.CommissionEnabled`;
- idempotent client-commission reminder;
- central `ApplySaldoDelta`;
- materialized saldo reconciliation;
- Meetings storage/API skeleton;
- 3 custom `RateSlot`;
- Rates вместо Kanban в dock;
- monthly archive framework;
- Telegram delivery state/retry.

---

## 33. Изменения по существующим файлам

### `internal/appdb/store.go`

Добавить:

- `SchemaVersion`;
- новые Store fields;
- вызов миграций;
- backup/reconciliation orchestration.

Новую request/saldo/settings логику по возможности разместить в отдельных `.go` файлах того же package.

### `internal/appdb/catalog.go`

Добавить/изменить:

- `Client.CommissionEnabled`;
- commission rule, если будет утверждён;
- сущность `Agent`;
- связи Agent ↔ Request.

### `internal/appdb/deal.go`

Расширить:

- `Payment.Kind`;
- `Payment.Auto`;
- Telegram delivery metadata;
- permission-aware request-linked operations.

Workflow желательно держать отдельно, а не превращать `deal.go` в общий orchestrator.

### `internal/appdb/access.go`

Добавить минимум:

```text
meetings
analytics
```

### `internal/appdb/position.go`

Обновить:

- default position access;
- normalization;
- migration существующих должностей.

Финансовую аналитику не следует автоматически выдавать всем ролям без решения бизнеса.

### `internal/appdb/chats.go`

Оставить generic chat catalog, если он нужен существующему функционалу. Business routing хранить в `MiniAppSettings`.

### `internal/hub/api.go`

Добавить новые routes/actions и обязательно провести все `request_id` mutations через единую request permission policy.

По мере изменения рекомендуется разнести крупные handler-группы по нескольким файлам.

### `internal/hub/bot.go`

Добавить:

```go
SendBytesToGroup(...)
```

и при необходимости helper проверки/test-send чата.

### `internal/hub/cron.go`

Добавить minute-level проверку monthly archive schedule в `Europe/Moscow`.

Существующие cron-задачи по курсам/балансу/сальдо не менять без необходимости.

### `web/app.js`

Основной объём UI:

- request ownership/filtering;
- workflow;
- statuses;
- comment edit;
- economics;
- meetings;
- analytics;
- settings;
- archive;
- custom rates;
- dock.

Если build pipeline позволяет, новые крупные блоки желательно выделить в модули. Если нет — хотя бы не копировать одну и ту же settings/business logic в нескольких местах.

### `web/app.css`

Добавить стили для:

- read-only request;
- workflow progress;
- status filters;
- settings;
- meetings;
- analytics;
- custom rates;
- Telegram delivery state.

### `web/index.html`

Только необходимые контейнеры/подключения. Основную динамику сохранять в JS, как в текущей архитектуре.

### `dist/linux`, `dist/windows`

Не редактировать вручную. Пересобрать после прохождения тестов.

---

## 34. Definition of Done

Работы считаются завершёнными, когда одновременно выполняется всё ниже:

1. `miniapp.json` из baseline `998ccbb` загружается без ручного редактирования.
2. Перед первой миграцией создаётся backup.
3. Новые поля/настройки переживают restart.
4. Операционист не может изменить чужую заявку прямым HTTP-запросом.
5. Workflow нельзя обойти прямым HTTP-запросом.
6. Failed status нельзя записать без причины.
7. Сальдо не расходится между журналом, текущим балансом, UI, report и export.
8. Client commission reminder идемпотентен.
9. Approval и Payment используют разные configurable chat IDs.
10. Ошибка Telegram не приводит к потере бизнес-записи.
11. Telegram delivery можно retry без дублей.
12. Monthly archive не создаёт два scheduled snapshot за один месяц.
13. Manual `Сохранить сейчас` работает.
14. Шесть существующих курсов не сломаны.
15. Канбан доступен после замены dock-кнопки.
16. Unit/API/UI tests проходят.
17. `go test ./...` проходит.
18. Frontend test suite проходит.
19. Linux/Windows dist пересобраны только после успешной проверки исходников.
20. Перед production deployment создан backup `data/miniapp.json` и файлового vault.
21. После deployment выполнен smoke-test внутри реального Telegram Mini App.

---

## 35. Практическая последовательность задач для разработчика

```text
01. Зафиксировать спорные workflow/finance правила
02. SchemaVersion + backup/migration framework
03. RequestPermission policy
04. Закрыть все request-linked server-side access holes
05. Mine/All + status filters
06. Новые statuses + transition validation + failed reason
07. Отдельный Comment + edit rules
08. WorkflowStage + server-side gating
09. Agent + RequestEconomics
10. Client.CommissionEnabled
11. MiniAppSettings + chat configuration API/UI
12. Central ApplySaldoDelta + SaldoBalance
13. Rebuild/reconcile legacy saldo
14. Approval auto-send + saldo snapshot + counterparty export
15. Separate payment chat + delivery state/retry
16. Auto Payment(kind=client_commission) on success close
17. Meetings
18. 3 custom RateSlot + capabilities endpoint
19. Rates в dock вместо Kanban
20. Monthly archive в Documents
21. Activity heartbeat/aggregation
22. Accounting & Analytics
23. Audit/security regression
24. Full tests
25. Backup production
26. Deploy + migration
27. Telegram/Mini App smoke-test
```

---

## 36. Итог

Для данного набора требований главная задача — не просто добавить кнопки, а сформировать несколько общих доменных механизмов:

```text
RequestPermission
Request status machine
Request workflow stage
Structured RequestEconomics
Central ApplySaldoDelta
Materialized SaldoBalance
MiniAppSettings
Telegram delivery state
Schema migrations
Audit log
```

Если сначала реализовать эти фундаментальные слои, требования № 5, 6, 17, 21, 22, 24 и 43 будут использовать одну согласованную бизнес-логику. Это существенно снижает риск ситуации, когда UI показывает одно состояние, Telegram — другое, а `miniapp.json` содержит третье.

---

## 37. Источники анализа

Репозиторий:

`https://github.com/Artiz23/MINI_APP/tree/main`

Базовый коммит:

`https://github.com/Artiz23/MINI_APP/commit/998ccbbb30bd1300de0ee3fa49e25fc4e52efabc`

Основные изученные файлы на данном baseline:

```text
cmd/miniapp/main.go

internal/appdb/access.go
internal/appdb/catalog.go
internal/appdb/chats.go
internal/appdb/deal.go
internal/appdb/position.go
internal/appdb/request_archive.go
internal/appdb/store.go
internal/appdb/task.go
internal/appdb/vault.go

internal/hub/api.go
internal/hub/auth.go
internal/hub/bot.go
internal/hub/cron.go

web/app.js
web/app.css
web/index.html

README.md
```
