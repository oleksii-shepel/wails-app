package main

// ElevatedService — JWT-secured persistent elevated IPC channel
//
// Architecture:
//   main process (unprivileged)          elevated helper process (admin/root)
//   ─────────────────────────────────    ────────────────────────────────────
//   1. generate 32-byte HMAC secret
//   2. pick random TCP port
//   3. launch helper via OS elevation      UAC / sudo fires ONCE
//      --elevated-service --port P
//      --secret <base64>
//                                         4. listen on 127.0.0.1:P
//   5. dial (with retry)                  5. accept one connection
//   6. mint initial 5-min JWT             6. validate JWT (sig + exp)
//   7. send newline-delimited JSON req     7. executeElevatedRequest(req)
//      {"token":"<JWT>","req":{...}}       8. write response JSON
//   9. decode response
//  10. rotate JWT every 4 min (TTL 5)
//
// Security:
//   • secret is random per session — never written to disk
//   • listener bound to 127.0.0.1 only
//   • JWT expiry prevents post-rotation replay attacks
//   • helper exits when the single accepted connection is closed

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ── Shared flag / protocol ────────────────────────────────────────────────────

const elevatedServiceFlag = "--elevated-service"

// elevatedServiceClaims is embedded in every IPC JWT.
type elevatedServiceClaims struct {
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

// ipcRequest is the wire format sent from main → helper.
type ipcRequest struct {
	Token   string          `json:"token"` // signed JWT
	Request elevatedRequest `json:"req"`
}

// ipcResponse is the wire format sent from helper → main.
type ipcResponse struct {
	Response elevatedResponse `json:"resp"`
}

// ── Server side (runs inside the elevated helper process) ────────────────────

// RunElevatedService is called from maybeRunElevatedService() when the process
// detects the --elevated-service flag. It blocks until the single accepted
// connection closes or the context is cancelled.
func RunElevatedService(port string, secretB64 string) error {
	secret, err := base64.RawURLEncoding.DecodeString(secretB64)
	if err != nil {
		return fmt.Errorf("elevated-service: bad secret: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		return fmt.Errorf("elevated-service: listen: %w", err)
	}
	defer ln.Close()

	// Accept exactly one connection — from the parent process.
	// When that connection closes we exit.
	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("elevated-service: accept: %w", err)
	}
	defer conn.Close()
	ln.Close() // stop accepting new connections

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024) // 4 MB max request
	encoder := json.NewEncoder(conn)

	for scanner.Scan() {
		line := scanner.Bytes()
		var ireq ipcRequest
		if err := json.Unmarshal(line, &ireq); err != nil {
			_ = encoder.Encode(ipcResponse{Response: elevatedResponse{
				Success: false, Error: "malformed request: " + err.Error(),
			}})
			continue
		}

		// Validate JWT.
		if verr := validateServiceJWT(ireq.Token, secret); verr != nil {
			_ = encoder.Encode(ipcResponse{Response: elevatedResponse{
				Success: false, Error: "unauthorized: " + verr.Error(),
			}})
			continue
		}

		resp := executeElevatedRequest(ireq.Request)
		if encErr := encoder.Encode(ipcResponse{Response: resp}); encErr != nil {
			return encErr // connection broken
		}
	}
	return scanner.Err()
}

func validateServiceJWT(tokenStr string, secret []byte) error {
	claims := &elevatedServiceClaims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return secret, nil
	}, jwt.WithIssuer("wails-elev-svc"), jwt.WithAudience("wails-elev-helper"))
	if err != nil || !tok.Valid {
		return errors.New("invalid or expired token")
	}
	return nil
}

// ── Client side (runs inside the main / Wails process) ───────────────────────

// ElevatedSessionStatus is returned to the frontend.
type ElevatedSessionStatus struct {
	Active    bool   `json:"active"`
	PID       int    `json:"pid,omitempty"`
	StartedAt int64  `json:"startedAt,omitempty"` // unix seconds
	Port      int    `json:"port,omitempty"`
	OpCount   int    `json:"opCount"`
	Error     string `json:"error,omitempty"`
}

// ElevatedServiceClient manages the client side of the persistent channel.
type ElevatedServiceClient struct {
	mu        sync.Mutex
	conn      net.Conn
	scanner   *bufio.Scanner
	secret    []byte
	sessionID string
	token     string    // current short-lived JWT
	tokenExp  time.Time // when to rotate
	startedAt time.Time
	port      int
	pid       int
	opCount   int
	stopCtx   context.Context
	stopFn    context.CancelFunc
}

