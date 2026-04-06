//go:build !windows && !darwin && !linux

package browserpane

import "fmt"

type unsupportedPane struct{}

func newPlatformPane(_ string) (pane, error) {
	return nil, fmt.Errorf("native browser pane is not supported on this platform")
}

func (p *unsupportedPane) Sync(Update) error {
	return fmt.Errorf("native browser pane is not supported on this platform")
}

func (p *unsupportedPane) Dispose() error {
	return nil
}
