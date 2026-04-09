package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	contracts "wails-app/internal/contracts"
)

const (
	remoteOAuthStatusPending   = "pending"
	remoteOAuthStatusComplete  = "complete"
	remoteOAuthStatusFailed    = "failed"
	remoteOAuthStatusCancelled = "cancelled"
)

type remoteOAuthSession struct {
	id           string
	protocol     contracts.RemoteProtocol
	clientID     string
	tenant       string
	redirectURI  string
	state        string
	codeVerifier string
	scopes       []string

	mu           sync.Mutex
	status       string
	err          string
	accountName  string
	accountEmail string
	accessToken  string
	refreshToken string
	expiry       time.Time
	refCount     int

	server *http.Server
}

type remoteOAuthProviderConfig struct {
	authURL       string
	tokenURL      string
	defaultScopes []string
}

type remoteOAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	Description  string `json:"error_description"`
}

func (s *State) RemoteOAuthBegin(req contracts.RemoteOAuthBeginRequest) (*contracts.RemoteOAuthBeginResponse, error) {
	protocol := req.Protocol
	if !isRemoteOAuthProtocol(protocol) {
		return nil, fmt.Errorf("oauth is not supported for protocol: %s", protocol)
	}

	clientID := strings.TrimSpace(req.ClientID)
	if clientID == "" {
		return nil, fmt.Errorf("client ID is required")
	}

	sessionID, err := randomToken(24)
	if err != nil {
		return nil, fmt.Errorf("oauth: generate session id: %w", err)
	}
	stateToken, err := randomToken(24)
	if err != nil {
		return nil, fmt.Errorf("oauth: generate state: %w", err)
	}
	codeVerifier, err := randomToken(48)
	if err != nil {
		return nil, fmt.Errorf("oauth: generate verifier: %w", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("oauth: start callback listener: %w", err)
	}

	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/oauth/callback", listener.Addr().(*net.TCPAddr).Port)
	session := &remoteOAuthSession{
		id:           sessionID,
		protocol:     protocol,
		clientID:     clientID,
		tenant:       normalizeRemoteOAuthTenant(protocol, req.Tenant),
		redirectURI:  redirectURI,
		state:        stateToken,
		codeVerifier: codeVerifier,
		scopes:       normalizeRemoteOAuthScopes(protocol, req.Scopes),
		status:       remoteOAuthStatusPending,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		s.handleRemoteOAuthCallback(session, w, r)
	})
	session.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 15 * time.Second,
	}

	s.remoteOAuthMu.Lock()
	if existing := s.remoteOAuth[sessionID]; existing != nil {
		s.remoteOAuthMu.Unlock()
		listener.Close()
		return nil, fmt.Errorf("oauth: duplicate session id")
	}
	s.remoteOAuth[sessionID] = session
	s.remoteOAuthMu.Unlock()

	go func() {
		if err := session.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			session.setFailure(fmt.Sprintf("callback listener failed: %v", err))
		}
	}()

	authURL, err := buildRemoteOAuthAuthURL(session)
	if err != nil {
		_ = session.shutdownCallbackServer(context.Background())
		s.remoteOAuthMu.Lock()
		delete(s.remoteOAuth, sessionID)
		s.remoteOAuthMu.Unlock()
		return nil, err
	}

	return &contracts.RemoteOAuthBeginResponse{
		SessionID:   session.id,
		Protocol:    session.protocol,
		AuthURL:     authURL,
		RedirectURI: session.redirectURI,
	}, nil
}

func (s *State) RemoteOAuthStatus(sessionID string) (*contracts.RemoteOAuthStatus, error) {
	session := s.getRemoteOAuthSession(sessionID)
	if session == nil {
		return nil, fmt.Errorf("oauth session not found")
	}
	return session.snapshot(), nil
}

func (s *State) RemoteOAuthCancel(sessionID string) error {
	session := s.getRemoteOAuthSession(sessionID)
	if session == nil {
		return nil
	}

	session.mu.Lock()
	if session.status == remoteOAuthStatusPending {
		session.status = remoteOAuthStatusCancelled
	}
	session.mu.Unlock()

	if session.mountRefs() == 0 {
		s.remoteOAuthMu.Lock()
		delete(s.remoteOAuth, sessionID)
		s.remoteOAuthMu.Unlock()
	}
	return session.shutdownCallbackServer(context.Background())
}

