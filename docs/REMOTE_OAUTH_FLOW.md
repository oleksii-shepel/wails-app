# Remote OAuth Flow

## Overview

The app now supports browser-based OAuth login for these remote providers:

- `gdrive`
- `dropbox`
- `onedrive`

This flow is session-only. OAuth client configuration and acquired tokens are kept in memory and are not persisted to disk.

The implementation is intended to feel similar to `npm login`:

1. The user starts authorization from the remote mount dialog.
2. The app opens the system browser.
3. The user signs in and approves access.
4. The provider redirects back to a temporary localhost callback.
5. The app exchanges the returned authorization code for tokens.
6. The user clicks `Connect` and the provider is mounted in the current session.

S3-compatible mounts are unchanged and continue to use direct credential entry.

## Why Localhost

The app uses `system browser + localhost callback + PKCE` instead of an embedded webview or a custom URI scheme.

Reasons:

- The app already has a strong localhost server pattern in the backend.
- The system browser gives the user normal SSO, passkeys, and saved credentials.
- This avoids platform-specific custom-protocol registration work.
- Providers are generally happier with system-browser OAuth than with embedded webviews.

The redirect URI is created dynamically per authorization attempt and looks like:

```text
http://localhost:<ephemeral-port>
```

## High-Level Flow

### 1. User starts OAuth

The frontend dialog in [remote-connect-dialog.component.ts](../frontend/src/app/components/remote-connect-dialog.component.ts) collects:

- provider protocol
- alias name for the eventual mount
- provider client ID
- optional Microsoft tenant for OneDrive

Then it calls `RemoteOAuthBegin`.

### 2. Backend creates a temporary OAuth session

The backend session manager lives in [remote_oauth.go](../internal/app/remote_oauth.go).

For each authorization attempt it creates:

- a random session ID
- a random `state` token
- a PKCE `code_verifier`
- a PKCE `code_challenge`
- a temporary localhost HTTP server on `127.0.0.1:0`
- a temporary localhost HTTP server on `127.0.0.1:0`

The returned `RemoteOAuthBeginResponse` contains:

- `sessionId`
- `authUrl`
- `redirectUri`

The listener still binds to loopback `127.0.0.1`, but the browser-facing redirect URI uses `localhost` so Microsoft desktop redirect registration can use a single `http://localhost` entry while ignoring the callback port.

### 3. Browser opens

The frontend opens `authUrl` using Wails runtime browser support, not the child webview.

The browser-based step is intentionally out-of-process from the main app UI.

### 4. Provider redirects back to localhost

The temporary callback handler validates:

- `state`
- provider error parameters
- returned authorization code presence

If validation succeeds, the backend exchanges the code for tokens.

### 5. OAuth session becomes ready

The frontend polls `RemoteOAuthStatus` until the session transitions from:

- `pending`

to one of:

- `complete`
- `failed`

When possible, the backend also loads a lightweight account profile so the dialog can show who was authorized.

### 6. User mounts the provider

When the user clicks `Connect`, the frontend calls `RemoteConnectOAuth`.

The backend then mounts the provider through the existing remote mount infrastructure in [remote.go](../internal/app/remote.go), but instead of passing a raw static token from the frontend, it passes an in-memory token source owned by the OAuth session.

## Backend Components

### Contracts

New OAuth-related wire types are defined in [remote.go](../internal/contracts/remote.go):

- `RemoteOAuthBeginRequest`
- `RemoteOAuthBeginResponse`
- `RemoteOAuthStatus`
- `RemoteOAuthConnectRequest`

The existing `RemoteConfig` still supports manual `accessToken` entry for non-browser token mode.

### OAuth session manager

[remote_oauth.go](../internal/app/remote_oauth.go) is responsible for:

- session lifecycle
- callback server startup and shutdown
- PKCE generation
- provider auth URL construction
- code exchange
- refresh-token based access-token renewal
- lightweight profile loading
- releasing OAuth sessions when a mount disconnects

The `State` struct in [state.go](../internal/app/state.go) now tracks:

- active OAuth sessions
- which mounted VFS roots reference which OAuth session

### Remote mounting

