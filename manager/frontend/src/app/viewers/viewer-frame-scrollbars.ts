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
  *::-webkit-scrollbar {
    width: var(--aether-scrollbar-size);
    height: var(--aether-scrollbar-size);
    background: transparent;
  }
  *::-webkit-scrollbar-track,
  *::-webkit-scrollbar-track-piece,
  *::-webkit-scrollbar-corner { background: var(--aether-scrollbar-track); }
  *::-webkit-scrollbar-thumb {
    min-width: 20px;
    min-height: 20px;
    border: 0;
    border-radius: 0;
    background: var(--aether-scrollbar-thumb);
    background-clip: border-box;
  }
  *::-webkit-scrollbar-thumb:hover { background: var(--aether-scrollbar-thumb-hover); }
  *::-webkit-scrollbar-thumb:active { background: var(--aether-scrollbar-thumb-active); }
  *::-webkit-scrollbar-button,
  *::-webkit-scrollbar-button:single-button,
  *::-webkit-scrollbar-button:vertical,
  *::-webkit-scrollbar-button:horizontal {
    -webkit-appearance: none;
    appearance: none;
    display: none;
    width: 0;
    height: 0;
  }
</style>`;
}

export function withViewerFrameScrollbars(source: string): string {
  if (source.includes('data-aether-viewer-scrollbars')) {
    return source;
  }

  const style = viewerFrameScrollbarStyle();
  if (/<\/head\s*>/i.test(source)) {
    return source.replace(/<\/head\s*>/i, `${style}</head>`);
  }

  if (/<head\b[^>]*>/i.test(source)) {
    return source.replace(/<head\b[^>]*>/i, `$&${style}`);
  }

  return `<!doctype html><html><head>${style}</head><body>${source}</body></html>`;
}
