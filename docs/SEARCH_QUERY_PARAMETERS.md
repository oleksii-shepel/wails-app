# Search URL Query Parameters - Complete Guide

## Overview
The `search://` scheme now supports comprehensive query parameters, eliminating the need for custom search dialogs. All filters are encoded in the URL itself, making searches shareable, bookmarkable, and programmable.

## Basic Usage

### Simple Pattern Search
```
search://?path=C:/Users&pattern=*.log
```

### With Multiple Filters
```
search://?path=filesystem://c&pattern=*.log&minSize=1048576&modifiedAfter=2024-01-01
```

## Available Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `path` | string | `/` | Base directory to search from |
| `pattern` | string | (required) | Search pattern (glob-style or regex) |
| `recursive` | boolean | `true` | Search subdirectories recursively |
| `regex` | boolean | `false` | Use regex pattern instead of glob syntax |
| `caseSensitive` | boolean | `false` | Case-sensitive pattern matching |
| `fileTypes` | string | `file,directory` | Comma-separated: `file`, `directory`, `symlink` |
| `extensions` | string | - | Comma-separated extensions: `.txt,.md,.log` |
| `minSize` | number | - | Minimum file size in bytes (1048576 = 1MB) |
| `maxSize` | number | - | Maximum file size in bytes |
| `modifiedAfter` | string | - | ISO 8601 date (inclusive): `2024-01-01` or `2024-01-01T12:00:00Z` |
| `modifiedBefore` | string | - | ISO 8601 date (inclusive): `2024-12-31` |
| `searchHidden` | boolean | `false` | Include hidden files in search |
| `contentSearch` | boolean | `false` | Search inside file contents (requires backend support) |
| `metadata` | string | - | Custom metadata filter (backend-specific) |

## Examples

### Find all log files larger than 1MB modified in January 2024
```
search://?path=C:/logs&pattern=*.log&minSize=1048576&modifiedAfter=2024-01-01&modifiedBefore=2024-01-31
```

### Search with regular expressions
```
search://?path=/home/user&pattern=test_.*\.js&regex=true&caseSensitive=true
```

### Find images from last 30 days
```
search://?path=My Documents&pattern=*.jpg,*.png&extensions=.jpg,.png&modifiedAfter=2024-02-23
```

### Non-recursive search in current directory only
```
search://?path=C:/work&pattern=*.txt&recursive=false
```

### Find large binary files
```
search://?path=C:&pattern=&fileTypes=file&minSize=10485760&extensions=.exe,.dll
```

### Search content within files
```
search://?path=/src&pattern=TODO&contentSearch=true
```

## Building Searches Programmatically

### Using SearchQueryBuilderService

```typescript
import { SearchQueryBuilderService } from './services/search-query-builder.service';

constructor(private searchBuilder: SearchQueryBuilderService) {}

buildComplexSearch() {
  const url = this.searchBuilder
    .create()
    .setPath('C:/Users/Documents')
    .setPattern('*.log')
    .setMinSize(1024 * 1024) // 1MB
    .setModifiedAfter('2024-01-01')
    .setExtensions('.log,.txt')
    .setCaseSensitive(false)
    .build();

  console.log(url);
  // Output: search://?path=C%3A%2FUsers%2FDocuments&pattern=*.log&minSize=1048576&modifiedAfter=2024-01-01&extensions=.log%2Ctxt&caseSensitive=false

  // Navigate to search
  this.router.navigateByUrl(url);
}
```

### Parsing Existing Search URLs
```typescript
const builder = this.searchBuilder
  .fromUrl('search://?path=C:/logs&pattern=*.log&minSize=1048576');

const description = builder.toDescription();
// Output: "Pattern: *.log | In: C:/logs | Min size: 1.0MB"

const params = builder.getParams();
// Access individual parameters
console.log(params.pattern); // "*.log"
console.log(params.minSize);  // 1048576
```

## Autocompletion Support

When typing `search://?` in the address bar:
- **Common file type filters** are suggested (.txt, .md, .log, .exe, .dll, .jpg, .png, etc.)
- **Size filters** are suggested (Larger than 1MB, Smaller than 1KB)
- **Date range suggestions** can be added
- **Saved searches** can be suggested from history

## URL Encoding Notes

Special characters are automatically encoded:
- Spaces: `%20`
- Slashes: `%2F` (but common chars like `/`, `:`, `*` are preserved)
- Commas: `%2C` (in extensions, etc.)

## Migrating from Search Dialog

Old approach:
```
// Open dialog, fill fields, execute
searchDialog.open({
  path: 'C:/logs',
  pattern: '*.log',
  minSize: 1048576,
  modifiedAfter: new Date('2024-01-01')
});
```

New approach:
```typescript
// Use builder to create URL and navigate
const url = this.searchBuilder
  .create()
  .setPath('C:/logs')
  .setPattern('*.log')
  .setMinSize(1048576)
  .setModifiedAfter('2024-01-01')
  .build();

// Navigate directly (no dialog needed)
this.router.navigateByUrl(url);

// Or create a bookmark/favorite with this URL
this.favoritesService.add({
  name: 'Recent Big Logs',
  path: url
});
```

## Benefits

✅ **No Dialog Needed** - Encode everything in the URL
✅ **Shareable** - Copy and paste search URLs with colleagues
✅ **Bookmarkable** - Save as favorites or in browser history
✅ **Programmable** - Build searches dynamically in TypeScript
✅ **Deep-linkable** - Navigation directly to search results
✅ **Context-preserving** - All search parameters are visible in address bar
✅ **Autocomplete-friendly** - Easy to discover available filters

## Future Enhancements

- Saved search templates as favorites
- Recent searches dropdown
- Search history with statistics
- Complex boolean queries (AND, OR, NOT combinations)
- Content preview in results with context highlighting