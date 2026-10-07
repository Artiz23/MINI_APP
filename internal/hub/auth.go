package hub

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"tg-bot-orh3/MINI_APP/internal/secrets"
)

type tgUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

func checkInitData(initData, token string) (tgUser, error) {
	var u tgUser
	if strings.TrimSpace(initData) == "" {
		return u, fmt.Errorf("нет initData")
	}
	vals, err := url.ParseQuery(initData)
	if err != nil {
		return u, err
	}
	hash := vals.Get("hash")
	if hash == "" {
		return u, fmt.Errorf("нет hash")
	}
	var pairs []string
	for k, v := range vals {
		if k == "hash" || len(v) == 0 {
			continue
		}
		pairs = append(pairs, k+"="+v[0])
	}
	sort.Strings(pairs)
	dataCheck := strings.Join(pairs, "\n")
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(dataCheck))
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(hash)) {
		return u, fmt.Errorf("подпись Mini App не совпала")
	}
	if err := json.Unmarshal([]byte(vals.Get("user")), &u); err != nil {
		return u, fmt.Errorf("user: %w", err)
	}
	if u.ID == 0 {
		return u, fmt.Errorf("пустой user id")
	}
	return u, nil
}

func (u tgUser) Display() string {
	n := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if n == "" {
		n = u.Username
	}
	if n == "" {
		n = strconv.FormatInt(u.ID, 10)
	}
	return n
}

func isOwner(id int64) bool { return secrets.IsMain(id) }
