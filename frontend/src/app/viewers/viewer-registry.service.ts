import { Injectable, Type } from '@angular/core';
import { ViewerContribution, ViewerRequest } from './viewer-contribution';
import { ViewerSurfaceComponent } from '../surfaces/viewer/viewer-surface.component';

function extLower(path: string): string {
  const p = (path ?? '').replace(/\\/g, '/');
  const i = p.lastIndexOf('.');
  if (i < 0) return '';
  return p.slice(i + 1).toLowerCase();
}

@Injectable({ providedIn: 'root' })
export class ViewerRegistryService {
  private readonly contributions: ViewerContribution[] = [];
  private readonly loaded = new Map<string, Type<unknown>>();

  constructor() {
    // Built-ins. This is the "spine" we can extend later with external bundles.
    this.register({
      id: 'legacy',
      title: 'Legacy Viewer',
      score: () => 1, // fallback for everything
      load: () => ViewerSurfaceComponent,
    });

    this.register({
      id: 'image',
      title: 'Image Viewer',
      score: (req) => {
        const ext = extLower(req.archiveContext?.entryPath ?? req.targetPath);
        // Native browser support
        if (ext === 'svg') return 50;
        if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'ico', 'avif'].includes(ext)) return 50;
        // Convertible formats (with ImageMagick on backend)
        if (['psd', 'ai', 'eps', 'xcf', 'tiff', 'tif', 'dng', 'tga', 'icns', 'pnm', 'dds', 'xwd', 'scr', 'xpm', 'exr', 'heif', 'heic', 'jp2', 'j2k', 'age'].includes(ext)) return 40;
        return 0;
      },
      load: async () => (await import('./builtins/image-viewer.component')).ImageViewerComponent,
    });

    this.register({
      id: 'video',
      title: 'Video Viewer',
      score: (req) => {
        const ext = extLower(req.archiveContext?.entryPath ?? req.targetPath);
        return ['mp4', 'm4v', 'webm', 'ogv', 'mov', 'qt', 'mkv', 'avi', 'divx', 'xvid', 'flv', 'ts', 'm2ts', 'mts', 'mpg', 'mpeg', 'vob', 'm2p', 'm2v', 'mjpeg', 'mjpg', 'm3u8', 'mxf', 'wmv'].includes(ext) ? 50 : 0;
      },
      load: async () => (await import('./builtins/video-viewer.component')).VideoViewerComponent,
    });

    this.register({
      id: 'audio',
      title: 'Audio Viewer',
      score: (req) => {
        const ext = extLower(req.archiveContext?.entryPath ?? req.targetPath);
        return ['mp3', 'wav', 'ogg', 'flac', 'aac', 'm4a'].includes(ext) ? 50 : 0;
      },
      load: async () => (await import('./builtins/audio-viewer.component')).AudioViewerComponent,
    });

    this.register({
      id: 'pdf',
      title: 'PDF Viewer',
      score: (req) => extLower(req.archiveContext?.entryPath ?? req.targetPath) === 'pdf' ? 50 : 0,
      load: async () => (await import('./builtins/pdf-viewer.component')).PdfViewerComponent,
    });

    this.register({
      id: 'json',
      title: 'JSON Viewer',
      score: (req) => ['json', 'jsonc'].includes(extLower(req.archiveContext?.entryPath ?? req.targetPath)) ? 60 : 0,
      load: async () => (await import('./builtins/json-viewer.component')).JsonViewerComponent,
    });

    this.register({
      id: 'markdown',
      title: 'Markdown Viewer',
      score: (req) => ['md', 'markdown', 'mdx'].includes(extLower(req.archiveContext?.entryPath ?? req.targetPath)) ? 60 : 0,
      load: async () => (await import('./builtins/markdown-viewer.component')).MarkdownViewerComponent,
    });

    this.register({
      id: 'text',
      title: 'Text Viewer',
      score: (req) => {
        const ext = extLower(req.archiveContext?.entryPath ?? req.targetPath);
        return ['txt', 'log', 'csv', 'tsv', 'ini', 'cfg', 'conf', 'yaml', 'yml', 'toml', 'xml', 'html', 'htm', 'css', 'scss', 'sass', 'less', 'js', 'ts', 'jsx', 'tsx', 'vue', 'svelte', 'py', 'rb', 'go', 'rs', 'java', 'kt', 'swift', 'c', 'cpp', 'h', 'hpp', 'cs', 'php', 'sh', 'bash', 'zsh', 'fish', 'ps1', 'bat', 'cmd', 'sql', 'graphql', 'gql', 'proto', 'dockerfile', 'gitignore', 'env', 'lock', 'diff', 'patch', 'r', 'lua', 'm', 'pl', 'ex', 'exs'].includes(ext) ? 40 : 0;
      },
      load: async () => (await import('./builtins/text-viewer.component')).TextViewerComponent,
    });
  }

  register(c: ViewerContribution): void {
    // Replace-by-id to support hot-reload/dev and later plugin updates.
    const i = this.contributions.findIndex(x => x.id === c.id);
    if (i >= 0) this.contributions.splice(i, 1, c);
    else this.contributions.push(c);
  }

  /**
   * Resolve a viewer component for the request.
   * If viewerId is provided and registered, it is forced.
   */
  async resolve(req: ViewerRequest): Promise<{ id: string; component: Type<unknown> }> {
    const best = this.pickContribution(req);
    return { id: best.id, component: await this.loadContribution(best) };
  }

  resolveId(req: ViewerRequest): string {
    return this.pickContribution(req).id;
  }

  private pickContribution(req: ViewerRequest): ViewerContribution {
    const forced = (req.viewerId ?? '').trim();
    if (forced) {
      const c = this.contributions.find(x => x.id === forced);
      if (c) return c;
    }

    let best: ViewerContribution | null = null;
    let bestScore = 0;
    for (const c of this.contributions) {
      const s = c.score(req);
      if (s > bestScore) {
        bestScore = s;
        best = c;
      }
    }
    if (!best) {
      // Should be unreachable due to legacy fallback registration.
      best = this.contributions[0];
    }
    return best;
  }

  private async loadContribution(c: ViewerContribution): Promise<Type<unknown>> {
    const cached = this.loaded.get(c.id);
    if (cached) return cached;
    const loaded = await Promise.resolve(c.load()) as Type<unknown>;
    this.loaded.set(c.id, loaded);
    return loaded;
  }
}
