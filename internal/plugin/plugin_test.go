package plugin

import (
	"context"
	"errors"
	"testing"

	"wails-app/internal/contracts"
)

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
type mockPlugin struct {
	info      Info
	initErr   error
	shutdown  bool
}

func (m *mockPlugin) Info() Info                         { return m.info }
func (m *mockPlugin) Init(config map[string]any) error   { return m.initErr }
func (m *mockPlugin) Shutdown() error                    { m.shutdown = true; return nil }

type mockCommandPlugin struct {
	mockPlugin
	applicable map[string]bool
	execErr    error
}

func (m *mockCommandPlugin) Commands() []CommandInfo { return m.info.Commands }
func (m *mockCommandPlugin) IsApplicable(commandID string, paths []string) bool {
	return m.applicable[commandID]
}
func (m *mockCommandPlugin) Execute(ctx context.Context, commandID string, paths []string) error {
	return m.execErr
}

type mockPreviewPlugin struct {
	mockPlugin
	canPreview bool
	priority   int
}

func (m *mockPreviewPlugin) CanPreview(mimeType, ext string) bool { return m.canPreview }
func (m *mockPreviewPlugin) Priority() int                        { return m.priority }
func (m *mockPreviewPlugin) GenerateThumbnail(ctx context.Context, path string) ([]byte, error) {
	return nil, nil
}
func (m *mockPreviewPlugin) Convert(ctx context.Context, path string, targetFormat string) ([]byte, error) {
	return nil, nil
}

type mockHookPlugin struct {
	mockPlugin
	beforeCopyErr error
}

func (m *mockHookPlugin) BeforeCopy(ctx context.Context, src, dst string) error { return m.beforeCopyErr }
func (m *mockHookPlugin) AfterCopy(ctx context.Context, src, dst string)         {}
func (m *mockHookPlugin) BeforeMove(ctx context.Context, src, dst string) error { return nil }
func (m *mockHookPlugin) AfterMove(ctx context.Context, src, dst string)         {}
func (m *mockHookPlugin) BeforeDelete(ctx context.Context, path string) error   { return nil }
func (m *mockHookPlugin) AfterDelete(ctx context.Context, path string)           {}
func (m *mockHookPlugin) BeforeMkdir(ctx context.Context, path string) error    { return nil }
func (m *mockHookPlugin) AfterMkdir(ctx context.Context, path string)            {}

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
type OperationResult = contracts.OperationResult

func TestRegistryRegisterRequiresID(t *testing.T) {
	r := NewRegistry()
	p := &mockPlugin{info: Info{ID: ""}}
	if err := r.Register(p); err == nil {
		t.Fatal("expected error for empty plugin ID")
	}
}

func TestRegistryRegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	p := &mockPlugin{info: Info{ID: "dup"}}
	if err := r.Register(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := r.Register(p); err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestRegistryListAndGet(t *testing.T) {
	r := NewRegistry()
	p1 := &mockPlugin{info: Info{ID: "a", Name: "Alpha"}}
	p2 := &mockPlugin{info: Info{ID: "b", Name: "Beta"}}
	_ = r.Register(p1)
	_ = r.Register(p2)

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(list))
	}

	if r.Get("a") != p1 {
		t.Fatal("expected Get(a) to return p1")
	}
	if r.Get("missing") != nil {
		t.Fatal("expected Get(missing) to return nil")
	}
}

func TestRegistryShutdown(t *testing.T) {
	r := NewRegistry()
	p := &mockPlugin{info: Info{ID: "s"}}
	_ = r.Register(p)
	r.Shutdown()
	if !p.shutdown {
		t.Fatal("expected plugin to be shutdown")
	}
}

func TestRegistryCommandPlugin(t *testing.T) {
	r := NewRegistry()
	cp := &mockCommandPlugin{
		mockPlugin: mockPlugin{info: Info{ID: "cmd", Commands: []CommandInfo{{ID: "open", Label: "Open"}}}},
		applicable: map[string]bool{"open": true},
	}
	_ = r.Register(cp)

	cmds := r.CommandPlugins()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command plugin, got %d", len(cmds))
	}
	if cmds["cmd"] != cp {
		t.Fatal("expected command plugin to be registered")
	}

	if err := r.InvokeCommand(context.Background(), "cmd", "open", []string{"/a"}); err != nil {
		t.Fatalf("invoke failed: %v", err)
	}
	if err := r.InvokeCommand(context.Background(), "missing", "open", nil); err == nil {
		t.Fatal("expected error for missing plugin")
	}
}

