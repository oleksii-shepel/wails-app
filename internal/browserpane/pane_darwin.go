//go:build darwin

package browserpane

import "fmt"

type darwinUnsupportedPane struct{}

func newPlatformPane(_ Update, _ func(string), _ func(string)) (pane, error) {
	return nil, fmt.Errorf("native browser pane is not supported on darwin")
}

func (p *darwinUnsupportedPane) Sync(Update) error {
	return fmt.Errorf("native browser pane is not supported on darwin")
}

func (p *darwinUnsupportedPane) CaptureScreenshot() (string, error) {
	return "", fmt.Errorf("native browser pane is not supported on darwin")
}

func (p *darwinUnsupportedPane) GetDocumentBackground() (string, error) {
	return "", fmt.Errorf("native browser pane is not supported on darwin")
}

func (p *darwinUnsupportedPane) PostMessage(string) error {
	return fmt.Errorf("native browser pane is not supported on darwin")
}

func (p *darwinUnsupportedPane) Dispose() error {
	return nil
}
