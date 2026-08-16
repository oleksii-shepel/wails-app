import { FileInfo, FileType } from '../../shared/protocol';
import { ViewerArchiveContext } from '../viewer-archive-context';

export const VIEWER_BASE_STYLES = `
  :host {
    display: flex;
    flex-direction: column;
    flex: 1 1 0%;
    min-height: 0;
    min-width: 0;
    overflow: hidden;
  }

  .viewer-surface {
    display: flex;
    flex-direction: column;
    flex: 1 1 0%;
    min-height: 0;
    overflow: hidden;
    background: var(--vsc-editor-background, #1e1e1e);
    color: var(--vsc-foreground, #cccccc);
  }

  .viewer-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 8px 12px;
    border-bottom: 1px solid var(--vsc-border, #454545);
    background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 92%, black);
    flex-shrink: 0;
    min-width: 0;
  }

  .header-main {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    flex: 1;
  }

  .file-icon {
    font-size: 18px;
    flex-shrink: 0;
    line-height: 1;
  }

  .header-text {
    min-width: 0;
    flex: 1;
  }

  .title {
    font-size: 12px;
    font-weight: 700;
    letter-spacing: 0.3px;
    text-transform: uppercase;
    opacity: 0.9;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .path {
    font-family: var(--vsc-font-family-mono, ui-monospace, 'Courier New', monospace);
    font-size: 11px;
    opacity: 0.6;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    margin-top: 1px;
  }

  .header-meta {
    display: flex;
    align-items: center;
    gap: 4px;
    flex-shrink: 0;
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .badge {
    font-size: 10px;
    font-weight: 600;
    padding: 2px 6px;
    border-radius: 3px;
    white-space: nowrap;
    letter-spacing: 0.3px;
  }

  .type-badge {
    background: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 20%, transparent);
    color: var(--vsc-accent-blue, #4da8ff);
    border: 1px solid color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 40%, transparent);
  }

  .archive-badge {
    background: color-mix(in srgb, var(--vsc-accent-orange, #d4a000) 20%, transparent);
    color: var(--vsc-accent-orange, var(--vsc-warning));
    border: 1px solid color-mix(in srgb, var(--vsc-accent-orange, #d4a000) 40%, transparent);
  }

  .size-badge,
  .dim-badge,
  .line-badge,
  .meta-badge {
    background: color-mix(in srgb, var(--vsc-foreground, #ccc) 8%, transparent);
    color: color-mix(in srgb, var(--vsc-foreground, #ccc) 60%, transparent);
    border: 1px solid color-mix(in srgb, var(--vsc-border, #454545) 50%, transparent);
  }

  .viewer-toolbar {
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 4px 8px;
    border-bottom: 1px solid var(--vsc-border, #454545);
    background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 96%, black);
    flex-shrink: 0;
  }

  .tool-btn {
    background: transparent;
    border: 1px solid transparent;
    color: var(--vsc-foreground, #ccc);
    font-size: 12px;
    padding: 2px 8px;
    border-radius: 3px;
    cursor: pointer;
    line-height: 1.4;
    min-width: 24px;
  }

  .tool-btn:hover {
    background: var(--vsc-list-hover-background, rgba(255,255,255,0.05));
    border-color: var(--vsc-border, #454545);
  }

  .tool-btn.active {
    background: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 20%, transparent);
    border-color: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 50%, transparent);
    color: var(--vsc-accent-blue, #4da8ff);
  }

  .zoom-label {
    font-size: 11px;
    min-width: 44px;
    text-align: center;
    opacity: 0.8;
    font-variant-numeric: tabular-nums;
  }

  .toolbar-divider {
    width: 1px;
    height: 16px;
    background: var(--vsc-border, #454545);
    margin: 0 4px;
  }

  .viewer-body {
    flex: 1 1 0%;
    min-height: 0;
    overflow: auto;
    position: relative;
  }

  .state-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    min-height: 160px;
    gap: 8px;
    padding: 24px;
    text-align: center;
  }

  .state-icon {
    font-size: 36px;
    line-height: 1;
    opacity: 0.55;
  }

  .state-label {
    font-size: 13px;
    opacity: 0.65;
  }

  .state-sub {
    font-size: 11px;
    opacity: 0.4;
  }

  .state-error .state-icon {
    opacity: 1;
  }

  .state-error .state-label {
    color: var(--vsc-error);
    opacity: 1;
  }

  .error-detail {
    font-size: 11px;
    font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
    color: color-mix(in srgb, #ff7070 70%, var(--vsc-foreground, #ccc));
    background: rgba(255, 0, 0, 0.07);
    border: 1px solid rgba(255, 112, 112, 0.2);
    border-radius: 4px;
    padding: 8px 12px;
    max-width: 480px;
    white-space: pre-wrap;
    word-break: break-word;
  }

  .spinner {
    width: 26px;
    height: 26px;
    border: 2px solid color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 25%, transparent);
    border-top-color: var(--vsc-accent-blue, #0078d4);
    border-radius: 50%;
    animation: builtin-viewer-spin 0.65s linear infinite;
  }

  @keyframes builtin-viewer-spin {
    to { transform: rotate(360deg); }
  }
`;

