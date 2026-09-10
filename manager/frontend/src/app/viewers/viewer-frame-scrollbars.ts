/**
 * Scrollbar skin for documents that render in a viewer-owned iframe. Frame
 * documents do not inherit the application's global stylesheet.
 */
const SCROLLBAR_FALLBACKS: Record<string, string> = {
  '--vsc-scrollbar-size': '12px',
  '--vsc-scrollbar-track': 'transparent',
  '--vsc-scrollbar-thumb': 'rgba(200, 200, 200, 0.16)',
  '--vsc-scrollbar-thumb-hover': 'rgba(200, 200, 200, 0.30)',
  '--vsc-scrollbar-thumb-active': 'rgba(200, 200, 200, 0.42)',
};

function activeScrollbarTokens(): Record<string, string> {
  const styles = typeof document === 'undefined' ? null : getComputedStyle(document.documentElement);
  return Object.fromEntries(Object.entries(SCROLLBAR_FALLBACKS).map(([name, fallback]) => [
    name,
    styles?.getPropertyValue(name).trim() || fallback,
  ]));
}

function viewerFrameScrollbarStyle(): string {
  const tokens = activeScrollbarTokens();
  return `<style data-aether-viewer-scrollbars>
  :root {
    --aether-scrollbar-size: ${tokens['--vsc-scrollbar-size']};
    --aether-scrollbar-track: ${tokens['--vsc-scrollbar-track']};
    --aether-scrollbar-thumb: ${tokens['--vsc-scrollbar-thumb']};
    --aether-scrollbar-thumb-hover: ${tokens['--vsc-scrollbar-thumb-hover']};
    --aether-scrollbar-thumb-active: ${tokens['--vsc-scrollbar-thumb-active']};
  }
  html::-webkit-scrollbar,
  body::-webkit-scrollbar,
  *::-webkit-scrollbar {
    width: var(--aether-scrollbar-size) !important;
    height: var(--aether-scrollbar-size) !important;
    background: transparent !important;
  }
  html::-webkit-scrollbar-track,
  body::-webkit-scrollbar-track,
  *::-webkit-scrollbar-track,
  html::-webkit-scrollbar-track-piece,
  body::-webkit-scrollbar-track-piece,
  *::-webkit-scrollbar-track-piece,
  html::-webkit-scrollbar-corner,
  body::-webkit-scrollbar-corner,
  *::-webkit-scrollbar-corner { background: var(--aether-scrollbar-track) !important; }
  html::-webkit-scrollbar-thumb,
  body::-webkit-scrollbar-thumb,
  *::-webkit-scrollbar-thumb {
    min-width: 20px !important;
    min-height: 20px !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: var(--aether-scrollbar-thumb) !important;
    background-clip: border-box !important;
  }
  html::-webkit-scrollbar-thumb:hover,
  body::-webkit-scrollbar-thumb:hover,
  *::-webkit-scrollbar-thumb:hover { background: var(--aether-scrollbar-thumb-hover) !important; }
  html::-webkit-scrollbar-thumb:active,
  body::-webkit-scrollbar-thumb:active,
  *::-webkit-scrollbar-thumb:active { background: var(--aether-scrollbar-thumb-active) !important; }
  *::-webkit-scrollbar-button,
  *::-webkit-scrollbar-button:single-button,
  *::-webkit-scrollbar-button:vertical,
  *::-webkit-scrollbar-button:horizontal,
  *::-webkit-scrollbar-button:vertical:decrement,
  *::-webkit-scrollbar-button:vertical:increment,
  *::-webkit-scrollbar-button:horizontal:decrement,
  *::-webkit-scrollbar-button:horizontal:increment {
    -webkit-appearance: none;
    appearance: none;
    display: none !important;
    width: 0 !important;
    height: 0 !important;
  }
</style>`;
}

export function withViewerFrameScrollbars(source: string): string {
  if (source.includes('data-aether-viewer-scrollbars')) {
    return source;
  }

  const style = viewerFrameScrollbarStyle();
  if (/<head\b[^>]*>/i.test(source)) {
    return source.replace(/<head\b[^>]*>/i, `$&${style}`);
  }

  if (/<\/head\s*>/i.test(source)) {
    return source.replace(/<\/head\s*>/i, `${style}</head>`);
  }

  return `<!doctype html><html><head>${style}</head><body>${source}</body></html>`;
}
