package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const testAdminPassword = "correct-horse-battery"

func newTestServer(t *testing.T) *Server {
	t.Helper()
	db, err := OpenDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := initializeAdminPassword(db, testAdminPassword); err != nil {
		t.Fatal(err)
	}
	return &Server{db: db, auth: newAuthState()}
}

func authTestRouter(s *Server) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/auth/status", s.authStatus)
	r.POST("/api/auth/login", s.login)
	r.POST("/api/auth/logout", s.logout)
	admin := r.Group("/api", s.requireAdmin)
	admin.POST("/auth/password", s.changeAdminPassword)
	admin.POST("/protected", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

func performJSON(r http.Handler, method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func loginCookie(t *testing.T, r http.Handler, password string) *http.Cookie {
	t.Helper()
	w := performJSON(r, http.MethodPost, "/api/auth/login", gin.H{"password": password}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", w.Code, w.Body.String())
	}
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == adminCookieName {
			return cookie
		}
	}
	t.Fatal("login did not set admin cookie")
	return nil
}

func TestInitializeAdminPasswordOnlyOnce(t *testing.T) {
	s := newTestServer(t)
	if err := initializeAdminPassword(s.db, "different-password-value"); err != nil {
		t.Fatal(err)
	}
	hash, err := s.adminPasswordHash()
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(testAdminPassword)); err != nil {
		t.Fatal("existing password was overwritten")
	}
}

func TestAdminSessionAndPasswordChange(t *testing.T) {
	s := newTestServer(t)
	r := authTestRouter(s)

	if w := performJSON(r, http.MethodPost, "/api/protected", nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("unprotected request status = %d", w.Code)
	}
	if w := performJSON(r, http.MethodPost, "/api/auth/login", gin.H{"password": "wrong-password"}, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d", w.Code)
	}

	cookie := loginCookie(t, r, testAdminPassword)
	key := sessionTokenHash(cookie.Value)
	s.auth.mu.Lock()
	s.auth.sessions[key] = adminSession{expiresAt: time.Now().Add(time.Second)}
	s.auth.mu.Unlock()

	if w := performJSON(r, http.MethodPost, "/api/protected", nil, cookie); w.Code != http.StatusOK {
		t.Fatalf("protected request status = %d, body = %s", w.Code, w.Body.String())
	}
	s.auth.mu.Lock()
	renewed := time.Until(s.auth.sessions[key].expiresAt)
	s.auth.mu.Unlock()
	if renewed < adminSessionTTL-time.Minute {
		t.Fatalf("session was not renewed, remaining = %s", renewed)
	}

	newPassword := "new-secure-password-2026"
	change := gin.H{
		"current_password": testAdminPassword,
		"new_password":     newPassword,
		"confirm_password": newPassword,
	}
	if w := performJSON(r, http.MethodPost, "/api/auth/password", change, cookie); w.Code != http.StatusOK {
		t.Fatalf("change password status = %d, body = %s", w.Code, w.Body.String())
	}
	if w := performJSON(r, http.MethodPost, "/api/protected", nil, cookie); w.Code != http.StatusUnauthorized {
		t.Fatalf("old session survived password change: %d", w.Code)
	}
	loginCookie(t, r, newPassword)
}

func TestLoginRateLimit(t *testing.T) {
	s := newTestServer(t)
	r := authTestRouter(s)
	for i := 1; i <= maxLoginFailures; i++ {
		w := performJSON(r, http.MethodPost, "/api/auth/login", gin.H{"password": "wrong-password"}, nil)
		want := http.StatusUnauthorized
		if i == maxLoginFailures {
			want = http.StatusTooManyRequests
		}
		if w.Code != want {
			t.Fatalf("attempt %d status = %d, want %d", i, w.Code, want)
		}
	}
	if w := performJSON(r, http.MethodPost, "/api/auth/login", gin.H{"password": testAdminPassword}, nil); w.Code != http.StatusTooManyRequests {
		t.Fatalf("locked login status = %d", w.Code)
	}
}
