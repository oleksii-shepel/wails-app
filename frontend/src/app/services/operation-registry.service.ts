import { Injectable } from '@angular/core';
import { OperationConfig, OperationKind, ThemeColor } from '../services/operation.types';

@Injectable({ providedIn: 'root' })
export class OperationRegistryService {
  private configs = new Map<OperationKind, OperationConfig>([
    ['copy', {
      kind: 'copy',
      title: 'Copying',
      icon: 'copy',
      themeColor: '--vsc-accent-blue',
      cancellable: true,
      pausable: false
    }],
    ['move', {
      kind: 'move',
      title: 'Moving',
      icon: 'move',
      themeColor: '--vsc-accent-purple',
      cancellable: true,
      pausable: false
    }],
    ['delete', {
      kind: 'delete',
      title: 'Deleting',
      icon: 'trash',
      themeColor: '--vsc-error',
      cancellable: true,
      pausable: false
    }],
    ['archive', {
      kind: 'archive',
      title: 'Archiving',
      icon: 'archive',
      themeColor: '--vsc-accent-orange',
      cancellable: true,
      pausable: false
    }],
    ['extract', {
      kind: 'extract',
      title: 'Extracting',
      icon: 'extract',
      themeColor: '--vsc-accent-green',
      cancellable: true,
      pausable: false
    }],
    ['search', {
      kind: 'search',
      title: 'Searching',
      icon: 'search',
      themeColor: '--vsc-info',
      cancellable: true,
      pausable: false
    }],
    ['upload', {
      kind: 'upload',
      title: 'Uploading',
      icon: 'upload',
      themeColor: '--vsc-accent-teal',
      cancellable: true,
      pausable: true
    }],
    ['download', {
      kind: 'download',
      title: 'Downloading',
      icon: 'download',
      themeColor: '--vsc-accent-blue',
      cancellable: true,
      pausable: true
    }]
  ]);

  register(config: OperationConfig): void {
    this.configs.set(config.kind, config);
  }

  get(kind: OperationKind): OperationConfig {
    return this.configs.get(kind) ?? {
      kind,
      title: 'Processing',
      icon: 'gear',
      themeColor: '--vsc-foreground-dim',
      cancellable: false,
      pausable: false
    };
  }

  getAll(): OperationConfig[] {
    return Array.from(this.configs.values());
  }
}