// filter-bar.types.ts
export interface FilterCategory {
  id: string;
  label: string;
  icon?: string;
  predicate: (entry: any) => boolean; // Filter function
  isCustom?: boolean;
  pattern?: string; // Add pattern for custom categories
}

export interface FilterPreset {
  id: string;
  name: string;
  categories: string[]; // Category IDs
  isBuiltIn?: boolean;
}

export interface CustomCategoryData {
  label: string;
  icon: string;
  pattern: string;
}

export const BUILT_IN_CATEGORIES: FilterCategory[] = [
  {
    id: 'all',
    label: 'All Items',
    icon: '📄',
    predicate: () => true
  },
  {
    id: 'folders',
    label: 'Folders Only',
    icon: '📁',
    predicate: (entry) => entry.type === 'directory' || entry.type === 'DIRECTORY'
  },
  {
    id: 'files',
    label: 'Files Only',
    icon: '📄',
    predicate: (entry) => entry.type === 'file' || entry.type === 'FILE'
  },
  {
    id: 'archives',
    label: 'Archives',
    icon: '🗜️',
    predicate: (entry) => /\.(zip|tar|tgz|gz|bz2|xz|rar|7z)$/i.test(entry.name)
  },
  {
    id: 'images',
    label: 'Images',
    icon: '🖼️',
    predicate: (entry) => /\.(jpg|jpeg|png|gif|bmp|svg|webp)$/i.test(entry.name)
  },
  {
    id: 'documents',
    label: 'Documents',
    icon: '📝',
    predicate: (entry) => /\.(pdf|doc|docx|txt|md|rtf|odt)$/i.test(entry.name)
  },
  {
    id: 'code',
    label: 'Code Files',
    icon: '💻',
    predicate: (entry) => /\.(js|ts|py|java|cpp|c|h|go|rs|php|html|css)$/i.test(entry.name)
  }
];

export const FILTER_PRESETS_KEY = 'filter-bar-presets';