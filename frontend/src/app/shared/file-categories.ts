export interface FileCategory {
  id: string;
  name: string;
  icon: string;
  extensions: string[];
}

export const DEFAULT_FILE_CATEGORIES: FileCategory[] = [
  { id: 'images',        name: 'Images',        icon: '🖼️',  extensions: ['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp', 'ico', 'tiff', 'raw', 'heic'] },
  { id: 'documents',     name: 'Documents',     icon: '📄',  extensions: ['pdf', 'doc', 'docx', 'odt', 'rtf', 'tex', 'epub'] },
  { id: 'spreadsheets',  name: 'Spreadsheets',  icon: '📊',  extensions: ['xls', 'xlsx', 'ods', 'csv', 'tsv'] },
  { id: 'presentations', name: 'Presentations', icon: '📽️', extensions: ['ppt', 'pptx', 'odp', 'key'] },
  { id: 'code',          name: 'Code',          icon: '💻',  extensions: ['js', 'ts', 'jsx', 'tsx', 'vue', 'py', 'java', 'cpp', 'c', 'h', 'cs', 'go', 'rs', 'rb', 'php', 'swift', 'kt'] },
  { id: 'web',           name: 'Web',           icon: '🌐',  extensions: ['html', 'htm', 'css', 'scss', 'sass', 'less', 'xml', 'xsl'] },
  { id: 'data',          name: 'Data',          icon: '🗄️', extensions: ['json', 'yaml', 'yml', 'toml', 'ini', 'sql', 'graphql', 'proto'] },
  { id: 'archives',      name: 'Archives',      icon: '📦',  extensions: ['zip', 'tar', 'gz', 'bz2', 'xz', '7z', 'rar', 'iso', 'dmg'] },
  { id: 'audio',         name: 'Audio',         icon: '🎵',  extensions: ['mp3', 'wav', 'flac', 'aac', 'ogg', 'm4a', 'wma'] },
  { id: 'video',         name: 'Video',         icon: '🎬',  extensions: ['mp4', 'mov', 'qt', 'avi', 'divx', 'xvid', 'mkv', 'wmv', 'flv', 'webm', 'm4v', 'ts', 'mts', 'm2ts', 'mpg', 'mpeg', 'vob', 'm2p', 'm2v', 'mjpeg', 'mjpg'] },
  { id: 'fonts',         name: 'Fonts',         icon: '🔤',  extensions: ['ttf', 'otf', 'woff', 'woff2', 'eot'] },
  { id: 'markdown',      name: 'Markdown',      icon: '📝',  extensions: ['md', 'mdx', 'rst', 'adoc'] },
  { id: 'scripts',       name: 'Scripts',       icon: '⌨️', extensions: ['sh', 'bash', 'zsh', 'fish', 'ps1', 'bat', 'cmd'] },
  { id: '3d',            name: '3D',            icon: '🎲',  extensions: ['obj', 'fbx', 'stl', 'blend', '3ds', 'gltf', 'glb'] },
  { id: 'design',        name: 'Design',        icon: '🎨',  extensions: ['fig', 'sketch', 'xd', 'psd', 'ai', 'afdesign'] },
];
