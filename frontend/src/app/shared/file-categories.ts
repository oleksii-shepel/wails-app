export interface FileCategory {
  id: string;
  name: string;
  icon: string;
  extensions: string[];
}

export const DEFAULT_FILE_CATEGORIES: FileCategory[] = [
  { id: 'images',        name: 'Images',        icon: '🖼️',  extensions: ['jpg', 'jpeg', 'jpe', 'png', 'apng', 'gif', 'webp', 'svg', 'svgz', 'bmp', 'dib', 'ico', 'cur', 'tif', 'tiff', 'avif', 'heic', 'heif', 'jfif', 'jxl', 'jp2', 'j2k', 'jpf', 'jpm', 'jpx', 'xcf', 'kra', 'raw', 'cr2', 'cr3', 'nef', 'arw', 'dng', 'orf', 'rw2'] },
  { id: 'documents',     name: 'Documents',     icon: '📄',  extensions: ['pdf', 'djvu', 'djv', 'doc', 'docm', 'docx', 'dot', 'dotm', 'dotx', 'odt', 'ott', 'pages', 'rtf', 'tex', 'latex', 'epub', 'mobi', 'azw', 'azw3', 'fb2', 'lit', 'wps'] },
  { id: 'spreadsheets',  name: 'Spreadsheets',  icon: '📊',  extensions: ['xls', 'xla', 'xlam', 'xlsb', 'xlsm', 'xlsx', 'xlt', 'xltm', 'xltx', 'ods', 'ots', 'numbers', 'csv', 'tsv', 'tab'] },
  { id: 'presentations', name: 'Presentations', icon: '📽️', extensions: ['ppt', 'pot', 'potm', 'potx', 'pps', 'ppsm', 'ppsx', 'pptm', 'pptx', 'odp', 'otp', 'key'] },
  { id: 'code',          name: 'Code',          icon: '💻',  extensions: ['c', 'cc', 'cpp', 'cxx', 'c++', 'h', 'hh', 'hpp', 'hxx', 'cs', 'fs', 'fsi', 'fsx', 'vb', 'go', 'rs', 'zig', 'nim', 'nims', 'py', 'pyi', 'pyx', 'pyw', 'rb', 'erb', 'php', 'phar', 'java', 'kt', 'kts', 'scala', 'sc', 'groovy', 'gvy', 'gradle', 'swift', 'm', 'mm', 'dart', 'lua', 'pl', 'pm', 'r', 'jl', 'cr', 'ex', 'exs', 'erl', 'hrl', 'clj', 'cljs', 'cljc', 'edn', 'hs', 'lhs', 'ml', 'mli', 'asm', 's', 'tsx', 'ts', 'cts', 'mts', 'jsx', 'js', 'cjs', 'mjs', 'vue', 'svelte'] },
  { id: 'web',           name: 'Web',           icon: '🌐',  extensions: ['html', 'htm', 'xhtml', 'shtml', 'css', 'scss', 'sass', 'less', 'pcss', 'postcss', 'styl', 'stylus', 'xsl', 'xslt', 'xsd', 'wsdl', 'astro'] },
  { id: 'data',          name: 'Data',          icon: '🗄️', extensions: ['json', 'jsonc', 'json5', 'ndjson', 'geojson', 'yaml', 'yml', 'toml', 'ini', 'cfg', 'conf', 'config', 'env', 'properties', 'plist', 'sql', 'sqlite', 'sqlite3', 'db', 'db3', 'parquet', 'arrow', 'avro', 'orc', 'feather', 'hdf', 'hdf5', 'graphql', 'gql', 'proto', 'xml'] },
  { id: 'archives',      name: 'Archives',      icon: '📦',  extensions: ['zip', 'zipx', 'tar', 'gz', 'tgz', 'bz2', 'tbz', 'tbz2', 'xz', 'txz', 'lz', 'lz4', 'lzma', 'zst', 'tzst', '7z', 'rar', 'cab', 'arj', 'cpio', 'deb', 'rpm', 'pkg', 'msix', 'appx', 'apk', 'jar', 'war', 'ear', 'whl', 'gem', 'iso', 'img', 'dmg', 'vhd', 'vhdx'] },
  { id: 'audio',         name: 'Audio',         icon: '🎵',  extensions: ['mp3', 'wav', 'wave', 'flac', 'aac', 'm4a', 'm4b', 'm4p', 'ogg', 'oga', 'opus', 'wma', 'aiff', 'aif', 'aifc', 'alac', 'amr', 'mid', 'midi', 'xm', 'it', 'mod', 's3m'] },
  { id: 'video',         name: 'Video',         icon: '🎬',  extensions: ['mp4', 'm4v', 'mov', 'qt', 'avi', 'divx', 'xvid', 'mkv', 'wmv', 'asf', 'flv', 'f4v', 'webm', 'ogv', 'ts', 'mts', 'm2ts', 'mpg', 'mpeg', 'mpe', 'mpv', 'mp2', 'm2p', 'm2v', 'vob', 'evo', 'mjpeg', 'mjpg', '3gp', '3g2', 'rm', 'rmvb'] },
  { id: 'fonts',         name: 'Fonts',         icon: '🔤',  extensions: ['ttf', 'ttc', 'otf', 'otc', 'woff', 'woff2', 'eot', 'pfb', 'pfm', 'fon', 'bdf', 'psf'] },
  { id: 'markdown',      name: 'Markdown',      icon: '📝',  extensions: ['md', 'mdown', 'markdown', 'mdx', 'mkd', 'mkdn', 'rst', 'adoc', 'asciidoc'] },
  { id: 'scripts',       name: 'Scripts',       icon: '⌨️', extensions: ['sh', 'bash', 'zsh', 'fish', 'ksh', 'csh', 'tcsh', 'ps1', 'psm1', 'psd1', 'bat', 'cmd', 'vbs', 'vbe', 'wsf', 'wsh'] },
  { id: '3d',            name: '3D',            icon: '🎲',  extensions: ['obj', 'mtl', 'fbx', 'stl', 'blend', '3ds', 'dae', 'gltf', 'glb', 'ply', 'usd', 'usda', 'usdc', 'usdz', 'abc', 'step', 'stp', 'iges', 'igs'] },
  { id: 'design',        name: 'Design',        icon: '🎨',  extensions: ['fig', 'sketch', 'xd', 'psd', 'ai', 'eps', 'indd', 'indt', 'afdesign', 'afphoto', 'afpub'] },
];

