package hub

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func signedAt(token string, at time.Time) string {
	user := `{"id":7,"first_name":"Test"}`
	date := strconv.FormatInt(at.Unix(), 10)
	data := "auth_date=" + date + "\nuser=" + user
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(data))
	return url.Values{"user": {user}, "auth_date": {date}, "hash": {hex.EncodeToString(mac.Sum(nil))}}.Encode()
}

func TestInitDataAge(t *testing.T) {
	const token = "test-token"
	if _, err := checkInitData(signedAt(token, time.Now()), token); err != nil {
		t.Fatal(err)
	}
	if _, err := checkInitData(signedAt(token, time.Now().Add(-25*time.Hour)), token); err == nil {
		t.Fatal("accepted expired initData")
	}
	if _, err := checkInitData(signedAt(token, time.Now().Add(6*time.Minute)), token); err == nil {
		t.Fatal("accepted future initData")
	}
}
