import { withViewerFrameScrollbars } from './viewer-frame-scrollbars';

describe('viewer frame scrollbars', () => {
  it('adds the transparent-track square-thumb style inside complete documents', () => {
    const document = withViewerFrameScrollbars('<html><head><title>Preview</title></head><body>Content</body></html>');

    expect(document).toContain('background: var(--aether-scrollbar-track) !important;');
    expect(document).toContain('border-radius: 0 !important;');
    expect(document).toContain('--aether-scrollbar-thumb:');
    expect(document.indexOf('data-aether-viewer-scrollbars')).toBeLessThan(document.indexOf('</head>'));
  });

  it('places the stylesheet before a document CSP can block it', () => {
    const document = withViewerFrameScrollbars('<html><head><meta http-equiv="Content-Security-Policy" content="style-src \'self\'"></head><body></body></html>');

    expect(document.indexOf('data-aether-viewer-scrollbars')).toBeLessThan(document.indexOf('Content-Security-Policy'));
  });

  it('wraps HTML fragments in a styled document', () => {
    expect(withViewerFrameScrollbars('<main>Content</main>')).toContain('<head>');
  });
});