export function buildDisplayName(info: FileInfo | null, targetPath: string, archiveContext: ViewerArchiveContext | null): string {
  if (info?.name) {
    return info.name;
  }
  if (archiveContext?.entryPath) {
    return baseName(archiveContext.entryPath);
  }
  return targetPath ? baseName(targetPath) : '';
}

export function baseName(path: string): string {
  return (path ?? '').replace(/\\/g, '/').split('/').pop() || path;
}

export function extractExt(path: string): string {
  const normalized = (path ?? '').replace(/\\/g, '/');
  const last = normalized.split('/').pop() ?? normalized;
  const index = last.lastIndexOf('.');
  return index >= 0 ? last.slice(index + 1).toLowerCase() : '';
}

export function formatSize(bytes: number): string {
  if (!bytes || bytes <= 0) {
    return '';
  }
  const units = ['B', 'KB', 'MB', 'GB'];
  const unitIndex = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / Math.pow(1024, unitIndex);
  return `${unitIndex === 0 ? value : value.toFixed(1)} ${units[unitIndex]}`;
}

export function makeSyntheticInfo(name: string, path: string, size: number): FileInfo {
  return {
    name,
    path,
    size,
    type: FileType.FILE,
    modified: 0,
    created: 0,
    accessed: 0,
    permissions: '',
    isHidden: false,
  } as FileInfo;
}

export function sanitizeSvg(text: string): string {
  return (text ?? '')
    .replace(/<\?xml[^>]*\?>/gi, '')
    .replace(/<!DOCTYPE[^>]*>/gi, '')
    .trim();
}

export function inferImageMime(ext: string): string | null {
  return ({
    png: 'image/png',
    jpg: 'image/jpeg',
    jpeg: 'image/jpeg',
    gif: 'image/gif',
    webp: 'image/webp',
    bmp: 'image/bmp',
    ico: 'image/x-icon',
    tif: 'image/tiff',
    tiff: 'image/tiff',
    avif: 'image/avif',
    psd: 'image/vnd.adobe.photoshop',
    ai: 'image/application/postscript',
    heic: 'image/heic',
    heif: 'image/heif',
  } as Record<string, string>)[ext] ?? null;
}

export function inferVideoMime(ext: string): string | null {
  return ({
    mp4: 'video/mp4',
    m4v: 'video/mp4',
    webm: 'video/webm',
    ogv: 'video/ogg',
    mov: 'video/quicktime',
    qt: 'video/quicktime',
    mkv: 'video/x-matroska',
    avi: 'video/x-msvideo',
    divx: 'video/x-msvideo',
    xvid: 'video/x-msvideo',
    flv: 'video/x-flv',
    ts: 'video/mp2t',
    m2ts: 'video/mp2t',
    mts: 'video/mp2t',
    mpg: 'video/mpeg',
    mpeg: 'video/mpeg',
    vob: 'video/mpeg',
    m2p: 'video/mpeg',
    m2v: 'video/mpeg',
    mjpeg: 'video/x-motion-jpeg',
    mjpg: 'video/x-motion-jpeg',
    m3u8: 'application/x-mpegURL',
    mxf: 'application/mxf',
    wmv: 'video/x-ms-wmv',
  } as Record<string, string>)[ext] ?? null;
}

