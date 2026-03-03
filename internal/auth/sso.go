package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"github.com/kerbos/k8sinsight/internal/store/model"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// SSOConfig 从 system_settings 读取的 SSO 配置
type SSOConfig struct {
	Enabled        bool
	ProviderName   string
	ClientID       string
	ClientSecret   string
	IssuerURL      string
	RedirectURI    string
	Scopes         []string
	AutoCreateUser bool
	DefaultRole    string
}

// SSOConfigResponse 返回给前端的 SSO 配置（不含敏感信息）
type SSOConfigResponse struct {
	Enabled      bool   `json:"enabled"`
	ProviderName string `json:"providerName"`
}

// stateEntry state/nonce 存储条目
type stateEntry struct {
	nonce     string
	expiresAt time.Time
}

// StateStore 内存 state/nonce 存储（含自动过期清理）
type StateStore struct {
	mu      sync.Mutex
	entries map[string]stateEntry
	stopCh  chan struct{}
}

// NewStateStore 创建 state 存储并启动自动清理
func NewStateStore() *StateStore {
	s := &StateStore{
		entries: make(map[string]stateEntry),
		stopCh:  make(chan struct{}),
	}
	go s.cleanup()
	return s
}

// Save 保存 state/nonce 对
func (s *StateStore) Save(state, nonce string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[state] = stateEntry{
		nonce:     nonce,
		expiresAt: time.Now().Add(ttl),
	}
}

// Consume 消费 state 并返回对应的 nonce（一次性使用）
func (s *StateStore) Consume(state string) (nonce string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, exists := s.entries[state]
	if !exists {
		return "", false
	}
	delete(s.entries, state)
	if time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.nonce, true
}

// cleanup 定期清理过期条目
func (s *StateStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for k, v := range s.entries {
				if now.After(v.expiresAt) {
					delete(s.entries, k)
				}
			}
			s.mu.Unlock()
		case <-s.stopCh:
			return
		}
	}
}

// SSOService SSO 认证服务
type SSOService struct {
	settingRepo  repository.SettingRepository
	userRepo     repository.UserRepository
	roleRepo     repository.RoleRepository
	tokenService *TokenService
	stateStore   *StateStore
	logger       *zap.Logger

	// OIDC provider 缓存
	mu           sync.Mutex
	cachedIssuer string
	provider     *oidc.Provider
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
}

// NewSSOService 创建 SSO 认证服务
func NewSSOService(
	settingRepo repository.SettingRepository,
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	tokenService *TokenService,
	logger *zap.Logger,
) *SSOService {
	return &SSOService{
		settingRepo:  settingRepo,
		userRepo:     userRepo,
		roleRepo:     roleRepo,
		tokenService: tokenService,
		stateStore:   NewStateStore(),
		logger:       logger.Named("auth.sso"),
	}
}

// SSO 配置键及默认值
var ssoSettingKeys = []string{
	"sso_enabled",
	"sso_provider_name",
	"sso_client_id",
	"sso_client_secret",
	"sso_issuer_url",
	"sso_redirect_uri",
	"sso_scopes",
	"sso_auto_create_user",
	"sso_default_role",
}

// loadConfig 从 system_settings 读取 SSO 配置
func (s *SSOService) loadConfig(ctx context.Context) (*SSOConfig, error) {
	settings, err := s.settingRepo.BatchGet(ctx, ssoSettingKeys)
	if err != nil {
		return nil, fmt.Errorf("读取 SSO 配置失败: %w", err)
	}

	scopes := []string{"openid", "profile", "email"}
	if v := settings["sso_scopes"]; v != "" {
		scopes = strings.Split(v, ",")
		for i := range scopes {
			scopes[i] = strings.TrimSpace(scopes[i])
		}
	}

	autoCreate := true
	if settings["sso_auto_create_user"] == "false" {
		autoCreate = false
	}

	defaultRole := settings["sso_default_role"]
	if defaultRole == "" {
		defaultRole = "viewer"
	}

	return &SSOConfig{
		Enabled:        settings["sso_enabled"] == "true",
		ProviderName:   settings["sso_provider_name"],
		ClientID:       settings["sso_client_id"],
		ClientSecret:   settings["sso_client_secret"],
		IssuerURL:      settings["sso_issuer_url"],
		RedirectURI:    settings["sso_redirect_uri"],
		Scopes:         scopes,
		AutoCreateUser: autoCreate,
		DefaultRole:    defaultRole,
	}, nil
}

