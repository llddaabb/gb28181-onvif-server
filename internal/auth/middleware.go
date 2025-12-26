package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"gb28181-onvif-server/internal/debug"
)

// ContextKey 上下文键类型
type ContextKey string

const (
	// UserContextKey 用户信息上下文键
	UserContextKey ContextKey = "user"
	// ClaimsContextKey 声明信息上下文键
	ClaimsContextKey ContextKey = "claims"
)

// Middleware 认证中间件
type Middleware struct {
	authManager *AuthManager
	// 不需要认证的路径前缀
	publicPaths []string
	// 不需要认证的精确路径
	publicExactPaths []string
}

// NewMiddleware 创建认证中间件
func NewMiddleware(am *AuthManager) *Middleware {
	return &Middleware{
		authManager: am,
		publicPaths: []string{
			"/api/auth/login",
			"/api/auth/logout",
			"/api/health",
			"/api/status",
			"/api/stats",
			"/api/resources",
			"/api/logs",
			"/api/config",
			"/api/services",
			"/api/gb28181/",
			"/api/onvif/",
			"/api/stream/",
			"/api/channel/",
			"/api/recording/",
			"/api/storage/",
			"/api/ai/",
			"/api/zlm/", // ZLM 媒体服务器相关 API
			"/assets/",
			"/jessibuca/",
			"/favicon.ico",
		},
		publicExactPaths: []string{
			"/",
			"/login",
			"/api/auth/login",
		},
	}
}

// isPublicPath 检查是否为公开路径
func (m *Middleware) isPublicPath(path string) bool {
	// 检查精确匹配
	for _, p := range m.publicExactPaths {
		if path == p {
			return true
		}
	}

	// 检查前缀匹配
	for _, prefix := range m.publicPaths {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	// 静态资源文件
	if strings.HasSuffix(path, ".js") ||
		strings.HasSuffix(path, ".css") ||
		strings.HasSuffix(path, ".png") ||
		strings.HasSuffix(path, ".jpg") ||
		strings.HasSuffix(path, ".svg") ||
		strings.HasSuffix(path, ".ico") ||
		strings.HasSuffix(path, ".woff") ||
		strings.HasSuffix(path, ".woff2") ||
		strings.HasSuffix(path, ".ttf") {
		return true
	}

	return false
}

// Handler 返回中间件处理函数
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 如果认证未启用，直接放行
		if !m.authManager.IsEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path

		// 公开路径直接放行
		if m.isPublicPath(path) {
			next.ServeHTTP(w, r)
			return
		}

		// 提取令牌
		token := ExtractTokenFromRequest(r)
		if token == "" {
			m.unauthorized(w, r, "missing authentication token")
			return
		}

		// 验证令牌
		claims, err := m.authManager.ValidateToken(token)
		if err != nil {
			debug.Warn("auth", "Token validation failed: %v", err)
			m.unauthorized(w, r, "invalid or expired token")
			return
		}

		// 获取用户信息
		user, err := m.authManager.GetUser(claims.Username)
		if err != nil {
			m.unauthorized(w, r, "user not found")
			return
		}

		if !user.Enabled {
			m.forbidden(w, r, "user account is disabled")
			return
		}

		// 将用户信息添加到上下文
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		ctx = context.WithValue(ctx, ClaimsContextKey, claims)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole 角色要求中间件
func (m *Middleware) RequireRole(requiredRole Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 如果认证未启用，直接放行
			if !m.authManager.IsEnabled() {
				next.ServeHTTP(w, r)
				return
			}

			claims, ok := r.Context().Value(ClaimsContextKey).(*Claims)
			if !ok {
				m.unauthorized(w, r, "authentication required")
				return
			}

			if !HasPermission(claims.Role, requiredRole) {
				m.forbidden(w, r, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// unauthorized 返回未授权响应
func (m *Middleware) unauthorized(w http.ResponseWriter, r *http.Request, message string) {
	// API请求返回JSON
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   message,
			"code":    "UNAUTHORIZED",
		})
		return
	}

	// 页面请求重定向到登录页
	http.Redirect(w, r, "/login", http.StatusFound)
}

// forbidden 返回禁止访问响应
func (m *Middleware) forbidden(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
		"code":    "FORBIDDEN",
	})
}

// GetUserFromContext 从上下文获取用户信息
func GetUserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value(UserContextKey).(*User); ok {
		return user
	}
	return nil
}

// GetClaimsFromContext 从上下文获取声明信息
func GetClaimsFromContext(ctx context.Context) *Claims {
	if claims, ok := ctx.Value(ClaimsContextKey).(*Claims); ok {
		return claims
	}
	return nil
}
