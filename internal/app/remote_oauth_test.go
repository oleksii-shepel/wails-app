package app

import (
	"net/url"
	"strings"
	"testing"

	contracts "wails-app/internal/contracts"
)

func TestBuildRemoteOAuthAuthURLGoogle(t *testing.T) {
	session := &remoteOAuthSession{
		protocol:     contracts.ProtocolGDrive,
		clientID:     "google-client",
		redirectURI:  "http://127.0.0.1:32123/oauth/callback",
		state:        "state-123",
		codeVerifier: "verifier-123",
		scopes:       []string{"https://www.googleapis.com/auth/drive.readonly"},
	}

	rawURL, err := buildRemoteOAuthAuthURL(session)
	if err != nil {
		t.Fatalf("buildRemoteOAuthAuthURL() error = %v", err)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}

	if parsed.Host != "accounts.google.com" {
		t.Fatalf("host = %q, want accounts.google.com", parsed.Host)
	}

	query := parsed.Query()
	if got := query.Get("client_id"); got != "google-client" {
		t.Fatalf("client_id = %q, want google-client", got)
	}
	if got := query.Get("redirect_uri"); got != session.redirectURI {
		t.Fatalf("redirect_uri = %q, want %q", got, session.redirectURI)
	}
	if got := query.Get("code_challenge_method"); got != "S256" {
		t.Fatalf("code_challenge_method = %q, want S256", got)
	}
	if got := query.Get("access_type"); got != "offline" {
		t.Fatalf("access_type = %q, want offline", got)
	}
	if got := query.Get("prompt"); got != "consent" {
		t.Fatalf("prompt = %q, want consent", got)
	}
	if got := query.Get("scope"); !strings.Contains(got, "drive.readonly") {
		t.Fatalf("scope = %q, want drive.readonly", got)
	}
}

func TestRemoteOAuthProviderOneDriveDefaultsTenant(t *testing.T) {
	config := remoteOAuthProvider(contracts.ProtocolOneDrive, "")
	if !strings.Contains(config.authURL, "/common/") {
		t.Fatalf("authURL = %q, want /common/", config.authURL)
	}
	if !strings.Contains(config.tokenURL, "/common/") {
		t.Fatalf("tokenURL = %q, want /common/", config.tokenURL)
	}
}

func TestNormalizeRemoteOAuthScopesUsesDefaults(t *testing.T) {
	scopes := normalizeRemoteOAuthScopes(contracts.ProtocolDropbox, nil)
	if len(scopes) == 0 {
		t.Fatal("expected default scopes")
	}
	if scopes[0] != "files.metadata.read" {
		t.Fatalf("first scope = %q, want files.metadata.read", scopes[0])
	}
}