func (s *State) RemoteConnectOAuth(req contracts.RemoteOAuthConnectRequest) (*contracts.RemoteMount, error) {
	session := s.getRemoteOAuthSession(req.SessionID)
	if session == nil {
		return nil, fmt.Errorf("oauth session not found")
	}
	status := session.snapshot()
	if status.Status != remoteOAuthStatusComplete {
		return nil, fmt.Errorf("oauth session is not ready")
	}

	mountName := strings.TrimSpace(req.Name)
	if mountName == "" {
		switch session.protocol {
		case contracts.ProtocolGDrive:
			mountName = "Google Drive"
		case contracts.ProtocolDropbox:
			mountName = "Dropbox"
		case contracts.ProtocolOneDrive:
			mountName = "OneDrive"
		}
	}

	mount, err := s.remoteConnect(contracts.RemoteConfig{
		Protocol: session.protocol,
		Name:     mountName,
	}, session)
	if err != nil {
		return nil, err
	}

	s.remoteOAuthMu.Lock()
	if current := s.remoteOAuth[session.id]; current != nil {
		current.mu.Lock()
		current.refCount++
		current.mu.Unlock()
		s.remoteOAuthRefs[mount.MountPoint] = session.id
	}
	s.remoteOAuthMu.Unlock()

	return mount, nil
}

func (s *State) handleRemoteOAuthCallback(session *remoteOAuthSession, w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if !strings.EqualFold(query.Get("state"), session.state) {
		session.setFailure("state mismatch")
		writeRemoteOAuthHTML(w, http.StatusBadRequest, "Authentication failed", "The OAuth state token did not match. Close this page and try again.")
		_ = session.shutdownCallbackServer(context.Background())
		return
	}

	if authErr := strings.TrimSpace(query.Get("error")); authErr != "" {
		description := strings.TrimSpace(query.Get("error_description"))
		if description != "" {
			session.setFailure(description)
		} else {
			session.setFailure(authErr)
		}
		writeRemoteOAuthHTML(w, http.StatusBadRequest, "Authentication failed", session.errString())
		_ = session.shutdownCallbackServer(context.Background())
		return
	}

	code := strings.TrimSpace(query.Get("code"))
	if code == "" {
		session.setFailure("authorization code was not returned")
		writeRemoteOAuthHTML(w, http.StatusBadRequest, "Authentication failed", "No authorization code was returned by the provider.")
		_ = session.shutdownCallbackServer(context.Background())
		return
	}

	if err := session.exchangeCode(r.Context(), code); err != nil {
		session.setFailure(err.Error())
		writeRemoteOAuthHTML(w, http.StatusBadRequest, "Authentication failed", session.errString())
		_ = session.shutdownCallbackServer(context.Background())
		return
	}

	_ = session.loadAccountProfile(r.Context())
	writeRemoteOAuthHTML(w, http.StatusOK, "Authentication complete", "You can close this tab and return to the app.")
	_ = session.shutdownCallbackServer(context.Background())
}

func (s *State) getRemoteOAuthSession(sessionID string) *remoteOAuthSession {
	s.remoteOAuthMu.Lock()
	defer s.remoteOAuthMu.Unlock()
	return s.remoteOAuth[strings.TrimSpace(sessionID)]
}

func (s *State) releaseRemoteOAuthMount(mountPoint string) {
	s.remoteOAuthMu.Lock()
	sessionID := s.remoteOAuthRefs[mountPoint]
	if sessionID != "" {
		delete(s.remoteOAuthRefs, mountPoint)
	}
	session := s.remoteOAuth[sessionID]
	if session != nil {
		session.mu.Lock()
		if session.refCount > 0 {
			session.refCount--
		}
		refs := session.refCount
		status := session.status
		session.mu.Unlock()
		if refs == 0 && status != remoteOAuthStatusPending {
			delete(s.remoteOAuth, sessionID)
		}
	}
	s.remoteOAuthMu.Unlock()
}

func (s *State) shutdownRemoteOAuthSessions() {
	s.remoteOAuthMu.Lock()
	sessions := make([]*remoteOAuthSession, 0, len(s.remoteOAuth))
	for id, session := range s.remoteOAuth {
		delete(s.remoteOAuth, id)
		sessions = append(sessions, session)
	}
	clear(s.remoteOAuthRefs)
	s.remoteOAuthMu.Unlock()

	for _, session := range sessions {
		_ = session.shutdownCallbackServer(context.Background())
	}
}

func (s *remoteOAuthSession) snapshot() *contracts.RemoteOAuthStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := &contracts.RemoteOAuthStatus{
		SessionID:    s.id,
		Protocol:     s.protocol,
		Status:       s.status,
		Error:        s.err,
		AccountName:  s.accountName,
		AccountEmail: s.accountEmail,
	}
	if !s.expiry.IsZero() {
		status.ExpiresAt = s.expiry.Unix()
	}
	return status
}

