import { Injectable, Type } from '@angular/core';
import { ContributionRegistry } from '../plugin/contribution-registry';
import { ViewerSurfaceComponent } from '../surfaces/viewer/viewer-surface.component';
import { AudioViewerComponent } from './builtins/audio-viewer.component';
import { ViewerContribution, ViewerRequest } from './viewer-contribution';

function extLower(path: string): string {
  const p = (path ?? '').replace(/\\/g, '/');
  const i = p.lastIndexOf('.');
  if (i < 0) return '';
  return p.slice(i + 1).toLowerCase();
}

function previewPath(req: ViewerRequest): string {
  return req.archiveContext?.entryPath || req.targetPath;
}

function normalizeMime(mime: string | null | undefined): string {
  return (mime ?? '').split(';', 1)[0].trim().toLowerCase();
}

function probeLooksFont(req: ViewerRequest): boolean {
  const mime = normalizeMime(req.probe?.mimeType);
  return mime.startsWith('font/')
    || mime === 'application/font-woff'
    || mime === 'application/x-font-woff'
    || mime === 'application/vnd.ms-fontobject'
    || mime === 'application/x-font-ttf'
    || mime === 'application/x-font-otf'
    || mime === 'application/octet-stream' && ['ttf', 'ttc', 'otf', 'otc', 'woff', 'woff2', 'eot'].includes(extLower(previewPath(req)));
}

function probeLooksText(req: ViewerRequest): boolean {
  const mime = normalizeMime(req.probe?.mimeType);
  if (mime.startsWith('text/')) return true;
  if (mime.endsWith('+json') || mime.endsWith('+xml')) return true;
  return !!req.probe?.encoding;
}

function probeLooksTransportStream(req: ViewerRequest): boolean {
  const mime = normalizeMime(req.probe?.mimeType);
  return mime === 'video/mp2t' || mime === 'application/mp2t';
}

function probeLooksBinary(req: ViewerRequest): boolean {
  const probe = req.probe;
  if (!probe) return false;
  if (probe.isArchive) return false;
  return !probe.isText;
}

@Injectable({ providedIn: 'root' })
export class ViewerRegistryService {
  readonly registry = new ContributionRegistry<ViewerContribution, ViewerRequest>();

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
        const ext = extLower(previewPath(req));
        if (ext === 'svg') return 50;
        if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'ico', 'avif'].includes(ext)) return 50;
        if (['tiff', 'tif', 'exr', 'heif', 'heic', 'jp2', 'j2k', 'avif', 'jxl', 'fits', 'hdr', 'pfm'].includes(ext)) return 40;
        return 0;
      },
      load: async () => (await import('./builtins/image-viewer.component')).ImageViewerComponent,
    });

    this.register({
      id: 'video',
      title: 'Video Viewer',
      score: (req) => {
        const ext = extLower(previewPath(req));
        if (ext === 'ts') {
          if (probeLooksText(req)) return 0;
          if (probeLooksTransportStream(req)) return 70;
        }
        return ['mp4', 'm4v', 'webm', 'ogv', 'mov', 'qt', 'mkv', 'avi', 'divx', 'xvid', 'flv', 'ts', 'm2ts', 'mts', 'mpg', 'mpeg', 'vob', 'm2p', 'm2v', 'mjpeg', 'mjpg', 'm3u8', 'mxf', 'wmv'].includes(ext) ? 50 : 0;
      },
      load: async () => (await import('./builtins/video-viewer.component')).VideoViewerComponent,
    });

    this.register({
      id: 'audio',
      title: 'Audio Viewer',
      score: (req) => {
        const ext = extLower(previewPath(req));
        return ['mp3', 'wav', 'ogg', 'flac', 'aac', 'm4a'].includes(ext) ? 50 : 0;
      },
      load: () => AudioViewerComponent,
    });

    this.register({
      id: 'font',
      title: 'Font Viewer',
      score: (req) => {
        const ext = extLower(previewPath(req));
        if (['ttf', 'ttc', 'otf', 'otc', 'woff', 'woff2', 'eot'].includes(ext)) return 72;
        return probeLooksFont(req) ? 72 : 0;
      },
      load: async () => (await import('./builtins/font-viewer.component')).FontViewerComponent,
    });

    this.register({
      id: 'office',
      title: 'Document Viewer',
      score: (req) => {
        const ext = extLower(previewPath(req));
        return ['pdf', 'doc', 'docx', 'docm', 'xls', 'xlsx', 'xlsm', 'ppt', 'pptx', 'pptm', 'odt', 'ods', 'odp', 'rtf', 'epub', 'djvu', 'djv'].includes(ext) ? 45 : 0;
      },
      load: async () => (await import('./builtins/office-viewer.component')).OfficeViewerComponent,
    });

    this.register({
      id: 'markdown',
      title: 'Markup Viewer',
      score: (req) => ['md', 'markdown', 'mdx', 'html', 'htm'].includes(extLower(previewPath(req))) ? 60 : 0,
      load: async () => (await import('./builtins/markdown-viewer.component')).MarkdownViewerComponent,
    });

    this.register({
      id: 'binary',
      title: 'Binary Viewer',
      score: (req) => probeLooksBinary(req) ? 30 : 0,
      load: async () => (await import('./builtins/binary-viewer.component')).BinaryViewerComponent,
    });

    this.register({
      id: 'text',
      title: 'Code Viewer',
      score: (req) => {
        const ext = extLower(previewPath(req));
        if (ext === 'ts') {
          if (probeLooksTransportStream(req)) return 0;
          if (probeLooksText(req)) return 65;
        }
        if (['json', 'jsonc'].includes(ext)) return 60;
        return ['txt', 'log', 'csv', 'tsv', 'ini', 'cfg', 'conf', 'yaml', 'yml', 'toml', 'xml', 'html', 'htm', 'css', 'scss', 'sass', 'less', 'js', 'ts', 'jsx', 'tsx', 'vue', 'svelte', 'py', 'rb', 'go', 'rs', 'java', 'kt', 'swift', 'c', 'cpp', 'h', 'hpp', 'cs', 'php', 'sh', 'bash', 'zsh', 'fish', 'ps1', 'bat', 'cmd', 'sql', 'graphql', 'gql', 'proto', 'dockerfile', 'gitignore', 'env', 'lock', 'diff', 'patch', 'r', 'lua', 'm', 'pl', 'ex', 'exs'].includes(ext) ? 40 : 0;
      },
      load: async () => (await import('./builtins/text-viewer.component')).TextViewerComponent,
    });
  }

  register(c: ViewerContribution): void {
    this.registry.register(c);
  }

  unregister(id: string): void {
    this.registry.unregister(id);
  }

  async resolve(req: ViewerRequest): Promise<{ id: string; component: Type<unknown> }> {
    return this.registry.resolve(req, req.viewerId);
  }

  resolveId(req: ViewerRequest): string {
    return this.registry.pick(req, req.viewerId).id;
  }

  list(): readonly ViewerContribution[] {
    return this.registry.list();
  }
}