package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// CSRF: Double-Submit Cookie
func EnsureCSRFCookie(w http.ResponseWriter, r *http.Request) string {
	if v, err := r.Cookie("csrf"); err == nil && v.Value != "" {
		return v.Value
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	tok := base64.RawURLEncoding.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{Name: "csrf", Value: tok, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
	return tok
}

func VerifyCSRF(r *http.Request) bool {
	form := r.PostFormValue("_csrf")
	cv, err := r.Cookie("csrf")
	if err != nil || form == "" {
		return false
	}
	if len(form) != len(cv.Value) {
		return false
	}
	var same byte
	for i := 0; i < len(form); i++ {
		same |= form[i] ^ cv.Value[i]
	}
	return same == 0
}

// Auth cookie: HMAC-SHA256 signed
func SetAuthCookie(w http.ResponseWriter, secret string, uid int64, username string) {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	nonce := base64.RawURLEncoding.EncodeToString(b)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	payload := []byte(strconv.FormatInt(uid, 10) + "|" + ts + "|" + username + "|" + nonce)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	sig := mac.Sum(nil)
	val := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig)
	http.SetCookie(w, &http.Cookie{Name: "auth", Value: val, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func GetAuthCookie(r *http.Request, secret string) (int64, string, bool) {
	ck, err := r.Cookie("auth")
	if err != nil || ck.Value == "" {
		return 0, "", false
	}
	parts := strings.SplitN(ck.Value, ".", 2)
	if len(parts) != 2 {
		return 0, "", false
	}
	payload, err1 := base64.RawURLEncoding.DecodeString(parts[0])
	sig, err2 := base64.RawURLEncoding.DecodeString(parts[1])
	if err1 != nil || err2 != nil {
		return 0, "", false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	exp := mac.Sum(nil)
	if !hmac.Equal(exp, sig) {
		return 0, "", false
	}
	fields := strings.Split(string(payload), "|")
	if len(fields) < 4 {
		return 0, "", false
	}
	uid, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, "", false
	}
	ts, _ := strconv.ParseInt(fields[1], 10, 64)
	if time.Since(time.Unix(ts, 0)) > 7*24*time.Hour {
		return 0, "", false
	}
	return uid, fields[2], true
}

func ClearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "auth", Value: "", Path: "/", HttpOnly: true, MaxAge: -1, SameSite: http.SameSiteLaxMode})
}
