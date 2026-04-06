//go:build darwin && !cgo

package browserpane

import "fmt"

type darwinUnsupportedPane struct{}

func newPlatformPane(_ Update) (pane, error) {
	return nil, fmt.Errorf("native browser pane requires cgo on darwin")
}

func (p *darwinUnsupportedPane) Sync(Update) error {
	return fmt.Errorf("native browser pane requires cgo on darwin")
}

func (p *darwinUnsupportedPane) CaptureScreenshot() (string, error) {
	return "", fmt.Errorf("native browser pane requires cgo on darwin")
}

func (p *darwinUnsupportedPane) GetDocumentBackground() (string, error) {
	return "", fmt.Errorf("native browser pane requires cgo on darwin")
}

func (p *darwinUnsupportedPane) Dispose() error {
	return nil
}