func (s *remoteOAuthSession) setFailure(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status == remoteOAuthStatusComplete {
		return
	}
	s.status = remoteOAuthStatusFailed
	s.err = strings.TrimSpace(message)
}

func (s *remoteOAuthSession) errString() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(s.err) == "" {
		return "Authentication failed."
	}
	return s.err
}

func (s *remoteOAuthSession) shutdownCallbackServer(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return s.server.Shutdown(shutdownCtx)
}

func (s *remoteOAuthSession) mountRefs() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refCount
}

func (s *remoteOAuthSession) AccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	if strings.TrimSpace(s.accessToken) == "" {
		err := s.err
		s.mu.Unlock()
		if err == "" {
			err = "oauth access token is not available"
		}
		return "", fmt.Errorf("%s", err)
	}
	if s.refreshToken == "" || time.Until(s.expiry) > 90*time.Second {
		token := s.accessToken
		s.mu.Unlock()
		return token, nil
	}
	s.mu.Unlock()

	if err := s.refresh(ctx); err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(s.accessToken) == "" {
		return "", fmt.Errorf("oauth access token is not available")
	}
	return s.accessToken, nil
}

func (s *remoteOAuthSession) exchangeCode(ctx context.Context, code string) error {
	token, err := s.exchange(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {s.clientID},
		"redirect_uri":  {s.redirectURI},
		"code_verifier": {s.codeVerifier},
	})
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = remoteOAuthStatusComplete
	s.err = ""
	s.accessToken = token.AccessToken
	if token.RefreshToken != "" {
		s.refreshToken = token.RefreshToken
	}
	s.expiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	return nil
}

func (s *remoteOAuthSession) refresh(ctx context.Context) error {
	s.mu.Lock()
	refreshToken := s.refreshToken
	s.mu.Unlock()
	if strings.TrimSpace(refreshToken) == "" {
		return nil
	}

	token, err := s.exchange(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {s.clientID},
		"scope":         {strings.Join(s.scopes, " ")},
	})
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessToken = token.AccessToken
	if token.RefreshToken != "" {
		s.refreshToken = token.RefreshToken
	}
	if token.ExpiresIn > 0 {
		s.expiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}
	return nil
}