export function inferAudioMime(ext: string): string | null {
  return ({
    mp3: 'audio/mpeg',
    wav: 'audio/wav',
    ogg: 'audio/ogg',
    flac: 'audio/flac',
    aac: 'audio/aac',
    m4a: 'audio/mp4',
  } as Record<string, string>)[ext] ?? null;
}

export function isTextExtension(ext: string): boolean {
  return new Set([
    'txt', 'log', 'csv', 'tsv', 'ini', 'cfg', 'conf', 'yaml', 'yml', 'toml',
    'xml', 'html', 'htm', 'css', 'scss', 'sass', 'less',
    'js', 'ts', 'jsx', 'tsx', 'vue', 'svelte',
    'py', 'rb', 'go', 'rs', 'java', 'kt', 'swift', 'c', 'cpp', 'h', 'hpp',
    'cs', 'php', 'sh', 'bash', 'zsh', 'fish', 'ps1', 'bat', 'cmd',
    'sql', 'graphql', 'gql', 'proto', 'dockerfile', 'gitignore', 'env',
    'lock', 'diff', 'patch', 'r', 'lua', 'm', 'pl', 'ex', 'exs',
  ]).has(ext);
}

export function looksLikeText(content: string): boolean {
  const sample = content.slice(0, 1024);
  let controlCount = 0;
  for (let index = 0; index < sample.length; index++) {
    const code = sample.charCodeAt(index);
    if (code < 9 || (code > 13 && code < 32)) {
      controlCount++;
    }
  }
  return sample.length === 0 || controlCount / sample.length < 0.05;
}

export function syntaxHighlightJson(json: string): string {
  return json
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(
      /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g,
      (match) => {
        if (/^"/.test(match)) {
          return match.endsWith(':')
            ? `<span class="json-key">${match}</span>`
            : `<span class="json-str">${match}</span>`;
        }
        if (/true|false/.test(match)) {
          return `<span class="json-bool">${match}</span>`;
        }
        if (/null/.test(match)) {
          return `<span class="json-null">${match}</span>`;
        }
        return `<span class="json-num">${match}</span>`;
      },
    );
}

export function renderMarkdown(markdown: string): string {
  let html = markdown
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/```[\w]*\n([\s\S]*?)```/g, (_match, code) => `<pre><code>${code}</code></pre>`)
    .replace(/^###### (.+)$/gm, '<h6>$1</h6>')
    .replace(/^##### (.+)$/gm, '<h5>$1</h5>')
    .replace(/^#### (.+)$/gm, '<h4>$1</h4>')
    .replace(/^### (.+)$/gm, '<h3>$1</h3>')
    .replace(/^## (.+)$/gm, '<h2>$1</h2>')
    .replace(/^# (.+)$/gm, '<h1>$1</h1>')
    .replace(/^---+$/gm, '<hr>')
    .replace(/^&gt; (.+)$/gm, '<blockquote>$1</blockquote>')
    .replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>')
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    .replace(/__(.+?)__/g, '<strong>$1</strong>')
    .replace(/_(.+?)_/g, '<em>$1</em>')
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/~~(.+?)~~/g, '<del>$1</del>')
    .replace(/!\[([^\]]*)\]\(([^)]+)\)/g, '<img src="$2" alt="$1">')
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>')
    .replace(/^\s*[-*+] (.+)$/gm, '<li>$1</li>')
    .replace(/^\s*\d+\. (.+)$/gm, '<li>$1</li>')
    .replace(/(<li>.*<\/li>\n?)+/gs, (match) => `<ul>${match}</ul>`)
    .replace(/\n{2,}/g, '</p><p>');

  if (!/^<(h[1-6]|ul|ol|pre|blockquote|hr)/.test(html.trim())) {
    html = `<p>${html}</p>`;
  }
  return html;
}