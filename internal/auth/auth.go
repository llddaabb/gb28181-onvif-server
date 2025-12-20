package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// 错误定义
var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
)

// Role 用户角色
type Role string

const (
	RoleAdmin    Role = "admin"    // 管理员，拥有所有权限
	RoleOperator Role = "operator" // 操作员，可以控制设备
	RoleViewer   Role = "viewer"   // 观看者，只能查看
)

// User 用户信息
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"-"` // 不在API响应中暴露
	Role      Role      `json:"role"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	LastLogin time.Time `json:"last_login,omitempty"`
}

// userPersist 用于持久化的用户结构（包含密码）
type userPersist struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"password"` // 持久化时保存密码
	Role      Role      `json:"role"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	LastLogin time.Time `json:"last_login,omitempty"`
}

// Claims JWT声明
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     Role   `json:"role"`
	jwt.RegisteredClaims
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	User    *User  `json:"user,omitempty"`
	Error   string `json:"error,omitempty"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	Enable          bool          `yaml:"Enable" json:"enable"`
	JWTSecret       string        `yaml:"JWTSecret" json:"jwt_secret,omitempty"`
	TokenExpiry     time.Duration `yaml:"TokenExpiry" json:"token_expiry"`
	UsersFile       string        `yaml:"UsersFile" json:"users_file"`
	DefaultAdmin    string        `yaml:"DefaultAdmin" json:"default_admin"`
	DefaultPassword string        `yaml:"DefaultPassword" json:"-"`
}

// DefaultAuthConfig 默认认证配置
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		Enable:          true,
		JWTSecret:       generateRandomSecret(),
		TokenExpiry:     24 * time.Hour,
		UsersFile:       "configs/users.json",
		DefaultAdmin:    "admin",
		DefaultPassword: "admin123",
	}
}

// AuthManager 认证管理器
type AuthManager struct {
	config    *AuthConfig
	users     map[string]*User
	mutex     sync.RWMutex
	jwtSecret []byte
}

// NewAuthManager 创建认证管理器
func NewAuthManager(config *AuthConfig) *AuthManager {
	if config == nil {
		config = DefaultAuthConfig()
	}

	if config.JWTSecret == "" {
		config.JWTSecret = generateRandomSecret()
	}

	am := &AuthManager{
		config:    config,
		users:     make(map[string]*User),
		jwtSecret: []byte(config.JWTSecret),
	}

	// 加载用户数据
	am.loadUsers()

	// 确保有默认管理员账户
	am.ensureDefaultAdmin()

	return am
}

// generateRandomSecret 生成随机密钥
func generateRandomSecret() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// loadUsers 从文件加载用户
func (am *AuthManager) loadUsers() error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	data, err := os.ReadFile(am.config.UsersFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在是正常的
		}
		return err
	}

	var users []*userPersist
	if err := json.Unmarshal(data, &users); err != nil {
		return err
	}

	for _, up := range users {
		user := &User{
			ID:        up.ID,
			Username:  up.Username,
			Password:  up.Password,
			Role:      up.Role,
			Enabled:   up.Enabled,
			CreatedAt: up.CreatedAt,
			UpdatedAt: up.UpdatedAt,
			LastLogin: up.LastLogin,
		}
		am.users[user.Username] = user
	}

	return nil
}

// saveUsers 保存用户到文件
func (am *AuthManager) saveUsers() error {
	// 注意：调用此函数时，调用者应该已经持有锁或者在安全的上下文中
	users := make([]*userPersist, 0, len(am.users))
	for _, user := range am.users {
		up := &userPersist{
			ID:        user.ID,
			Username:  user.Username,
			Password:  user.Password,
			Role:      user.Role,
			Enabled:   user.Enabled,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			LastLogin: user.LastLogin,
		}
		users = append(users, up)
	}

	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(am.config.UsersFile, data, 0600)
}

// ensureDefaultAdmin 确保有默认管理员账户
func (am *AuthManager) ensureDefaultAdmin() {
	am.mutex.RLock()
	_, exists := am.users[am.config.DefaultAdmin]
	am.mutex.RUnlock()

	if !exists {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(am.config.DefaultPassword), bcrypt.DefaultCost)
		user := &User{
			ID:        generateUserID(),
			Username:  am.config.DefaultAdmin,
			Password:  string(hashedPassword),
			Role:      RoleAdmin,
			Enabled:   true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		am.mutex.Lock()
		am.users[user.Username] = user
		am.mutex.Unlock()

		am.saveUsers()
	}
}

// generateUserID 生成用户ID
func generateUserID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Authenticate 验证用户名密码
func (am *AuthManager) Authenticate(username, password string) (*User, error) {
	am.mutex.RLock()
	user, exists := am.users[username]
	am.mutex.RUnlock()

	if !exists {
		return nil, ErrInvalidCredentials
	}

	if !user.Enabled {
		return nil, ErrForbidden
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// 更新最后登录时间
	am.mutex.Lock()
	user.LastLogin = time.Now()
	am.mutex.Unlock()
	am.saveUsers()

	return user, nil
}

// GenerateToken 生成JWT令牌
func (am *AuthManager) GenerateToken(user *User) (string, error) {
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(am.config.TokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "gb28181-onvif-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(am.jwtSecret)
}

// ValidateToken 验证JWT令牌
func (am *AuthManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return am.jwtSecret, nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// GetUser 获取用户信息
func (am *AuthManager) GetUser(username string) (*User, error) {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	user, exists := am.users[username]
	if !exists {
		return nil, ErrUserNotFound
	}

	return user, nil
}

// GetUsers 获取所有用户
func (am *AuthManager) GetUsers() []*User {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	users := make([]*User, 0, len(am.users))
	for _, user := range am.users {
		// 创建副本，不暴露密码
		userCopy := *user
		userCopy.Password = ""
		users = append(users, &userCopy)
	}

	return users
}

// CreateUser 创建用户
func (am *AuthManager) CreateUser(username, password string, role Role) (*User, error) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if _, exists := am.users[username]; exists {
		return nil, ErrUserExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &User{
		ID:        generateUserID(),
		Username:  username,
		Password:  string(hashedPassword),
		Role:      role,
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	am.users[username] = user
	am.saveUsers()

	return user, nil
}

// UpdateUser 更新用户
func (am *AuthManager) UpdateUser(username string, updates map[string]interface{}) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	user, exists := am.users[username]
	if !exists {
		return ErrUserNotFound
	}

	if password, ok := updates["password"].(string); ok && password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		user.Password = string(hashedPassword)
	}

	if role, ok := updates["role"].(string); ok {
		user.Role = Role(role)
	}

	if enabled, ok := updates["enabled"].(bool); ok {
		user.Enabled = enabled
	}

	user.UpdatedAt = time.Now()
	am.saveUsers()

	return nil
}

// DeleteUser 删除用户
func (am *AuthManager) DeleteUser(username string) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if _, exists := am.users[username]; !exists {
		return ErrUserNotFound
	}

	// 不允许删除最后一个管理员
	adminCount := 0
	for _, user := range am.users {
		if user.Role == RoleAdmin && user.Enabled {
			adminCount++
		}
	}

	if am.users[username].Role == RoleAdmin && adminCount <= 1 {
		return errors.New("cannot delete the last admin user")
	}

	delete(am.users, username)
	am.saveUsers()

	return nil
}

// ChangePassword 修改密码
func (am *AuthManager) ChangePassword(username, oldPassword, newPassword string) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	user, exists := am.users[username]
	if !exists {
		return ErrUserNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return ErrInvalidCredentials
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.Password = string(hashedPassword)
	user.UpdatedAt = time.Now()
	am.saveUsers()

	return nil
}

// ExtractTokenFromRequest 从请求中提取令牌
func ExtractTokenFromRequest(r *http.Request) string {
	// 从 Authorization 头提取
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	// 从 cookie 提取
	cookie, err := r.Cookie("auth_token")
	if err == nil {
		return cookie.Value
	}

	// 从查询参数提取
	return r.URL.Query().Get("token")
}

// HasPermission 检查角色是否有权限
func HasPermission(role Role, requiredRole Role) bool {
	roleLevel := map[Role]int{
		RoleViewer:   1,
		RoleOperator: 2,
		RoleAdmin:    3,
	}

	return roleLevel[role] >= roleLevel[requiredRole]
}

// IsEnabled 检查认证是否启用
func (am *AuthManager) IsEnabled() bool {
	return am.config.Enable
}