func (s *remoteOAuthSession) exchange(ctx context.Context, values url.Values) (*remoteOAuthTokenResponse, error) {
	config := remoteOAuthProvider(s.protocol, s.tenant)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, config.tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var token remoteOAuthTokenResponse
	if err := json.NewDecoder(response.Body).Decode(&token); err != nil {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return nil, fmt.Errorf("oauth token response decode failed: %v %s", err, strings.TrimSpace(string(body)))
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusBadRequest {
		message := strings.TrimSpace(token.Description)
		if message == "" {
			message = strings.TrimSpace(token.Error)
		}
		if message == "" {
			message = fmt.Sprintf("token exchange failed with HTTP %d", response.StatusCode)
		}
		return nil, fmt.Errorf("%s", message)
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return nil, fmt.Errorf("provider did not return an access token")
	}
	return &token, nil
}

func (s *remoteOAuthSession) loadAccountProfile(ctx context.Context) error {
	type profile struct {
		Name  string
		Email string
	}

	result := profile{}
	switch s.protocol {
	case contracts.ProtocolGDrive:
		var response struct {
			User struct {
				DisplayName  string `json:"displayName"`
				EmailAddress string `json:"emailAddress"`
			} `json:"user"`
		}
		if err := s.getJSON(ctx, "https://www.googleapis.com/drive/v3/about?fields=user(displayName,emailAddress)", &response); err != nil {
			return err
		}
		result.Name = response.User.DisplayName
		result.Email = response.User.EmailAddress
	case contracts.ProtocolDropbox:
		var response struct {
			Name struct {
				DisplayName string `json:"display_name"`
			} `json:"name"`
			Email string `json:"email"`
		}
		if err := s.postJSON(ctx, "https://api.dropboxapi.com/2/users/get_current_account", "null", &response); err != nil {
			return err
		}
		result.Name = response.Name.DisplayName
		result.Email = response.Email
	case contracts.ProtocolOneDrive:
		var response struct {
			DisplayName       string `json:"displayName"`
			UserPrincipalName string `json:"userPrincipalName"`
			Mail              string `json:"mail"`
		}
		if err := s.getJSON(ctx, "https://graph.microsoft.com/v1.0/me?$select=displayName,userPrincipalName,mail", &response); err != nil {
			return err
		}
		result.Name = response.DisplayName
		result.Email = strings.TrimSpace(response.Mail)
		if result.Email == "" {
			result.Email = response.UserPrincipalName
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(result.Name) != "" {
		s.accountName = result.Name
	}
	if strings.TrimSpace(result.Email) != "" {
		s.accountEmail = result.Email
	}
	return nil
}

func (s *remoteOAuthSession) getJSON(ctx context.Context, endpoint string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	token, err := s.AccessToken(ctx)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return fmt.Errorf("profile request failed: HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(response.Body).Decode(target)
}

func (s *remoteOAuthSession) postJSON(ctx context.Context, endpoint string, body string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return err
	}
	token, err := s.AccessToken(ctx)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return fmt.Errorf("profile request failed: HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(response.Body).Decode(target)
}

func buildRemoteOAuthAuthURL(session *remoteOAuthSession) (string, error) {
	config := remoteOAuthProvider(session.protocol, session.tenant)
	values := url.Values{
		"client_id":             {session.clientID},
		"redirect_uri":          {session.redirectURI},
		"response_type":         {"code"},
		"state":                 {session.state},
		"code_challenge":        {pkceChallenge(session.codeVerifier)},
		"code_challenge_method": {"S256"},
	}

	scope := strings.Join(session.scopes, " ")
	if scope != "" {
		values.Set("scope", scope)
	}

	switch session.protocol {
	case contracts.ProtocolGDrive:
		values.Set("access_type", "offline")
		values.Set("include_granted_scopes", "true")
		values.Set("prompt", "consent")
	case contracts.ProtocolDropbox:
		values.Set("token_access_type", "offline")
	case contracts.ProtocolOneDrive:
		values.Set("response_mode", "query")
	}

	authURL, err := url.Parse(config.authURL)
	if err != nil {
		return "", err
	}
	authURL.RawQuery = values.Encode()
	return authURL.String(), nil
}

func remoteOAuthProvider(protocol contracts.RemoteProtocol, tenant string) remoteOAuthProviderConfig {
	switch protocol {
	case contracts.ProtocolGDrive:
		return remoteOAuthProviderConfig{
			authURL:       "https://accounts.google.com/o/oauth2/v2/auth",
			tokenURL:      "https://oauth2.googleapis.com/token",
			defaultScopes: []string{"https://www.googleapis.com/auth/drive.readonly"},
		}
	case contracts.ProtocolDropbox:
		return remoteOAuthProviderConfig{
			authURL:       "https://www.dropbox.com/oauth2/authorize",
			tokenURL:      "https://api.dropboxapi.com/oauth2/token",
			defaultScopes: []string{"files.metadata.read", "files.content.read"},
		}
	case contracts.ProtocolOneDrive:
		base := "https://login.microsoftonline.com/" + normalizeRemoteOAuthTenant(protocol, tenant) + "/oauth2/v2.0"
		return remoteOAuthProviderConfig{
			authURL:       base + "/authorize",
			tokenURL:      base + "/token",
			defaultScopes: []string{"offline_access", "openid", "profile", "Files.Read"},
		}
	default:
		return remoteOAuthProviderConfig{}
	}
}

func normalizeRemoteOAuthScopes(protocol contracts.RemoteProtocol, scopes []string) []string {
	if len(scopes) == 0 {
		return append([]string(nil), remoteOAuthProvider(protocol, "").defaultScopes...)
	}
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		trimmed := strings.TrimSpace(scope)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return append([]string(nil), remoteOAuthProvider(protocol, "").defaultScopes...)
	}
	return result
}

func normalizeRemoteOAuthTenant(protocol contracts.RemoteProtocol, tenant string) string {
	if protocol != contracts.ProtocolOneDrive {
		return ""
	}
	trimmed := strings.TrimSpace(tenant)
	if trimmed == "" {
		return "common"
	}
	return trimmed
}

func isRemoteOAuthProtocol(protocol contracts.RemoteProtocol) bool {
	return protocol == contracts.ProtocolGDrive ||
		protocol == contracts.ProtocolDropbox ||
		protocol == contracts.ProtocolOneDrive
}

func randomToken(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func writeRemoteOAuthHTML(w http.ResponseWriter, status int, title string, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, "<!doctype html><html><head><meta charset=\"utf-8\"><title>"+htmlEscape(title)+"</title><style>body{font-family:Segoe UI,Arial,sans-serif;background:#10161f;color:#f3f6fb;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;padding:24px}main{max-width:560px;background:#182130;border:1px solid #2c394b;border-radius:14px;padding:24px;box-shadow:0 18px 50px rgba(0,0,0,.28)}h1{margin:0 0 10px;font-size:22px}p{margin:0;color:#d1d9e6;line-height:1.5}</style></head><body><main><h1>"+htmlEscape(title)+"</h1><p>"+htmlEscape(message)+"</p></main></body></html>")
}

func htmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}
