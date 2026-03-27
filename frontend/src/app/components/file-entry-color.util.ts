import { FileType, type FileInfo } from '../shared/protocol';

export type FileMimeColorKind =
  | 'default'
  | 'directory'
  | 'archive'
  | 'symlink'
  | 'text'
  | 'code'
  | 'image'
  | 'video'
  | 'audio'
  | 'document'
  | 'data'
  | 'executable';

const CODE_MIME_SET = new Set([
  'application/javascript',
  'application/ecmascript',
  'application/json',
  'application/ld+json',
  'application/manifest+json',
  'application/typescript',
  'application/vnd.api+json',
  'application/x-httpd-php',
  'application/x-javascript',
  'application/x-sh',
  'application/x-shellscript',
  'application/xml',
  'application/x-yaml',
  'application/yaml',
  'text/css',
  'text/html',
  'text/javascript',
  'text/jsx',
  'text/markdown',
  'text/mdx',
  'text/typescript',
  'text/x-c',
  'text/x-c++',
  'text/x-csharp',
  'text/x-go',
  'text/x-java-source',
  'text/x-kotlin',
  'text/x-lua',
  'text/x-python',
  'text/x-ruby',
  'text/x-rust',
  'text/x-sass',
  'text/x-scss',
  'text/x-shellscript',
  'text/x-sql',
  'text/x-swift',
  'text/xml',
]);

const DOCUMENT_MIME_SET = new Set([
  'application/msword',
  'application/pdf',
  'application/postscript',
  'application/rtf',
  'application/vnd.ms-excel',
  'application/vnd.ms-powerpoint',
  'application/vnd.oasis.opendocument.presentation',
  'application/vnd.oasis.opendocument.spreadsheet',
  'application/vnd.oasis.opendocument.text',
  'application/vnd.openxmlformats-officedocument.presentationml.presentation',
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  'text/csv',
  'text/plain',
  'text/rtf',
]);

const DATA_MIME_SET = new Set([
  'application/sql',
  'application/x-sql',
  'application/x-sqlite3',
  'application/xml',
  'application/yaml',
  'application/x-yaml',
  'text/csv',
  'text/tab-separated-values',
]);

const EXECUTABLE_MIME_SET = new Set([
  'application/java-archive',
  'application/vnd.microsoft.portable-executable',
  'application/x-appimage',
  'application/x-deb',
  'application/x-dosexec',
  'application/x-executable',
  'application/x-mach-binary',
  'application/x-msdownload',
  'application/x-msi',
  'application/x-rpm',
]);

const TEXT_EXT_SET = new Set([
  '.txt', '.text', '.log', '.cfg', '.conf', '.ini', '.nfo', '.asc', '.rst'
]);

const CODE_EXT_SET = new Set([
  '.c', '.cc', '.cpp', '.cxx', '.cs', '.css', '.go', '.h', '.hpp', '.htm', '.html', '.java', '.js', '.json',
  '.jsx', '.kt', '.kts', '.less', '.lua', '.mjs', '.php', '.pl', '.ps1', '.py', '.rb', '.rs', '.sass', '.scss',
  '.sh', '.sql', '.svelte', '.swift', '.toml', '.ts', '.tsx', '.vue', '.xml', '.yaml', '.yml'
]);

const IMAGE_EXT_SET = new Set([
  '.avif', '.bmp', '.gif', '.heic', '.ico', '.jpeg', '.jpg', '.png', '.svg', '.tif', '.tiff', '.webp'
]);

const VIDEO_EXT_SET = new Set([
  '.3gp', '.avi', '.flv', '.m4v', '.mkv', '.mov', '.mp4', '.mpeg', '.mpg', '.mts', '.m2ts', '.webm', '.wmv'
]);

const AUDIO_EXT_SET = new Set([
  '.aac', '.aiff', '.alac', '.flac', '.m4a', '.mid', '.midi', '.mp3', '.ogg', '.opus', '.wav', '.wma'
]);

const DOCUMENT_EXT_SET = new Set([
  '.csv', '.doc', '.docx', '.epub', '.odp', '.ods', '.odt', '.pdf', '.ppt', '.pptx', '.rtf', '.xls', '.xlsx'
]);

const DATA_EXT_SET = new Set([
  '.db', '.db3', '.jsonl', '.parquet', '.sqlite', '.sqlite3', '.tsv'
]);

const EXECUTABLE_EXT_SET = new Set([
  '.apk', '.appimage', '.bat', '.cmd', '.com', '.dll', '.exe', '.jar', '.msi', '.scr'
]);

function normalizeMime(mime: string | null | undefined): string {
  return (mime ?? '').split(';', 1)[0].trim().toLowerCase();
}

function getNormalizedExtension(name: string | null | undefined): string {
  const value = (name ?? '').trim().toLowerCase();
  const lastDot = value.lastIndexOf('.');
  return lastDot >= 0 ? value.slice(lastDot) : '';
}

function getKindFromExtension(name: string | null | undefined): FileMimeColorKind | null {
  const ext = getNormalizedExtension(name);
  if (!ext) return null;
  if (IMAGE_EXT_SET.has(ext)) return 'image';
  if (VIDEO_EXT_SET.has(ext)) return 'video';
  if (AUDIO_EXT_SET.has(ext)) return 'audio';
  if (EXECUTABLE_EXT_SET.has(ext)) return 'executable';
  if (DOCUMENT_EXT_SET.has(ext)) return 'document';
  if (DATA_EXT_SET.has(ext)) return 'data';
  if (CODE_EXT_SET.has(ext)) return 'code';
  if (TEXT_EXT_SET.has(ext)) return 'text';
  return null;
}

export function getFileMimeColorKind(entry: FileInfo, colorizeByMime: boolean): FileMimeColorKind {
  if (!entry) return 'default';
  if (!colorizeByMime) return 'default';
  if (entry.type === FileType.SYMLINK || entry.isSymlink) return 'symlink';
  if (entry.type === FileType.DIRECTORY) return 'directory';
  if (entry.type === FileType.ARCHIVE) return 'archive';

  const extKind = getKindFromExtension(entry.name);
  if (extKind) return extKind;

  const mime = normalizeMime(entry.mimeType);
  if (!mime) return 'default';

  if (mime.startsWith('image/')) return 'image';
  if (mime.startsWith('video/')) return 'video';
  if (mime.startsWith('audio/')) return 'audio';
  if (mime.startsWith('text/')) {
    if (mime === 'text/plain' || mime === 'text/rtf' || mime === 'text/csv') return 'document';
    if (CODE_MIME_SET.has(mime)) return 'code';
    return 'text';
  }
  if (mime.endsWith('+json') || mime.endsWith('+xml')) return 'code';
  if (CODE_MIME_SET.has(mime)) return 'code';
  if (DOCUMENT_MIME_SET.has(mime)) return 'document';
  if (DATA_MIME_SET.has(mime)) return 'data';
  if (EXECUTABLE_MIME_SET.has(mime)) return 'executable';
  if (mime.startsWith('font/')) return 'data';
  return 'default';
}