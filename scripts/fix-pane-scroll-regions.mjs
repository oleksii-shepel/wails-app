import { readFileSync, writeFileSync } from 'node:fs';

const files = [
  'manager/frontend/src/app/components/registry-pane.component.ts',
  'manager/frontend/src/app/components/network-pane.component.ts',
  'manager/frontend/src/app/components/environment-pane.component.ts',
  'manager/frontend/src/app/components/autorun-pane.component.ts',
];

const overrideCss = `

    /* Keep Details/List headers outside the scrollable row viewport. */
    .list {
      display: flex !important;
      flex-direction: column;
      overflow: hidden !important;
      padding: 0 !important;
    }

    .list-viewport {
      flex: 1 1 auto;
      min-height: 0;
      overflow: auto;
      padding: 4px;
      overscroll-behavior: none;
      scroll-behavior: auto;
      touch-action: pan-y;
    }

    .list.view-grid .list-viewport {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(108px, 1fr));
      align-content: start;
      gap: 5px;
      padding: 7px;
    }

    .list.view-list .list-viewport,
    .list.view-details .list-viewport {
      display: grid;
      grid-template-columns: minmax(100%, max-content);
      align-content: start;
      padding: 0;
    }

    .list.view-list .list-viewport > .entry,
    .list.view-details .list-viewport > .entry {
      grid-column: 1;
      width: auto !important;
      min-width: 0 !important;
      justify-self: stretch;
      margin-left: 4px;
      margin-right: 4px;
    }

    .list.view-list .list-viewport > .list-virtual-spacer,
    .list.view-details .list-viewport > .list-virtual-spacer {
      grid-column: 1;
      width: auto !important;
      min-width: 0 !important;
      justify-self: stretch;
    }

    .list-viewport:focus,
    .list-viewport:focus-visible {
      outline: none !important;
      box-shadow: none !important;
    }
`;

for (const file of files) {
  let source = readFileSync(file, 'utf8');

  const outerRe = /      <div\s+        #listViewport\s+        class="list"\s+        data-pane-content\s+        data-pane-details-root\s+        \*ngIf="!activePanelPresentation && !error && viewMode !== 'tree'"\s+        tabindex="-1"\s+        role="listbox"\s+        (?<aria>\[attr\.aria-label\]="[^"]+")\s+        (?<classes>(?:\[[^\n]+\]="[^"]+"\s+)+)\s+        \(scroll\)="onContentScroll\(\$event\)"\s+        \[class\.background-drop-target\]="backgroundDropActive"\s+        \(click\)="onBackgroundClick\(\$event\)"\s+\(contextmenu\)="onBackgroundContextMenu\(\$event\)"\s+        \(dragover\)="onBackgroundDragOver\(\$event\)"\s+        \(dragleave\)="onBackgroundDragLeave\(\$event\)"\s+        \(drop\)="onBackgroundDrop\(\$event\)">/;
  const outer = outerRe.exec(source);
  if (!outer?.groups) {
    throw new Error(`Could not find outer list viewport in ${file}`);
  }

  source = source.replace(outerRe, `      <div
        class="list"
        data-pane-details-root
        *ngIf="!activePanelPresentation && !error && viewMode !== 'tree'"
        ${outer.groups.classes.trimEnd()}
        [class.background-drop-target]="backgroundDropActive">`);

  const headerStart = source.indexOf('<div class="details-header-viewport"');
  if (headerStart < 0) {
    throw new Error(`Could not find details header viewport in ${file}`);
  }

  const detailsClassIndex = source.indexOf('class="details-header"', headerStart);
  if (detailsClassIndex < 0) {
    throw new Error(`Could not find details header class in ${file}`);
  }
  source = source.slice(0, detailsClassIndex) + '#detailsHeader ' + source.slice(detailsClassIndex);

  const rowsStartRe = /\n\s*<div\s+\*ngIf="listVirtualWindow\.enabled && listVirtualWindow\.beforePx > 0"/g;
  rowsStartRe.lastIndex = headerStart;
  const rowsStartMatch = rowsStartRe.exec(source);
  if (!rowsStartMatch) {
    throw new Error(`Could not find row content start in ${file}`);
  }
  const rowsStart = rowsStartMatch.index;
  source = source.slice(0, rowsStart) + `

      <div
        #listViewport
        class="list-viewport"
        data-pane-content
        tabindex="-1"
        role="listbox"
        ${outer.groups.aria}
        (scroll)="onListViewportScroll($event)"
        (click)="onBackgroundClick($event)"
        (contextmenu)="onBackgroundContextMenu($event)"
        (dragover)="onBackgroundDragOver($event)"
        (dragleave)="onBackgroundDragLeave($event)"
        (drop)="onBackgroundDrop($event)">` + source.slice(rowsStart);

  const dropIntentRe = /\n\s*<div\s+class="drop-intent"/g;
  dropIntentRe.lastIndex = rowsStart;
  const dropIntentMatch = dropIntentRe.exec(source);
  if (!dropIntentMatch) {
    throw new Error(`Could not find drop intent boundary in ${file}`);
  }
  const dropIntent = dropIntentMatch.index;
  source = source.slice(0, dropIntent) + '\n        </div>' + source.slice(dropIntent);

  source = source.replace(
    /(@ViewChild\('listViewport'\) listViewport\?: ElementRef<HTMLElement>;\s*)/,
    `$1  @ViewChild('detailsHeader') detailsHeader?: ElementRef<HTMLElement>;\n`,
  );

  source = source.replace(
    /(\n  onContentScroll\(event: Event\): void \{)/,
    `
  onListViewportScroll(event: Event): void {
    this.onContentScroll(event);
    const header = this.detailsHeader?.nativeElement;
    const target = event.currentTarget as HTMLElement | null;
    if (header && target) {
      header.style.transform = target.scrollLeft > 0 ? \`translateX(\${-target.scrollLeft}px)\` : '';
    }
  }
$1`,
  );

  source = source.replace(/\n  `\],\r?\n}\)/, `${overrideCss}\n  \`],\n})`);

  writeFileSync(file, source);
}