// mintJWT creates a new 5-minute JWT signed with the session secret.
func (c *ElevatedServiceClient) mintJWT() (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(5 * time.Minute)
	claims := &elevatedServiceClaims{
		SessionID: c.sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "wails-elev-svc",
			Audience:  jwt.ClaimStrings{"wails-elev-helper"},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(c.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// refreshTokenLocked rotates the JWT when within 1 min of expiry. Caller holds mu.
func (c *ElevatedServiceClient) refreshTokenLocked() error {
	if time.Until(c.tokenExp) > 1*time.Minute {
		return nil
	}
	tok, exp, err := c.mintJWT()
	if err != nil {
		return err
	}
	c.token = tok
	c.tokenExp = exp
	return nil
}

// Alive reports whether the service connection is healthy.
func (c *ElevatedServiceClient) Alive() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}

// Dispatch sends a request to the elevated helper and returns the response.
// Serialised via mutex; safe for concurrent callers.
func (c *ElevatedServiceClient) Dispatch(req elevatedRequest) (elevatedResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return elevatedResponse{}, errors.New("elevated service not connected")
	}
	if err := c.refreshTokenLocked(); err != nil {
		return elevatedResponse{}, fmt.Errorf("token refresh: %w", err)
	}

	frame, err := json.Marshal(ipcRequest{Token: c.token, Request: req})
	if err != nil {
		return elevatedResponse{}, err
	}
	frame = append(frame, '\n')

	if _, err := c.conn.Write(frame); err != nil {
		c.teardownLocked()
		return elevatedResponse{}, fmt.Errorf("write: %w", err)
	}
	if !c.scanner.Scan() {
		c.teardownLocked()
		if e := c.scanner.Err(); e != nil {
			return elevatedResponse{}, fmt.Errorf("read: %w", e)
		}
		return elevatedResponse{}, errors.New("elevated service closed connection")
	}
	var iresp ipcResponse
	if err := json.Unmarshal(c.scanner.Bytes(), &iresp); err != nil {
		return elevatedResponse{}, fmt.Errorf("decode: %w", err)
	}
	c.opCount++
	if !iresp.Response.Success {
		return elevatedResponse{}, errors.New(iresp.Response.Error)
	}
	return iresp.Response, nil
}

// Status returns a snapshot of the session status for the frontend.
func (c *ElevatedServiceClient) Status() ElevatedSessionStatus {
	if c == nil {
		return ElevatedSessionStatus{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return ElevatedSessionStatus{}
	}
	return ElevatedSessionStatus{
		Active:    true,
		PID:       c.pid,
		StartedAt: c.startedAt.Unix(),
		Port:      c.port,
		OpCount:   c.opCount,
	}
}

// Stop closes the connection and signals the helper to exit.
func (c *ElevatedServiceClient) Stop() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.teardownLocked()
	if c.stopFn != nil {
		c.stopFn()
	}
}

func (c *ElevatedServiceClient) teardownLocked() {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

// StartElevatedServiceClient launches the elevated helper (one OS elevation
// prompt) and returns a connected, JWT-authenticated client.
func StartElevatedServiceClient(exePath string) (*ElevatedServiceClient, error) {
	// 1. Cryptographic session material.
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}
	sidBytes := make([]byte, 12)
	if _, err := rand.Read(sidBytes); err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}
	sid := base64.RawURLEncoding.EncodeToString(sidBytes)

	// 2. Reserve a free TCP port.
	freeListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("allocate port: %w", err)
	}
	port := freeListener.Addr().(*net.TCPAddr).Port
	freeListener.Close()

	// 3. Launch the elevated helper (one UAC / sudo prompt).
	secretB64 := base64.RawURLEncoding.EncodeToString(secret)
	pid, err := launchElevatedService(exePath, port, secretB64)
	if err != nil {
		return nil, fmt.Errorf("launch elevated service: %w", err)
	}

	// 4. Dial with exponential back-off (helper needs a moment to bind).
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	var conn net.Conn
	for attempt := 0; attempt < 25; attempt++ {
		conn, err = net.DialTimeout("tcp", addr, 400*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(time.Duration(40+attempt*30) * time.Millisecond)
	}
	if err != nil {
		return nil, fmt.Errorf("connect to elevated service on :%d: %w", port, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	client := &ElevatedServiceClient{
		conn:      conn,
		scanner:   newIpcScanner(conn),
		secret:    secret,
		sessionID: sid,
		startedAt: time.Now(),
		port:      port,
		pid:       pid,
		stopCtx:   ctx,
		stopFn:    cancel,
	}

	// 5. Mint the initial JWT.
	tok, exp, err := client.mintJWT()
	if err != nil {
		conn.Close()
		cancel()
		return nil, fmt.Errorf("initial jwt: %w", err)
	}
	client.token = tok
	client.tokenExp = exp

	// 6. Background JWT rotation every 4 minutes.
	go func() {
		ticker := time.NewTicker(4 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				client.mu.Lock()
				_ = client.refreshTokenLocked()
				client.mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	return client, nil
}

func newIpcScanner(conn net.Conn) *bufio.Scanner {
	s := bufio.NewScanner(conn)
	s.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	return s
}

// ── Flag detection for the helper process ────────────────────────────────────

// maybeRunElevatedService checks os.Args for --elevated-service.
// Returns (true, ...) when the flag was present so main() can exit cleanly.
func maybeRunElevatedService() (bool, error) {
	args := os.Args
	found := false
	for _, a := range args {
		if a == elevatedServiceFlag {
			found = true
			break
		}
	}
	if !found {
		return false, nil
	}
	port := argValue(args, "--port")
	secret := argValue(args, "--secret")
	if port == "" || secret == "" {
		return true, errors.New("elevated-service: missing --port or --secret")
	}
	return true, RunElevatedService(port, secret)
}

func argValue(args []string, flag string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}
