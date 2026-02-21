import { FileInfo, FileType } from '../shared/protocol';

// ── Core types ────────────────────────────────────────────────────────────────

export interface FilterPreset {
  id: string;
  name: string;
  /** Filter string understood by FilterService.parseFilterText() */
  filter: string;
  icon?: string;
  description?: string;
  group?: string;
  isDynamic?: boolean;
  /** Real match count from the current dataset */
  count?: number;
}

export interface FilterGroup {
  name: string;
  presets: FilterPreset[];
}

export interface FilterCategory {
  id: string;
  label: string;
  icon?: string;
  pattern?: string;
  isCustom?: boolean;
  predicate?: (entry: FileInfo) => boolean;
}

export interface CustomCategoryData {
  label: string;
  icon: string;
  pattern: string;
}

export const FILTER_PRESETS_KEY = 'filter-bar-presets';
export const GROUP_ORDER = ['Type', 'Special', 'Date', 'Size', 'Extensions', 'Other'];

// kept for any legacy imports
export const BUILT_IN_CATEGORIES: FilterCategory[] = [];

// ── File categories ───────────────────────────────────────────────────────────

export interface FileCategory {
  id: string;
  name: string;
  icon: string;
  extensions: string[];
}

export const DEFAULT_FILE_CATEGORIES: FileCategory[] = [
  { id: 'images',        name: 'Images',        icon: '🖼️',  extensions: ['jpg','jpeg','png','gif','webp','svg','bmp','ico','tiff','raw','heic'] },
  { id: 'documents',     name: 'Documents',     icon: '📄',  extensions: ['pdf','doc','docx','odt','rtf','tex','epub'] },
  { id: 'spreadsheets',  name: 'Spreadsheets',  icon: '📊',  extensions: ['xls','xlsx','ods','csv','tsv'] },
  { id: 'presentations', name: 'Presentations', icon: '📽️', extensions: ['ppt','pptx','odp','key'] },
  { id: 'code',          name: 'Code',          icon: '💻',  extensions: ['js','ts','jsx','tsx','vue','py','java','cpp','c','h','cs','go','rs','rb','php','swift','kt'] },
  { id: 'web',           name: 'Web',           icon: '🌐',  extensions: ['html','htm','css','scss','sass','less','xml','xsl'] },
  { id: 'data',          name: 'Data',          icon: '🗄️', extensions: ['json','yaml','yml','toml','ini','sql','graphql','proto'] },
  { id: 'archives',      name: 'Archives',      icon: '📦',  extensions: ['zip','tar','gz','bz2','xz','7z','rar','iso','dmg'] },
  { id: 'audio',         name: 'Audio',         icon: '🎵',  extensions: ['mp3','wav','flac','aac','ogg','m4a','wma'] },
  { id: 'video',         name: 'Video',         icon: '🎬',  extensions: ['mp4','mov','avi','mkv','wmv','flv','webm','m4v'] },
  { id: 'fonts',         name: 'Fonts',         icon: '🔤',  extensions: ['ttf','otf','woff','woff2','eot'] },
  { id: 'markdown',      name: 'Markdown',      icon: '📝',  extensions: ['md','mdx','rst','adoc'] },
  { id: 'scripts',       name: 'Scripts',       icon: '⌨️', extensions: ['sh','bash','zsh','fish','ps1','bat','cmd'] },
  { id: '3d',            name: '3D',            icon: '🎲',  extensions: ['obj','fbx','stl','blend','3ds','gltf','glb'] },
  { id: 'design',        name: 'Design',        icon: '🎨',  extensions: ['fig','sketch','xd','psd','ai','afdesign'] },
];

// ── Dynamic preset builder ────────────────────────────────────────────────────

/**
 * Analyse a dataset and return FilterPresets with real match counts.
 * Presets with count === 0 are omitted — only what's actually in the folder shows.
 *
 * Filter strings are compatible with FilterService.parseFilterText().
 */
