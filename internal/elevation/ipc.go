package elevation

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
	"path/filepath"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const HelperFlag = "--elevated-helper"
const ServiceFlag = "--elevated-service"

type Handler func(Request) Response

type LaunchServiceFunc func(exePath string, port int, secretB64 string) (int, error)

type LaunchProcessFunc func(exePath, reqB64, resultPath string) error

type ExecutablePathFunc func() (string, error)

type serviceClaims struct {
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

type ipcRequest struct {
	Token   string  `json:"token"`
	Request Request `json:"req"`
}

type ipcResponse struct {
	Response Response `json:"resp"`
}

type IPCClient struct {
	mu        sync.Mutex
	conn      net.Conn
	scanner   *bufio.Scanner
	secret    []byte
	sessionID string
	token     string
	tokenExp  time.Time
	startedAt time.Time
	port      int
	pid       int
	opCount   int
	stopCtx   context.Context
	stopFn    context.CancelFunc
}

func RunService(port string, secretB64 string, handler Handler) error {
	secret, err := base64.RawURLEncoding.DecodeString(secretB64)
	if err != nil {
		return fmt.Errorf("elevated-service: bad secret: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		return fmt.Errorf("elevated-service: listen: %w", err)
	}
	defer ln.Close()

	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("elevated-service: accept: %w", err)
	}
	defer conn.Close()
	ln.Close()

	scanner := newScanner(conn)
	encoder := json.NewEncoder(conn)

	for scanner.Scan() {
		var req ipcRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			_ = encoder.Encode(ipcResponse{Response: Response{Success: false, Error: "malformed request: " + err.Error()}})
			continue
		}
		if err := validateJWT(req.Token, secret); err != nil {
			_ = encoder.Encode(ipcResponse{Response: Response{Success: false, Error: "unauthorized: " + err.Error()}})
			continue
		}
		if err := encoder.Encode(ipcResponse{Response: handler(req.Request)}); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func StartClient(exePath string, launch LaunchServiceFunc) (*IPCClient, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}
	sidBytes := make([]byte, 12)
	if _, err := rand.Read(sidBytes); err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}
	sid := base64.RawURLEncoding.EncodeToString(sidBytes)

	freeListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("allocate port: %w", err)
	}
	port := freeListener.Addr().(*net.TCPAddr).Port
	freeListener.Close()

	secretB64 := base64.RawURLEncoding.EncodeToString(secret)
	pid, err := launch(exePath, port, secretB64)
	if err != nil {
		return nil, fmt.Errorf("launch elevated service: %w", err)
	}

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
	client := &IPCClient{
		conn:      conn,
		scanner:   newScanner(conn),
		secret:    secret,
		sessionID: sid,
		startedAt: time.Now(),
		port:      port,
		pid:       pid,
		stopCtx:   ctx,
		stopFn:    cancel,
	}

	tok, exp, err := client.mintJWT()
	if err != nil {
		conn.Close()
		cancel()
		return nil, fmt.Errorf("initial jwt: %w", err)
	}
	client.token = tok
	client.tokenExp = exp

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

func MaybeRunService(args []string, handler Handler) (bool, error) {
	if !containsArg(args, ServiceFlag) {
		return false, nil
	}
	port := argValue(args, "--port")
	secret := argValue(args, "--secret")
	if port == "" || secret == "" {
		return true, errors.New("elevated-service: missing --port or --secret")
	}
	return true, RunService(port, secret, handler)
}

func MaybeRunHelper(args []string, handler Handler) (bool, error) {
	if len(args) < 2 || args[1] != HelperFlag {
		return false, nil
	}
	reqB64, resultPath, err := parseHelperArgs(args[2:])
	if err != nil {
		return true, err
	}
	reqData, err := base64.RawURLEncoding.DecodeString(reqB64)
	if err != nil {
		return true, err
	}
	var req Request
	if err := json.Unmarshal(reqData, &req); err != nil {
		return true, err
	}
	out, err := json.Marshal(handler(req))
	if err != nil {
		return true, err
	}
	if err := os.WriteFile(resultPath, out, 0600); err != nil {
		return true, err
	}
	return true, nil
}

func RunRequest(req Request, executablePath ExecutablePathFunc, launch LaunchProcessFunc) (Response, error) {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}
	reqB64 := base64.RawURLEncoding.EncodeToString(reqBytes)

	resultPath := filepath.Join(os.TempDir(), fmt.Sprintf("wails-elev-%d.json", time.Now().UnixNano()))
	defer os.Remove(resultPath)

	exePath, err := executablePath()
	if err != nil {
		return Response{}, err
	}
	if err := launch(exePath, reqB64, resultPath); err != nil {
		return Response{}, err
	}

	data, err := os.ReadFile(resultPath)
	if err != nil {
		return Response{}, err
	}

	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return Response{}, err
	}
	if !resp.Success {
		if resp.Error == "" {
			return Response{}, errors.New("elevated operation failed")
		}
		return Response{}, errors.New(resp.Error)
	}
	return resp, nil
}

func (c *IPCClient) Alive() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}

func (c *IPCClient) Dispatch(req Request) (Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return Response{}, errors.New("elevated service not connected")
	}
	if err := c.refreshTokenLocked(); err != nil {
		return Response{}, fmt.Errorf("token refresh: %w", err)
	}

	frame, err := json.Marshal(ipcRequest{Token: c.token, Request: req})
	if err != nil {
		return Response{}, err
	}
	frame = append(frame, '\n')

	if _, err := c.conn.Write(frame); err != nil {
		c.teardownLocked()
		return Response{}, fmt.Errorf("write: %w", err)
	}
	if !c.scanner.Scan() {
		c.teardownLocked()
		if err := c.scanner.Err(); err != nil {
			return Response{}, fmt.Errorf("read: %w", err)
		}
		return Response{}, errors.New("elevated service closed connection")
	}
	var resp ipcResponse
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		return Response{}, fmt.Errorf("decode: %w", err)
	}
	c.opCount++
	if !resp.Response.Success {
		return Response{}, errors.New(resp.Response.Error)
	}
	return resp.Response, nil
}

func (c *IPCClient) Status() SessionStatus {
	if c == nil {
		return SessionStatus{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return SessionStatus{}
	}
	return SessionStatus{Active: true, PID: c.pid, StartedAt: c.startedAt.Unix(), Port: c.port, OpCount: c.opCount}
}

func (c *IPCClient) Stop() {
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

func (c *IPCClient) teardownLocked() {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

func (c *IPCClient) mintJWT() (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(5 * time.Minute)
	claims := &serviceClaims{
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

func (c *IPCClient) refreshTokenLocked() error {
	if time.Until(c.tokenExp) > time.Minute {
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

func newScanner(conn net.Conn) *bufio.Scanner {
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	return scanner
}

func validateJWT(tokenStr string, secret []byte) error {
	claims := &serviceClaims{}
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

func parseHelperArgs(args []string) (requestB64 string, resultPath string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--request":
			if i+1 < len(args) {
				requestB64 = args[i+1]
				i++
			}
		case "--result":
			if i+1 < len(args) {
				resultPath = args[i+1]
				i++
			}
		}
	}
	if requestB64 == "" || resultPath == "" {
		return "", "", errors.New("missing --request/--result for elevated helper")
	}
	return requestB64, resultPath, nil
}

func containsArg(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
}

func argValue(args []string, flag string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}