// ensureProvider 初始化或刷新 OIDC Provider 缓存
func (s *SSOService) ensureProvider(ctx context.Context, cfg *SSOConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.provider != nil && s.cachedIssuer == cfg.IssuerURL {
		// issuer 未变，复用缓存
		return nil
	}

	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return fmt.Errorf("初始化 OIDC Provider 失败: %w", err)
	}

	s.provider = provider
	s.cachedIssuer = cfg.IssuerURL
	s.oauth2Config = &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.Scopes,
	}
	s.verifier = provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})

	return nil
}

// GetSSOConfig 获取 SSO 状态（前端使用）
func (s *SSOService) GetSSOConfig(ctx context.Context) (*SSOConfigResponse, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &SSOConfigResponse{
		Enabled:      cfg.Enabled,
		ProviderName: cfg.ProviderName,
	}, nil
}

// generateRandom 生成随机十六进制字符串
func generateRandom(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GetAuthURL 生成 OIDC 授权 URL
func (s *SSOService) GetAuthURL(ctx context.Context) (string, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return "", err
	}
	if !cfg.Enabled {
		return "", fmt.Errorf("SSO 未启用")
	}
	if cfg.ClientID == "" || cfg.IssuerURL == "" {
		return "", fmt.Errorf("SSO 配置不完整")
	}

	if err := s.ensureProvider(ctx, cfg); err != nil {
		return "", err
	}

	state, err := generateRandom(16)
	if err != nil {
		return "", fmt.Errorf("生成 state 失败: %w", err)
	}
	nonce, err := generateRandom(16)
	if err != nil {
		return "", fmt.Errorf("生成 nonce 失败: %w", err)
	}

	s.stateStore.Save(state, nonce, 10*time.Minute)

	s.mu.Lock()
	authURL := s.oauth2Config.AuthCodeURL(state, oidc.Nonce(nonce))
	s.mu.Unlock()

	return authURL, nil
}

// SSOTokenResponse SSO 登录成功后返回的令牌
type SSOTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// HandleCallback 处理 OIDC 回调
func (s *SSOService) HandleCallback(ctx context.Context, code, state string) (*SSOTokenResponse, error) {
	// 1. 验证 state
	nonce, ok := s.stateStore.Consume(state)
	if !ok {
		return nil, fmt.Errorf("无效或过期的 state")
	}

	// 2. 加载配置
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, fmt.Errorf("SSO 未启用")
	}

	if err := s.ensureProvider(ctx, cfg); err != nil {
		return nil, err
	}

	// 3. 交换 authorization code
	s.mu.Lock()
	oauth2Cfg := s.oauth2Config
	verifier := s.verifier
	s.mu.Unlock()

	oauth2Token, err := oauth2Cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("交换授权码失败: %w", err)
	}

	// 4. 提取并验证 ID Token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("响应中缺少 id_token")
	}

	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("验证 ID Token 失败: %w", err)
	}

	// 5. 验证 nonce
	if idToken.Nonce != nonce {
		return nil, fmt.Errorf("nonce 不匹配")
	}

	// 6. 提取用户信息
	var claims struct {
		Sub               string `json:"sub"`
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
		Name              string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("解析用户信息失败: %w", err)
	}

	// 7. 匹配或创建用户
	user, err := s.matchOrCreateUser(ctx, claims.Sub, claims.PreferredUsername, claims.Email, claims.Name, cfg)
	if err != nil {
		return nil, err
	}

	// 8. 检查用户是否被禁用
	if !user.IsActive {
		return nil, fmt.Errorf("用户已被禁用")
	}

	// 9. 生成 JWT
	accessToken, err := s.tokenService.GenerateAccessToken(user)
	if err != nil {
		return nil, fmt.Errorf("生成 access token 失败: %w", err)
	}

	rawRefresh, hashRefresh, err := s.tokenService.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("生成 refresh token 失败: %w", err)
	}

	rt := s.tokenService.NewRefreshTokenModel(user.ID, hashRefresh)
	if err := s.userRepo.CreateRefreshToken(ctx, rt); err != nil {
		return nil, fmt.Errorf("保存 refresh token 失败: %w", err)
	}

	s.logger.Info("SSO 登录成功",
		zap.String("username", user.Username),
		zap.String("ssoSubject", claims.Sub),
	)

	return &SSOTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
	}, nil
}