[remote.go](../internal/app/remote.go) now has an internal `remoteConnect(..., tokenSource)` path.

That allows:

- manual token mounts via `RemoteConnect`
- browser-authorized mounts via `RemoteConnectOAuth`

without duplicating the mount logic.

### VFS token source abstraction

Cloud VFS adapters now support either:

- a static access token
- a backend-owned token source

The token source interface lives in [token_source.go](../vfs/cloudutil/token_source.go).

It is used by:

- [fs.go](../vfs/gdrive/fs.go)
- [fs.go](../vfs/dropbox/fs.go)
- [fs.go](../vfs/onedrive/fs.go)

This is important because refreshed access tokens should stay on the backend side and not be pushed back into the frontend form state.

## Frontend Behavior

The remote mount dialog now has two modes for cloud providers:

- `Browser Sign-In`
- `Existing Access Token`

The browser sign-in path is the default for:

- Google Drive
- Dropbox
- OneDrive

The dialog now supports:

- provider client ID entry
- optional OneDrive tenant entry
- `Authorize`
- polling session state
- showing successful authorization state before the final mount step

This UI is implemented in [remote-connect-dialog.component.ts](../frontend/src/app/components/remote-connect-dialog.component.ts).

The frontend API wrapper for the new methods is in [api.service.ts](../frontend/src/app/services/api.service.ts).

## Provider Notes

### Google Drive

Uses:

- auth endpoint: `https://accounts.google.com/o/oauth2/v2/auth`
- token endpoint: `https://oauth2.googleapis.com/token`

Default scope:

- `https://www.googleapis.com/auth/drive.readonly`

Additional behavior:

- requests offline access
- includes granted scopes
- forces consent so a refresh token is more likely to be returned

### Dropbox

Uses:

- auth endpoint: `https://www.dropbox.com/oauth2/authorize`
- token endpoint: `https://api.dropboxapi.com/oauth2/token`

Default scopes:

- `files.metadata.read`
- `files.content.read`

Additional behavior:

- requests `token_access_type=offline`

### OneDrive

Uses Microsoft identity platform v2 endpoints:

- `https://login.microsoftonline.com/<tenant>/oauth2/v2.0/authorize`
- `https://login.microsoftonline.com/<tenant>/oauth2/v2.0/token`

Default tenant:

- `common`

Default scopes:

- `offline_access`
- `openid`
- `profile`
- `Files.Read`

## Session Lifetime

OAuth state is session-only.

That means:

- client IDs are not persisted by this feature
- access tokens are not persisted
- refresh tokens are not persisted
- active mounts last only while the app is running

On disconnect:

- the VFS mount is removed
- the OAuth session reference count is reduced
- unreferenced completed OAuth sessions are dropped

On app shutdown:

- temporary callback servers are shut down
- OAuth session state is discarded

## Current Limitations

### 1. Provider registration is still required

The app now implements the OAuth flow itself, but each provider still requires a valid app registration and client ID from the user or deployment owner.

### 2. No secure persistence yet

Refresh tokens are not stored across restarts.

That is intentional for the first version because the app does not yet have a secure secret-storage layer.

### 3. Cloud mounts remain read-only

Current cloud VFS adapters are read-only:

- Google Drive
- Dropbox
- OneDrive

### 4. Google native document export is still incomplete

Native Google Workspace documents are not yet exported/downloaded through the mount layer. Non-folder `application/vnd.google-apps.*` items still need additional handling.

### 5. Production Angular optimization build may fail in restricted environments

The development Angular build passes, but fully optimized production builds may still hit local environment process restrictions unrelated to this feature.

## Verification Notes

During implementation, the following checks were used:

- TypeScript app compile without emit
- Angular development build
- Go formatting

Backend `internal/app` tests still hit an existing Windows runtime issue (`0xc0000135`) in this environment, and a full Go build can also depend on local FFmpeg/pkg-config availability.

## Recommended Next Steps

If this flow is extended, the next high-value steps are:

1. Add secure persistence for refresh tokens.
2. Add disconnect/revoke behavior per provider.
3. Add provider configuration persistence for non-secret client settings if desired.
4. Add write support for supported cloud providers.
5. Add Google Workspace document export handling.
