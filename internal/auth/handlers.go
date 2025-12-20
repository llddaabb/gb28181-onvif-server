package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"gb28181-onvif-server/internal/debug"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authManager *AuthManager
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(am *AuthManager) *AuthHandler {
	return &AuthHandler{authManager: am}
}

// HandleLogin 处理登录请求
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		h.jsonError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	user, err := h.authManager.Authenticate(req.Username, req.Password)
	if err != nil {
		debug.Warn("auth", "Login failed for user %s: %v", req.Username, err)
		h.jsonError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	token, err := h.authManager.GenerateToken(user)
	if err != nil {
		debug.Error("auth", "Failed to generate token: %v", err)
		h.jsonError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// 设置cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(h.authManager.config.TokenExpiry.Seconds()),
		SameSite: http.SameSiteLaxMode,
	})

	debug.Info("auth", "User %s logged in successfully", req.Username)

	// 返回用户信息（不包含密码）
	userCopy := *user
	userCopy.Password = ""

	h.jsonResponse(w, http.StatusOK, LoginResponse{
		Success: true,
		Token:   token,
		User:    &userCopy,
	})
}

// HandleLogout 处理登出请求
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// 清除cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "logged out successfully",
	})
}

// HandleGetCurrentUser 获取当前用户信息
func (h *AuthHandler) HandleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		h.jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	// 返回用户信息（不包含密码）
	userCopy := *user
	userCopy.Password = ""

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"user":    &userCopy,
	})
}

// HandleListUsers 列出所有用户（仅管理员）
func (h *AuthHandler) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromContext(r.Context())
	if claims == nil || claims.Role != RoleAdmin {
		h.jsonError(w, http.StatusForbidden, "admin access required")
		return
	}

	users := h.authManager.GetUsers()

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"users":   users,
	})
}

// HandleCreateUser 创建用户（仅管理员）
func (h *AuthHandler) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromContext(r.Context())
	if claims == nil || claims.Role != RoleAdmin {
		h.jsonError(w, http.StatusForbidden, "admin access required")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		h.jsonError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	role := Role(req.Role)
	if role == "" {
		role = RoleViewer
	}

	user, err := h.authManager.CreateUser(req.Username, req.Password, role)
	if err != nil {
		if err == ErrUserExists {
			h.jsonError(w, http.StatusConflict, "user already exists")
		} else {
			h.jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	debug.Info("auth", "User %s created by %s", req.Username, claims.Username)

	// 返回用户信息（不包含密码）
	userCopy := *user
	userCopy.Password = ""

	h.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"user":    &userCopy,
	})
}

// HandleUpdateUser 更新用户（仅管理员）
func (h *AuthHandler) HandleUpdateUser(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromContext(r.Context())
	if claims == nil || claims.Role != RoleAdmin {
		h.jsonError(w, http.StatusForbidden, "admin access required")
		return
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		h.jsonError(w, http.StatusBadRequest, "username is required")
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.authManager.UpdateUser(username, updates); err != nil {
		if err == ErrUserNotFound {
			h.jsonError(w, http.StatusNotFound, "user not found")
		} else {
			h.jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	debug.Info("auth", "User %s updated by %s", username, claims.Username)

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "user updated successfully",
	})
}

// HandleDeleteUser 删除用户（仅管理员）
func (h *AuthHandler) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromContext(r.Context())
	if claims == nil || claims.Role != RoleAdmin {
		h.jsonError(w, http.StatusForbidden, "admin access required")
		return
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		h.jsonError(w, http.StatusBadRequest, "username is required")
		return
	}

	// 不允许删除自己
	if username == claims.Username {
		h.jsonError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	if err := h.authManager.DeleteUser(username); err != nil {
		if err == ErrUserNotFound {
			h.jsonError(w, http.StatusNotFound, "user not found")
		} else {
			h.jsonError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	debug.Info("auth", "User %s deleted by %s", username, claims.Username)

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "user deleted successfully",
	})
}

// HandleChangePassword 修改密码
func (h *AuthHandler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		h.jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		h.jsonError(w, http.StatusBadRequest, "old_password and new_password are required")
		return
	}

	if len(req.NewPassword) < 6 {
		h.jsonError(w, http.StatusBadRequest, "new password must be at least 6 characters")
		return
	}

	if err := h.authManager.ChangePassword(user.Username, req.OldPassword, req.NewPassword); err != nil {
		if err == ErrInvalidCredentials {
			h.jsonError(w, http.StatusUnauthorized, "incorrect old password")
		} else {
			h.jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	debug.Info("auth", "User %s changed password", user.Username)

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "password changed successfully",
	})
}

// HandleRefreshToken 刷新令牌
func (h *AuthHandler) HandleRefreshToken(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		h.jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	token, err := h.authManager.GenerateToken(user)
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// 更新cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(h.authManager.config.TokenExpiry.Seconds()),
		SameSite: http.SameSiteLaxMode,
	})

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"token":      token,
		"expires_at": time.Now().Add(h.authManager.config.TokenExpiry),
	})
}

// HandleValidateToken 验证令牌
func (h *AuthHandler) HandleValidateToken(w http.ResponseWriter, r *http.Request) {
	token := ExtractTokenFromRequest(r)
	if token == "" {
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"valid": false,
		})
		return
	}

	claims, err := h.authManager.ValidateToken(token)
	if err != nil {
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"valid": false,
		})
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"valid":    true,
		"username": claims.Username,
		"role":     claims.Role,
	})
}

// jsonResponse 发送JSON响应
func (h *AuthHandler) jsonResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// jsonError 发送JSON错误响应
func (h *AuthHandler) jsonError(w http.ResponseWriter, statusCode int, message string) {
	h.jsonResponse(w, statusCode, map[string]interface{}{
		"success": false,
		"error":   message,
	})
}
