package plugin

import (
	"context"

	contracts "wails-app/internal/contracts"
)

// OperationHook allows plugins to intercept and extend file operations.
// Hooks run in registration order. A Before* method returning an error
// cancels the operation.
type OperationHook interface {
	Plugin

	BeforeCopy(ctx context.Context, src, dst string) error
	AfterCopy(ctx context.Context, src, dst string)

	BeforeMove(ctx context.Context, src, dst string) error
	AfterMove(ctx context.Context, src, dst string)

	BeforeDelete(ctx context.Context, path string) error
	AfterDelete(ctx context.Context, path string)

	BeforeMkdir(ctx context.Context, path string) error
	AfterMkdir(ctx context.Context, path string)
}

// NoOpOperationHook is an embeddable helper for plugins that only want to
// override a subset of hooks.
type NoOpOperationHook struct{}

func (NoOpOperationHook) BeforeCopy(ctx context.Context, src, dst string) error { return nil }
func (NoOpOperationHook) AfterCopy(ctx context.Context, src, dst string)         {}
func (NoOpOperationHook) BeforeMove(ctx context.Context, src, dst string) error { return nil }
func (NoOpOperationHook) AfterMove(ctx context.Context, src, dst string)         {}
func (NoOpOperationHook) BeforeDelete(ctx context.Context, path string) error   { return nil }
func (NoOpOperationHook) AfterDelete(ctx context.Context, path string)           {}
func (NoOpOperationHook) BeforeMkdir(ctx context.Context, path string) error    { return nil }
func (NoOpOperationHook) AfterMkdir(ctx context.Context, path string)            {}

// NoOpOperationHook is designed to be embedded into a struct that already
// implements Plugin. It does not satisfy OperationHook on its own because
// OperationHook embeds Plugin.

// CompositeOperationHook runs a list of hooks in order.
type CompositeOperationHook []OperationHook

func (c CompositeOperationHook) runBefore(ctx context.Context, fn func(OperationHook) error) error {
	for _, h := range c {
		if err := fn(h); err != nil {
			return err
		}
	}
	return nil
}

func (c CompositeOperationHook) runAfter(ctx context.Context, fn func(OperationHook)) {
	for _, h := range c {
		fn(h)
	}
}

func (c CompositeOperationHook) BeforeCopy(ctx context.Context, src, dst string) error {
	return c.runBefore(ctx, func(h OperationHook) error { return h.BeforeCopy(ctx, src, dst) })
}
func (c CompositeOperationHook) AfterCopy(ctx context.Context, src, dst string) {
	c.runAfter(ctx, func(h OperationHook) { h.AfterCopy(ctx, src, dst) })
}
func (c CompositeOperationHook) BeforeMove(ctx context.Context, src, dst string) error {
	return c.runBefore(ctx, func(h OperationHook) error { return h.BeforeMove(ctx, src, dst) })
}
func (c CompositeOperationHook) AfterMove(ctx context.Context, src, dst string) {
	c.runAfter(ctx, func(h OperationHook) { h.AfterMove(ctx, src, dst) })
}
func (c CompositeOperationHook) BeforeDelete(ctx context.Context, path string) error {
	return c.runBefore(ctx, func(h OperationHook) error { return h.BeforeDelete(ctx, path) })
}
func (c CompositeOperationHook) AfterDelete(ctx context.Context, path string) {
	c.runAfter(ctx, func(h OperationHook) { h.AfterDelete(ctx, path) })
}
func (c CompositeOperationHook) BeforeMkdir(ctx context.Context, path string) error {
	return c.runBefore(ctx, func(h OperationHook) error { return h.BeforeMkdir(ctx, path) })
}
func (c CompositeOperationHook) AfterMkdir(ctx context.Context, path string) {
	c.runAfter(ctx, func(h OperationHook) { h.AfterMkdir(ctx, path) })
}

// OperationResultAdapter adapts the hook signature to contracts.OperationResult.
func OperationResultAdapter(success bool, msg string, paths ...string) contracts.OperationResult {
	return contracts.OperationResult{
		Success:       success,
		Message:       msg,
		AffectedPaths: paths,
	}
}