// matchOrCreateUser SSO 用户匹配策略：SSO Subject → Username → 自动创建
func (s *SSOService) matchOrCreateUser(ctx context.Context, subject, preferredUsername, email, name string, cfg *SSOConfig) (*model.User, error) {
	providerName := cfg.ProviderName
	if providerName == "" {
		providerName = "oidc"
	}

	// 1. 按 SSO Subject 查找
	user, err := s.userRepo.FindBySSOSubject(ctx, providerName, subject)
	if err == nil {
		return user, nil
	}

	// 2. 按 username 查找（优先 preferred_username，退选 email 前缀）
	username := preferredUsername
	if username == "" && email != "" {
		parts := strings.SplitN(email, "@", 2)
		username = parts[0]
	}
	if username == "" && name != "" {
		username = name
	}

	if username != "" {
		user, err = s.userRepo.FindByUsername(ctx, username)
		if err == nil {
			// 已有同名本地用户，绑定 SSO 信息
			user.AuthSource = "sso"
			user.SSOProvider = providerName
			user.SSOSubject = subject
			if err := s.userRepo.Update(ctx, user); err != nil {
				return nil, fmt.Errorf("绑定 SSO 信息失败: %w", err)
			}
			return user, nil
		}
	}

	// 3. 自动创建用户
	if !cfg.AutoCreateUser {
		return nil, fmt.Errorf("SSO 用户不存在且未启用自动创建")
	}

	if username == "" {
		username = "sso_" + subject[:8]
	}

	// 确保 username 唯一
	baseUsername := username
	for i := 1; ; i++ {
		_, err := s.userRepo.FindByUsername(ctx, username)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				break
			}
			return nil, fmt.Errorf("查询用户失败: %w", err)
		}
		username = fmt.Sprintf("%s_%d", baseUsername, i)
		if i > 100 {
			return nil, fmt.Errorf("无法生成唯一用户名")
		}
	}

	// 验证角色是否存在
	role := cfg.DefaultRole
	if _, err := s.roleRepo.FindByName(ctx, role); err != nil {
		s.logger.Warn("SSO 默认角色不存在，使用 viewer", zap.String("role", role))
		role = "viewer"
	}

	// 生成随机密码（SSO 用户不使用密码登录）
	randomPwd, _ := generateRandom(16)
	pwdHash, err := HashPassword(randomPwd)
	if err != nil {
		return nil, fmt.Errorf("生成密码哈希失败: %w", err)
	}

	newUser := &model.User{
		ID:           uuid.New().String(),
		Username:     username,
		PasswordHash: pwdHash,
		Role:         role,
		IsActive:     true,
		AuthSource:   "sso",
		SSOProvider:  providerName,
		SSOSubject:   subject,
	}

	if err := s.userRepo.Create(ctx, newUser); err != nil {
		return nil, fmt.Errorf("创建 SSO 用户失败: %w", err)
	}

	s.logger.Info("自动创建 SSO 用户",
		zap.String("username", username),
		zap.String("ssoSubject", subject),
		zap.String("role", role),
	)

	return newUser, nil
}
