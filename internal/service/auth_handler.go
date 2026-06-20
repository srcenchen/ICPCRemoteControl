package service

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// LoginRequest holds credentials for login.
type LoginRequest struct {
	Password string `json:"password"`
}

// ChangePasswordRequest holds payload for password updates.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// AuthHandler manages admin authentication and session tokens.
type AuthHandler struct {
	settings *ServerSettings
	jwtKey   []byte
}

// NewAuthHandler creates a new AuthHandler with a static signing key.
func NewAuthHandler(settings *ServerSettings) *AuthHandler {
	return &AuthHandler{
		settings: settings,
		jwtKey:   []byte("icpc-remote-control-jwt-secret-key-signature"),
	}
}

// Login handles admin authentication (POST /api/auth/login).
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if !h.settings.VerifyPassword(req.Password) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "密码不正确"})
		return
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expirationTime),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Subject:   "admin",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(h.jwtKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
		return
	}

	// Set JWT as a HTTP cookie for automatic page/websocket handshakes authentication
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt_token",
		Value:    tokenString,
		Expires:  expirationTime,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{"token": tokenString})
}

// Logout invalidates the user cookie session (POST /api/auth/logout).
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt_token",
		Value:    "",
		Expires:  time.Unix(0, 0),
		Path:     "/",
		HttpOnly: true,
	})
	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// ChangePassword updates the admin password (POST /api/auth/password).
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.NewPassword == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "新密码不能为空"})
		return
	}

	if !h.settings.VerifyPassword(req.OldPassword) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "旧密码不正确"})
		return
	}

	if err := h.settings.SetAdminPassword(req.NewPassword); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "密码修改成功"})
}

// AuthMiddleware wraps a handler enforcing JWT verification.
func (h *AuthHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Exclude public paths
		if path == "/login.html" ||
			path == "/api/auth/login" ||
			strings.HasPrefix(path, "/assets/") ||
			strings.HasPrefix(path, "/broadcast/") ||
			path == "/ws/broadcast" {
			next.ServeHTTP(w, r)
			return
		}

		// Check Cookie first (for page visits / websockets)
		var tokenStr string
		cookie, err := r.Cookie("jwt_token")
		if err == nil {
			tokenStr = cookie.Value
		} else {
			// Check Authorization Header (for AJAX requests)
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if tokenStr == "" {
			h.unauthorized(w, r)
			return
		}

		claims := &jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			return h.jwtKey, nil
		})

		if err != nil || !token.Valid {
			h.unauthorized(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *AuthHandler) unauthorized(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
	} else {
		http.Redirect(w, r, "/login.html", http.StatusFound)
	}
}