export function buildPresetsFromDataset(dataset: FileInfo[]): FilterPreset[] {
  if (!dataset?.length) return [];

  const now  = Date.now();
  const DAY  = 86_400_000;

  // ── Count everything in one pass ─────────────────────────────────────────
  let files = 0, dirs = 0, symlinks = 0, hidden = 0;
  const dateCounts  = { today: 0, thisWeek: 0, thisMonth: 0 };
  const sizeCounts  = { small: 0, medium: 0, large: 0, huge: 0 };
  const extMap      = new Map<string, number>(); // ext (no dot) → count

  for (const f of dataset) {
    // Type
    if      (f.type === FileType.FILE)      files++;
    else if (f.type === FileType.DIRECTORY) dirs++;
    else if (f.type === FileType.SYMLINK)   symlinks++;

    // Hidden (dot-prefix)
    if (f.name.startsWith('.')) hidden++;

    // Modified date
    const age = now - f.modified * 1000;
    if (age < DAY)       dateCounts.today++;
    if (age < 7  * DAY)  dateCounts.thisWeek++;
    if (age < 30 * DAY)  dateCounts.thisMonth++;

    // Size + extension — files only
    if (f.type === FileType.FILE) {
      const sz = f.size ?? 0;
      if      (sz < 102_400)        sizeCounts.small++;
      else if (sz < 10_485_760)     sizeCounts.medium++;
      else if (sz < 1_073_741_824)  sizeCounts.large++;
      else                          sizeCounts.huge++;

      const dot = f.name.lastIndexOf('.');
      if (dot > 0) {
        const ext = f.name.slice(dot + 1).toLowerCase();
        extMap.set(ext, (extMap.get(ext) ?? 0) + 1);
      }
    }
  }

  // ── Tally per-category counts ─────────────────────────────────────────────
  const catCounts = new Map<string, number>();
  for (const cat of DEFAULT_FILE_CATEGORIES) {
    let n = 0;
    for (const ext of cat.extensions) n += extMap.get(ext) ?? 0;
    if (n > 0) catCounts.set(cat.id, n);
  }

  const out: FilterPreset[] = [];

  // Type
  if (files)    out.push({ id: 'files',    name: 'Files',    filter: 'type:file',    icon: '📄', group: 'Type',    count: files });
  if (dirs)     out.push({ id: 'dirs',     name: 'Folders',  filter: 'type:dir',     icon: '📁', group: 'Type',    count: dirs });
  if (symlinks) out.push({ id: 'symlinks', name: 'Symlinks', filter: 'type:symlink', icon: '🔗', group: 'Type',    count: symlinks });

  // Special
  if (hidden)   out.push({ id: 'hidden',   name: 'Hidden',   filter: 'hidden:true',  icon: '👻', group: 'Special', count: hidden });

  // Date — isDynamic so they get the green "📊" hint
  if (dateCounts.today)     out.push({ id: 'today',      name: 'Today',      filter: 'modified:today', icon: '⚡', group: 'Date', isDynamic: true, count: dateCounts.today });
  if (dateCounts.thisWeek)  out.push({ id: 'this-week',  name: 'This week',  filter: 'modified:week',  icon: '📅', group: 'Date', isDynamic: true, count: dateCounts.thisWeek });
  if (dateCounts.thisMonth) out.push({ id: 'this-month', name: 'This month', filter: 'modified:month', icon: '🗓️', group: 'Date', isDynamic: true, count: dateCounts.thisMonth });

  // Size
  if (sizeCounts.small)  out.push({ id: 'size-small',  name: '< 100 KB',       filter: 'size<102400',                icon: '🔹', group: 'Size', count: sizeCounts.small });
  if (sizeCounts.medium) out.push({ id: 'size-medium', name: '100 KB – 10 MB', filter: 'size>102400 size<10485760',  icon: '🔸', group: 'Size', count: sizeCounts.medium });
  if (sizeCounts.large)  out.push({ id: 'size-large',  name: '> 10 MB',        filter: 'size>10485760',              icon: '🔶', group: 'Size', count: sizeCounts.large });
  if (sizeCounts.huge)   out.push({ id: 'size-huge',   name: '> 1 GB',         filter: 'size>1073741824',            icon: '🔴', group: 'Size', count: sizeCounts.huge });

  // Named extension categories — only those present in dataset
  for (const cat of DEFAULT_FILE_CATEGORIES) {
    const count = catCounts.get(cat.id);
    if (!count) continue;
    out.push({
      id:        `ext-${cat.id}`,
      name:       cat.name,
      // Multiple ext: tokens → FilterService ORs them within the same type bucket
      filter:     cat.extensions.map(e => `ext:${e}`).join(' '),
      icon:       cat.icon,
      group:     'Extensions',
      isDynamic:  true,
      count,
    });
  }

  // Top-5 uncategorised extensions (≥2 occurrences)
  const covered = new Set(DEFAULT_FILE_CATEGORIES.flatMap(c => c.extensions));
  Array.from(extMap.entries())
    .filter(([ext, n]) => n >= 2 && !covered.has(ext))
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
    .forEach(([ext, count]) => out.push({
      id:        `ext-raw-${ext}`,
      name:      `.${ext}`,
      filter:    `ext:${ext}`,
      icon:      '📎',
      group:     'Extensions',
      isDynamic:  true,
      count,
    }));

  return out;
}
