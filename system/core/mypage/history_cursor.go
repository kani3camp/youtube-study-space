package mypage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"app.modules/core/timeutil"
)

const historyCursorTTL = 15 * time.Minute

type HistoryQuery struct {
	From, To time.Time
	PageSize int
	Cursor   string
}

type historyCursor struct {
	Owner                     string `json:"owner"`
	Authorization             string `json:"authorization"`
	From, To                  string
	PageSize                  int
	AsOf, ExpiresAt, AfterEnd time.Time
	AfterID                   string
}

type historyCipher struct {
	aead     cipher.AEAD
	ownerKey []byte
	aad      []byte
}

func deriveHistoryKey(secret []byte, environment, purpose string) []byte {
	mac := hmac.New(sha256.New, secret)
	if _, err := mac.Write([]byte("mypage.history.v1\x00" + environment + "\x00" + purpose)); err != nil {
		return nil
	}
	return mac.Sum(nil)
}

func newHistoryCipher(secret []byte, environment string) (*historyCipher, error) {
	if len(secret) == 0 || environment != "development" {
		return nil, apiError("FEATURE_UNAVAILABLE")
	}
	block, err := aes.NewCipher(deriveHistoryKey(secret, environment, "cursor-encryption"))
	if err != nil {
		return nil, fmt.Errorf("create history cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create history authenticated cursor: %w", err)
	}
	return &historyCipher{aead: aead, ownerKey: deriveHistoryKey(secret, environment, "owner-binding"), aad: []byte("mypage-history-v1\x00" + environment)}, nil
}

func (c *historyCipher) owner(uid string) string {
	mac := hmac.New(sha256.New, c.ownerKey)
	if _, err := mac.Write([]byte(uid)); err != nil {
		return ""
	}
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *historyCipher) seal(value historyCursor) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode history cursor: %w", err)
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("create history cursor nonce: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(c.aead.Seal(nonce, nonce, body, c.aad)), nil
}

func (c *historyCipher) open(token, uid, authorization string, q HistoryQuery, now time.Time) (historyCursor, error) {
	var value historyCursor
	if len(token) > 2048 {
		return value, apiError("INVALID_CURSOR")
	}
	encoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(encoded) < c.aead.NonceSize()+c.aead.Overhead() {
		return value, apiError("INVALID_CURSOR")
	}
	nonce, body := encoded[:c.aead.NonceSize()], encoded[c.aead.NonceSize():]
	plain, err := c.aead.Open(nil, nonce, body, c.aad)
	if err != nil {
		return value, apiError("INVALID_CURSOR")
	}
	decoder := json.NewDecoder(strings.NewReader(string(plain)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || !hmac.Equal([]byte(value.Owner), []byte(c.owner(uid))) || value.Authorization != authorization || value.From != q.From.Format("2006-01-02") || value.To != q.To.Format("2006-01-02") || value.PageSize != q.PageSize || value.AsOf.IsZero() || value.AsOf.After(now) || !now.Before(value.ExpiresAt) || value.ExpiresAt.Sub(value.AsOf) > historyCursorTTL || value.AfterID == "" || len(value.AfterID) > 256 || strings.Contains(value.AfterID, "/") || value.AfterEnd.Before(q.From) || !value.AfterEnd.Before(q.To) {
		return historyCursor{}, apiError("INVALID_CURSOR")
	}
	return value, nil
}

func parseHistoryQuery(raw string) (HistoryQuery, error) {
	var q HistoryQuery
	if len(raw) > 3072 {
		return q, apiError("INVALID_REQUEST")
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return q, apiError("INVALID_REQUEST")
	}
	for key, items := range values {
		if (key != "from" && key != "to" && key != "limit" && key != "cursor") || len(items) != 1 {
			return q, apiError("INVALID_REQUEST")
		}
	}
	q.From, err = time.ParseInLocation("2006-01-02", values.Get("from"), timeutil.JapanLocation())
	if err != nil || q.From.Year() < 1 {
		return q, apiError("INVALID_REQUEST")
	}
	q.To, err = time.ParseInLocation("2006-01-02", values.Get("to"), timeutil.JapanLocation())
	if err != nil || !q.To.After(q.From) || q.To.Sub(q.From) > 31*24*time.Hour {
		return q, apiError("INVALID_REQUEST")
	}
	q.PageSize = 50
	if value := values.Get("limit"); value != "" {
		q.PageSize, err = strconv.Atoi(value)
		if err != nil || q.PageSize < 1 || q.PageSize > 100 {
			return q, apiError("INVALID_REQUEST")
		}
	}
	q.Cursor = values.Get("cursor")
	if len(q.Cursor) > 2048 {
		return q, apiError("INVALID_CURSOR")
	}
	return q, nil
}
