//go:build !windows && !darwin && !linux

package browserpane

import "fmt"

type unsupportedPane struct{}

func newPlatformPane(_ Update, _ func(string), _ func(string)) (pane, error) {
	return nil, fmt.Errorf("native browser pane is not supported on this platform")
}

func (p *unsupportedPane) Sync(Update) error {
	return fmt.Errorf("native browser pane is not supported on this platform")
}

func (p *unsupportedPane) CaptureScreenshot() (string, error) {
	return "", fmt.Errorf("native browser pane is not supported on this platform")
}

func (p *unsupportedPane) GetDocumentBackground() (string, error) {
	return "", fmt.Errorf("native browser pane is not supported on this platform")
}

func (p *unsupportedPane) PostMessage(string) error {
	return fmt.Errorf("native browser pane is not supported on this platform")
}

func (p *unsupportedPane) Dispose() error {
	return nil
}
