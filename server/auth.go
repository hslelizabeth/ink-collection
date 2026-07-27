package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const (
	adminPasswordKey = "admin_password_hash"
	adminCookieName  = "ink_admin_session"
	adminSessionTTL  = 30 * time.Minute
	loginLockTime    = 10 * time.Minute
	maxLoginFailures = 5
)

type adminSession struct {
	expiresAt time.Time
}

type loginAttempt struct {
	failures    int
	lockedUntil time.Time
}

type authState struct {
	mu       sync.Mutex
	sessions map[string]adminSession
	attempts map[string]loginAttempt
}

func newAuthState() *authState {
	return &authState{
		sessions: make(map[string]adminSession),
		attempts: make(map[string]loginAttempt),
	}
}

func (d *DB) setting(key string) (string, error) {
	var value string
	err := d.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	return value, err
}

func (d *DB) setSetting(key, value string) error {
	_, err := d.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func validateAdminPassword(password string) error {
	if utf8.RuneCountInString(password) < 12 {
		return errors.New("管理密码至少需要 12 个字符")
	}
	if len(password) > 72 {
		return errors.New("管理密码不能超过 72 个字节")
	}
	return nil
}

func initializeAdminPassword(db *DB, initialPassword string) error {
	_, err := db.setting(adminPasswordKey)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read admin password: %w", err)
	}
	if initialPassword == "" {
		return errors.New("首次启动必须通过 ADMIN_INITIAL_PASSWORD 设置管理密码（至少 12 个字符）")
	}
	if err := validateAdminPassword(initialPassword); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(initialPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	if err := db.setSetting(adminPasswordKey, string(hash)); err != nil {
		return fmt.Errorf("save admin password: %w", err)
	}
	return nil
}

func randomSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func sessionTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func requestUsesHTTPS(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	proto := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Proto"), ",")[0])
	return strings.EqualFold(proto, "https")
}

func setAdminCookie(c *gin.Context, token string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     adminCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   requestUsesHTTPS(c),
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *Server) adminPasswordHash() ([]byte, error) {
	hash, err := s.db.setting(adminPasswordKey)
	return []byte(hash), err
}

func (s *Server) authenticated(c *gin.Context, renew bool) bool {
	token, err := c.Cookie(adminCookieName)
	if err != nil || token == "" {
		return false
	}
	key := sessionTokenHash(token)
	now := time.Now()

	s.auth.mu.Lock()
	session, ok := s.auth.sessions[key]
	if ok && !session.expiresAt.After(now) {
		delete(s.auth.sessions, key)
		ok = false
	}
	if ok && renew {
		session.expiresAt = now.Add(adminSessionTTL)
		s.auth.sessions[key] = session
	}
	s.auth.mu.Unlock()

	if ok && renew {
		setAdminCookie(c, token, int(adminSessionTTL.Seconds()))
	}
	return ok
}

func (s *Server) requireAdmin(c *gin.Context) {
	if !s.authenticated(c, true) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "管理会话已失效，请重新验证"})
		return
	}
	c.Next()
}

func (s *Server) authStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"authenticated": s.authenticated(c, true),
		"expires_in":    int(adminSessionTTL.Seconds()),
	})
}

func (s *Server) loginLocked(ip string) (bool, time.Duration) {
	s.auth.mu.Lock()
	defer s.auth.mu.Unlock()
	attempt := s.auth.attempts[ip]
	remaining := time.Until(attempt.lockedUntil)
	if remaining > 0 {
		return true, remaining
	}
	if !attempt.lockedUntil.IsZero() {
		delete(s.auth.attempts, ip)
	}
	return false, 0
}

func (s *Server) recordLoginFailure(ip string) bool {
	s.auth.mu.Lock()
	defer s.auth.mu.Unlock()
	attempt := s.auth.attempts[ip]
	attempt.failures++
	if attempt.failures >= maxLoginFailures {
		attempt.lockedUntil = time.Now().Add(loginLockTime)
	}
	s.auth.attempts[ip] = attempt
	return !attempt.lockedUntil.IsZero()
}

func (s *Server) clearLoginFailures(ip string) {
	s.auth.mu.Lock()
	delete(s.auth.attempts, ip)
	s.auth.mu.Unlock()
}

func remoteIP(c *gin.Context) string {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err == nil {
		return host
	}
	return c.Request.RemoteAddr
}

func (s *Server) login(c *gin.Context) {
	ip := remoteIP(c)
	if locked, remaining := s.loginLocked(ip); locked {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":       "登录尝试过多，请稍后再试",
			"retry_after": int(remaining.Seconds()) + 1,
		})
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	hash, err := s.adminPasswordHash()
	if err != nil || bcrypt.CompareHashAndPassword(hash, []byte(in.Password)) != nil {
		locked := s.recordLoginFailure(ip)
		status := http.StatusUnauthorized
		message := "访问密码错误"
		if locked {
			status = http.StatusTooManyRequests
			message = "登录尝试过多，请 10 分钟后再试"
		}
		c.JSON(status, gin.H{"error": message})
		return
	}

	token, err := randomSessionToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建管理会话失败"})
		return
	}
	s.auth.mu.Lock()
	s.auth.sessions[sessionTokenHash(token)] = adminSession{expiresAt: time.Now().Add(adminSessionTTL)}
	s.auth.mu.Unlock()
	s.clearLoginFailures(ip)
	setAdminCookie(c, token, int(adminSessionTTL.Seconds()))
	c.JSON(http.StatusOK, gin.H{"authenticated": true, "expires_in": int(adminSessionTTL.Seconds())})
}

func (s *Server) logout(c *gin.Context) {
	if token, err := c.Cookie(adminCookieName); err == nil && token != "" {
		s.auth.mu.Lock()
		delete(s.auth.sessions, sessionTokenHash(token))
		s.auth.mu.Unlock()
	}
	setAdminCookie(c, "", -1)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) changeAdminPassword(c *gin.Context) {
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if in.NewPassword != in.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "两次输入的新密码不一致"})
		return
	}
	if err := validateAdminPassword(in.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hash, err := s.adminPasswordHash()
	if err != nil || bcrypt.CompareHashAndPassword(hash, []byte(in.CurrentPassword)) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前密码错误"})
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新密码失败"})
		return
	}
	if err := s.db.setSetting(adminPasswordKey, string(newHash)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新密码失败"})
		return
	}
	s.auth.mu.Lock()
	clear(s.auth.sessions)
	s.auth.mu.Unlock()
	setAdminCookie(c, "", -1)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