function normalizeExtension(extension: string | null | undefined): string {
  return (extension ?? '').trim().toLowerCase().replace(/^\./, '');
}

export const FILE_CATEGORY_BY_ID = new Map<string, FileCategory>(
  DEFAULT_FILE_CATEGORIES.map((category) => [category.id, category] as const),
);

export const FILE_CATEGORIES_BY_EXTENSION = new Map<string, FileCategory[]>();
export const FILE_CATEGORY_EXTENSIONS = new Set<string>();

for (const category of DEFAULT_FILE_CATEGORIES) {
  for (const extension of category.extensions) {
    const normalized = normalizeExtension(extension);
    if (!normalized) continue;

    FILE_CATEGORY_EXTENSIONS.add(normalized);

    const matches = FILE_CATEGORIES_BY_EXTENSION.get(normalized);
    if (matches) {
      matches.push(category);
    } else {
      FILE_CATEGORIES_BY_EXTENSION.set(normalized, [category]);
    }
  }
}

export function getFileCategoriesForExtension(extension: string | null | undefined): FileCategory[] {
  return FILE_CATEGORIES_BY_EXTENSION.get(normalizeExtension(extension)) ?? [];
}

export function getPrimaryFileCategoryForExtension(extension: string | null | undefined): FileCategory | null {
  return getFileCategoriesForExtension(extension)[0] ?? null;
}

export function hasCollidingFileCategories(extension: string | null | undefined): boolean {
  return getFileCategoriesForExtension(extension).length > 1;
}
