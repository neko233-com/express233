package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

const jwtCookie = "express233_jwt"

type jwtClaims struct {
	UserID     int64  `json:"uid"`
	Username   string `json:"usr"`
	IsAdmin    bool   `json:"adm"`
	TenantID   int64  `json:"tid"`
	TenantSlug string `json:"tsl"`
	Exp        int64  `json:"exp"`
}

type jwtAuth struct {
	secret []byte
}

func newJWTAuth() *jwtAuth {
	sec := os.Getenv("EXPRESS233_JWT_SECRET")
	if sec == "" {
		sec = "express233-dev-secret-change-in-production"
	}
	return &jwtAuth{secret: []byte(sec)}
}

func (j *jwtAuth) sign(sess session, ttl time.Duration) (string, error) {
	if j == nil {
		return "", errors.New("jwt not configured")
	}
	c := jwtClaims{
		UserID:     sess.UserID,
		Username:   sess.Username,
		IsAdmin:    sess.IsAdmin,
		TenantID:   sess.TenantID,
		TenantSlug: sess.TenantSlug,
		Exp:        time.Now().Add(ttl).Unix(),
	}
	return j.encode(c)
}

func (j *jwtAuth) verify(token string) (jwtClaims, error) {
	var zero jwtClaims
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return zero, errors.New("invalid token")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return zero, err
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return zero, err
	}
	mac := hmac.New(sha256.New, j.secret)
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return zero, errors.New("invalid signature")
	}
	var c jwtClaims
	if err := json.Unmarshal(payload, &c); err != nil {
		return zero, err
	}
	if time.Now().Unix() > c.Exp {
		return zero, errors.New("token expired")
	}
	return c, nil
}

func (j *jwtAuth) encode(c jwtClaims) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, j.secret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig, nil
}

func (c jwtClaims) session() session {
	return session{
		UserID:     c.UserID,
		Username:   c.Username,
		IsAdmin:    c.IsAdmin,
		TenantID:   c.TenantID,
		TenantSlug: c.TenantSlug,
		Expires:    time.Unix(c.Exp, 0),
	}
}

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func (s *Server) setJWTCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     jwtCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 7,
	})
}

func (s *Server) clearJWTCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     jwtCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func (s *Server) sessionFromRequest(r *http.Request) (session, bool) {
	if tok := extractBearerToken(r); tok != "" {
		if c, err := s.jwt.verify(tok); err == nil {
			return c.session(), true
		}
	}
	if c, err := r.Cookie(jwtCookie); err == nil && c.Value != "" {
		if claims, err := s.jwt.verify(c.Value); err == nil {
			return claims.session(), true
		}
	}
	// 兼容旧 session
	return s.sessions.getFromRequest(r)
}

// getFromRequest 旧版 cookie session。
func (ss *sessionStore) getFromRequest(r *http.Request) (session, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return session{}, false
	}
	return ss.get(c.Value)
}