func TestRegistryPreviewPlugin(t *testing.T) {
	r := NewRegistry()
	pp := &mockPreviewPlugin{
		mockPlugin: mockPlugin{info: Info{ID: "prev"}},
		canPreview: true,
		priority:   10,
	}
	_ = r.Register(pp)

	previews := r.PreviewPlugins()
	if len(previews) != 1 {
		t.Fatalf("expected 1 preview plugin, got %d", len(previews))
	}
}

func TestRegistryOperationHook(t *testing.T) {
	r := NewRegistry()
	h := &mockHookPlugin{mockPlugin: mockPlugin{info: Info{ID: "hook"}}}
	_ = r.Register(h)

	hooks := r.OperationHooks()
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}
}

func TestPreviewPicker(t *testing.T) {
	low := &mockPreviewPlugin{mockPlugin: mockPlugin{info: Info{ID: "low"}}, canPreview: true, priority: 5}
	high := &mockPreviewPlugin{mockPlugin: mockPlugin{info: Info{ID: "high"}}, canPreview: true, priority: 10}
	no := &mockPreviewPlugin{mockPlugin: mockPlugin{info: Info{ID: "no"}}, canPreview: false, priority: 100}

	picker := NewPreviewPicker([]PreviewPlugin{low, high, no})
	best := picker.Pick("image/png", ".png")
	if best != high {
		t.Fatal("expected high-priority plugin to be picked")
	}

	picker2 := NewPreviewPicker([]PreviewPlugin{no})
	if picker2.Pick("text/plain", ".txt") != nil {
		t.Fatal("expected nil when no plugin can preview")
	}
}

func TestCommandResultToOperationResult(t *testing.T) {
	cr := CommandResult{Success: true, Message: "Done"}
	or := cr.ToOperationResult("/a", "/b")
	if !or.Success || or.Message != "Done" {
		t.Fatalf("unexpected operation result: %+v", or)
	}
	if len(or.AffectedPaths) != 2 || or.AffectedPaths[0] != "/a" {
		t.Fatal("affected paths mismatch")
	}
}

func TestOperationResultAdapter(t *testing.T) {
	or := OperationResultAdapter(false, "fail", "/x")
	if or.Success || or.Message != "fail" || len(or.AffectedPaths) != 1 {
		t.Fatalf("unexpected result: %+v", or)
	}
}

func TestNoOpOperationHook(t *testing.T) {
	var h NoOpOperationHook
	ctx := context.Background()
	if err := h.BeforeCopy(ctx, "a", "b"); err != nil {
		t.Fatal("expected no error")
	}
	if err := h.BeforeMove(ctx, "a", "b"); err != nil {
		t.Fatal("expected no error")
	}
	if err := h.BeforeDelete(ctx, "a"); err != nil {
		t.Fatal("expected no error")
	}
	if err := h.BeforeMkdir(ctx, "a"); err != nil {
		t.Fatal("expected no error")
	}
	// After* methods are no-ops (compile check)
	h.AfterCopy(ctx, "a", "b")
	h.AfterMove(ctx, "a", "b")
	h.AfterDelete(ctx, "a")
	h.AfterMkdir(ctx, "a")
}

func TestCompositeOperationHookBeforeStopsOnError(t *testing.T) {
	h1 := &mockHookPlugin{mockPlugin: mockPlugin{info: Info{ID: "h1"}}}
	h2 := &mockHookPlugin{mockPlugin: mockPlugin{info: Info{ID: "h2"}}, beforeCopyErr: errors.New("stop")}
	h3 := &mockHookPlugin{mockPlugin: mockPlugin{info: Info{ID: "h3"}}}

	comp := CompositeOperationHook{h1, h2, h3}
	if err := comp.BeforeCopy(context.Background(), "a", "b"); err == nil {
		t.Fatal("expected error from h2")
	}

	// After methods should all run regardless
	comp.AfterCopy(context.Background(), "a", "b")
}

func TestCompositeOperationHookRunsAll(t *testing.T) {
	h1 := &mockHookPlugin{mockPlugin: mockPlugin{info: Info{ID: "h1"}}}
	h2 := &mockHookPlugin{mockPlugin: mockPlugin{info: Info{ID: "h2"}}}

	comp := CompositeOperationHook{h1, h2}
	if err := comp.BeforeMove(context.Background(), "a", "b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryVFSPluginsReturnsCopy(t *testing.T) {
	r := NewRegistry()
	v1 := r.VFSPlugins()
	if len(v1) != 0 {
		t.Fatal("expected empty VFS plugins")
	}
}

func TestRegistryCommandPluginsReturnsCopy(t *testing.T) {
	r := NewRegistry()
	cp := &mockCommandPlugin{mockPlugin: mockPlugin{info: Info{ID: "c"}}}
	_ = r.Register(cp)
	c1 := r.CommandPlugins()
	delete(c1, "c")
	if r.Get("c") == nil {
		t.Fatal("expected original registry to be unchanged")
	}
}
