import { DEFAULT_FILE_CATEGORIES, FILE_CATEGORY_EXTENSIONS, type FileCategory } from '../shared/file-categories';
import { FileInfo, FileType } from '../shared/protocol';

export { DEFAULT_FILE_CATEGORIES, type FileCategory } from '../shared/file-categories';

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
export const GROUP_ORDER = ['Basic', 'Extensions', 'Special', 'Date', 'Size', 'Other'];

export const GROUP_MODES: Record<string, 'AND' | 'OR'> = {
  Basic: 'OR',
  Extensions: 'OR',
  Other: 'OR',
  Special: 'AND',
  Date: 'AND',
  Size: 'AND',
};

// Kept for any legacy imports.
export const BUILT_IN_CATEGORIES: FilterCategory[] = [];

/**
 * Analyze a dataset and return FilterPresets with real match counts.
 * Presets with count === 0 are omitted so only categories present in the folder are shown.
 *
 * Filter strings are compatible with FilterService.parseFilterText().
 */
export function buildPresetsFromDataset(dataset: FileInfo[]): FilterPreset[] {
  if (!dataset?.length) return [];

  const now = Date.now();
  const DAY = 86_400_000;

  let files = 0;
  let dirs = 0;
  let symlinks = 0;
  let hidden = 0;
  const dateCounts = { today: 0, thisWeek: 0, thisMonth: 0 };
  const sizeCounts = { small: 0, medium: 0, large: 0, huge: 0 };
  const extMap = new Map<string, number>();

  for (const f of dataset) {
    if (f.type === FileType.FILE) {
      files++;
    } else if (f.type === FileType.DIRECTORY) {
      dirs++;
    }

    if (f.type === FileType.SYMLINK || (f as any).isSymlink) symlinks++;
    if (f.name.startsWith('.')) hidden++;

    const age = now - f.modified * 1000;
    if (age < DAY) dateCounts.today++;
    if (age < 7 * DAY) dateCounts.thisWeek++;
    if (age < 30 * DAY) dateCounts.thisMonth++;

    if (f.type !== FileType.FILE) continue;

    const sz = f.size ?? 0;
    if (sz < 102_400) {
      sizeCounts.small++;
    } else if (sz < 10_485_760) {
      sizeCounts.medium++;
    } else if (sz < 1_073_741_824) {
      sizeCounts.large++;
    } else {
      sizeCounts.huge++;
    }

    const dot = f.name.lastIndexOf('.');
    if (dot > 0) {
      const ext = f.name.slice(dot + 1).toLowerCase();
      extMap.set(ext, (extMap.get(ext) ?? 0) + 1);
    }
  }

  const catCounts = new Map<string, number>();
  for (const cat of DEFAULT_FILE_CATEGORIES) {
    let count = 0;
    for (const ext of cat.extensions) count += extMap.get(ext) ?? 0;
    if (count > 0) catCounts.set(cat.id, count);
  }

  const out: FilterPreset[] = [];

  if (files) {
    out.push({ id: 'files', name: 'Files', filter: 'type:file', icon: '📄', group: 'Basic', count: files });
  }
  if (dirs) {
    out.push({ id: 'dirs', name: 'Folders', filter: 'type:dir', icon: '📁', group: 'Basic', count: dirs });
  }
  if (symlinks) {
    out.push({ id: 'symlinks', name: 'Symlinks', filter: 'type:symlink', icon: '🔗', group: 'Basic', count: symlinks });
  }

  for (const cat of DEFAULT_FILE_CATEGORIES) {
    const count = catCounts.get(cat.id);
    if (!count) continue;

    out.push({
      id: `ext-${cat.id}`,
      name: cat.name,
      filter: cat.extensions.map((ext) => `ext:${ext}`).join(' '),
      icon: cat.icon,
      group: 'Extensions',
      isDynamic: true,
      count,
    });
  }

  Array.from(extMap.entries())
    .filter(([ext, count]) => count >= 2 && !FILE_CATEGORY_EXTENSIONS.has(ext))
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
    .forEach(([ext, count]) => {
      out.push({
        id: `ext-raw-${ext}`,
        name: `.${ext}`,
        filter: `ext:${ext}`,
        icon: '📎',
        group: 'Extensions',
        isDynamic: true,
        count,
      });
    });

  if (hidden) {
    out.push({ id: 'hidden', name: 'Hidden', filter: 'hidden:true', icon: '👻', group: 'Special', count: hidden });
  }

  if (dateCounts.today) {
    out.push({ id: 'today', name: 'Today', filter: 'modified:today', icon: '⚡', group: 'Date', isDynamic: true, count: dateCounts.today });
  }
  if (dateCounts.thisWeek) {
    out.push({ id: 'this-week', name: 'This week', filter: 'modified:week', icon: '🗓', group: 'Date', isDynamic: true, count: dateCounts.thisWeek });
  }
  if (dateCounts.thisMonth) {
    out.push({ id: 'this-month', name: 'This month', filter: 'modified:month', icon: '🗓️', group: 'Date', isDynamic: true, count: dateCounts.thisMonth });
  }

  if (sizeCounts.small) {
    out.push({ id: 'size-small', name: '< 100 KB', filter: 'size<102400', icon: '🔹', group: 'Size', count: sizeCounts.small });
  }
  if (sizeCounts.medium) {
    out.push({ id: 'size-medium', name: '100 KB – 10 MB', filter: 'size>102400 size<10485760', icon: '🔸', group: 'Size', count: sizeCounts.medium });
  }
  if (sizeCounts.large) {
    out.push({ id: 'size-large', name: '> 10 MB', filter: 'size>10485760', icon: '🔶', group: 'Size', count: sizeCounts.large });
  }
  if (sizeCounts.huge) {
    out.push({ id: 'size-huge', name: '> 1 GB', filter: 'size>1073741824', icon: '🔴', group: 'Size', count: sizeCounts.huge });
  }

  return out;
}
