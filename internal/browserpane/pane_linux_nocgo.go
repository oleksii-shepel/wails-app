//go:build linux && !cgo

package browserpane

import "fmt"

type linuxUnsupportedPane struct{}

func newPlatformPane(_ Update, _ func(string)) (pane, error) {
	return nil, fmt.Errorf("native browser pane requires cgo on linux")
}

func (p *linuxUnsupportedPane) Sync(Update) error {
	return fmt.Errorf("native browser pane requires cgo on linux")
}

func (p *linuxUnsupportedPane) CaptureScreenshot() (string, error) {
	return "", fmt.Errorf("native browser pane requires cgo on linux")
}

func (p *linuxUnsupportedPane) GetDocumentBackground() (string, error) {
	return "", fmt.Errorf("native browser pane requires cgo on linux")
}

func (p *linuxUnsupportedPane) Dispose() error {
	return nil
}
