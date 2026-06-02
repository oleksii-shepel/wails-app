package elevation

import (
	"errors"
	"os"
	"testing"
	"time"
)

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
type testEvent struct {
	name string
	data interface{}
}

type mockClient struct {
	alive   bool
	status  SessionStatus
	stopped bool
}

func (m *mockClient) Alive() bool                 { return m.alive }
func (m *mockClient) Dispatch(r Request) (Response, error) { return Response{}, nil }
func (m *mockClient) Status() SessionStatus       { return m.status }
func (m *mockClient) Stop()                       { m.stopped = true }

func TestIsPermissionDenied(t *testing.T) {
	if IsPermissionDenied(nil) {
		t.Fatal("expected false for nil error")
	}
	if !IsPermissionDenied(os.ErrPermission) {
		t.Fatal("expected true for os.ErrPermission")
	}
	if !IsPermissionDenied(errors.New("permission denied")) {
		t.Fatal("expected true for 'permission denied' message")
	}
	if !IsPermissionDenied(errors.New("Access is denied")) {
		t.Fatal("expected true for 'Access is denied' message")
	}
	if !IsPermissionDenied(errors.New("operation not permitted")) {
		t.Fatal("expected true for 'operation not permitted' message")
	}
	if !IsPermissionDenied(errors.New("EACCES")) {
		t.Fatal("expected true for 'EACCES' message")
	}
	if IsPermissionDenied(errors.New("some random error")) {
		t.Fatal("expected false for unrelated error")
	}
}

func TestShouldRetryDeleteWithElevation(t *testing.T) {
	if !ShouldRetryDeleteWithElevation(os.ErrPermission) {
		t.Fatal("expected true for permission denied")
	}
	if !ShouldRetryDeleteWithElevation(errors.New("shfileoperationw returned error code 0x78")) {
		t.Fatal("expected true for SHFileOperation error")
	}
	if ShouldRetryDeleteWithElevation(nil) {
		t.Fatal("expected false for nil error")
	}
	if ShouldRetryDeleteWithElevation(errors.New("random")) {
		t.Fatal("expected false for unrelated error")
	}
}

func TestServiceStartSession(t *testing.T) {
	var events []testEvent
	s := NewService(
		func(name string, data interface{}) { events = append(events, testEvent{name, data}) },
		func(exePath string) (Client, error) {
			return &mockClient{alive: true, status: SessionStatus{Active: true, PID: 123}}, nil
		},
	)
	s.resolveExePath = func() (string, error) { return "/app", nil }

	status, err := s.StartSession()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Active || status.PID != 123 {
		t.Fatalf("unexpected status: %+v", status)
	}

	// Second start should reuse existing client
	status2, err := s.StartSession()
	if err != nil {
		t.Fatalf("unexpected error on re-start: %v", err)
	}
	if status2.PID != 123 {
		t.Fatalf("expected same session, got %+v", status2)
	}
}

func TestServiceStopSession(t *testing.T) {
	var events []testEvent
	client := &mockClient{alive: true}
	s := NewService(
		func(name string, data interface{}) { events = append(events, testEvent{name, data}) },
		nil,
	)
	s.client = client

	s.StopSession()
	if !client.stopped {
		t.Fatal("expected client to be stopped")
	}
	if s.Client() != nil {
		t.Fatal("expected client to be nil after stop")
	}
}

func TestServiceStatus(t *testing.T) {
	s := NewService(nil, nil)
	if s.Status().Active {
		t.Fatal("expected inactive status with no client")
	}

	s.client = &mockClient{status: SessionStatus{Active: true, Port: 9999}}
	if s.Status().Port != 9999 {
		t.Fatal("expected status from client")
	}
}

func TestServiceConfirmAndWait(t *testing.T) {
	s := NewService(nil, nil)

	go func() {
		time.Sleep(50 * time.Millisecond)
		s.Confirm("req-1", true)
	}()

	approved, err := s.WaitForApproval("req-1", time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Fatal("expected approval")
	}
}

func TestServiceWaitTimeout(t *testing.T) {
	s := NewService(nil, nil)
	_, err := s.WaitForApproval("req-timeout", 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestServiceConfirmMissingRequest(t *testing.T) {
	s := NewService(nil, nil)
	// Should not panic
	s.Confirm("missing", true)
}

func TestServiceEmitNoOp(t *testing.T) {
	s := NewService(nil, nil)
	// Should not panic
	s.emit("test", "data")
}
