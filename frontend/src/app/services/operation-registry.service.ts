import { Injectable } from '@angular/core';
import { OperationKind } from '../shared/protocol-enhanced';

export interface OperationConfig {
  kind: OperationKind;
  title: string;
  icon: string;
  themeColor: string;
  cancellable: boolean;
  pausable: boolean;
}

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
    ['calculate', {
      kind: 'calculate',
      title: 'Calculating',
      icon: 'calculator',
      themeColor: '--vsc-info',
      cancellable: true,
      pausable: false
    }]
  ]);

  /**
   * Register a new operation type configuration
   */
  register(config: OperationConfig): void {
    this.configs.set(config.kind, config);
  }

  /**
   * Get configuration for an operation kind
   */
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

  /**
   * Get all registered operation configurations
   */
  getAll(): OperationConfig[] {
    return Array.from(this.configs.values());
  }

  /**
   * Check if an operation kind exists
   */
  has(kind: OperationKind): boolean {
    return this.configs.has(kind);
  }

  /**
   * Remove an operation configuration
   */
  unregister(kind: OperationKind): boolean {
    return this.configs.delete(kind);
  }

  /**
   * Update an existing operation configuration
   */
  update(kind: OperationKind, partialConfig: Partial<OperationConfig>): boolean {
    const existing = this.configs.get(kind);
    if (!existing) return false;
    
    this.configs.set(kind, { ...existing, ...partialConfig });
    return true;
  }
}