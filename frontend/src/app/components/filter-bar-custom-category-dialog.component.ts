// filter-bar-custom-category-dialog.component.ts
import { Component, Input, Output, EventEmitter, OnInit, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { FilterCategory, CustomCategoryData } from './filter-bar.types';

@Component({
  selector: 'app-filter-bar-custom-category-dialog',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="fb-dialog-overlay" (click)="onOverlayClick($event)">
      <div class="fb-dialog" [ngStyle]="{ left: left + 'px', top: top + 'px' }">
        <div class="fb-dialog-header">
          <h3>{{ category ? 'Edit' : 'Add' }} Custom Category</h3>
          <button class="fb-dialog-close" (click)="cancel.emit()">×</button>
        </div>
        
        <div class="fb-dialog-body">
          <div class="fb-form-group">
            <label>Name</label>
            <input
              type="text"
              [(ngModel)]="name"
              placeholder="e.g., My Documents"
              class="fb-input"
              autofocus
            />
          </div>
          
          <div class="fb-form-group">
            <label>Icon (emoji)</label>
            <input
              type="text"
              [(ngModel)]="icon"
              placeholder="📁"
              class="fb-input fb-input-small"
              maxlength="2"
            />
          </div>
          
          <div class="fb-form-group">
            <label>Filter Pattern</label>
            <input
              type="text"
              [(ngModel)]="pattern"
              placeholder="*.pdf,*.doc,*.txt"
              class="fb-input"
            />
            <div class="fb-hint">
              Comma-separated patterns (e.g., *.jpg,*.png)
            </div>
          </div>
          
          <div class="fb-form-group">
            <label>Preview</label>
            <div class="fb-preview">
              <span class="fb-preview-icon">{{ icon || '📄' }}</span>
              <span class="fb-preview-name">{{ name || 'New Category' }}</span>
              <span class="fb-preview-pattern">{{ pattern || 'No pattern' }}</span>
            </div>
          </div>
        </div>
        
        <div class="fb-dialog-footer">
          <button class="fb-btn fb-btn-secondary" (click)="cancel.emit()">Cancel</button>
          <button
            class="fb-btn fb-btn-primary"
            [disabled]="!isValid"
            (click)="onSave()"
          >{{ category ? 'Update' : 'Add' }}</button>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .fb-dialog-overlay {
      position: fixed;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background: transparent;
      z-index: 10001;
    }

    .fb-dialog {
      position: fixed;
      width: 320px;
      background: var(--vsc-panel-background);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      animation: fb-dialog-appear var(--vsc-transition-fast) cubic-bezier(0.4, 0, 0.2, 1);
    }

    @keyframes fb-dialog-appear {
      from { opacity: 0; transform: scale(0.95); }
      to { opacity: 1; transform: scale(1); }
    }

    .fb-dialog-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: var(--vsc-padding-lg);
      border-bottom: 1px solid var(--vsc-border-subtle);
      
      h3 {
        margin: 0;
        font-size: var(--vsc-font-size);
        font-weight: 600;
        color: var(--vsc-foreground-bright);
      }
    }

    .fb-dialog-close {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 24px;
      height: 24px;
      padding: 0;
      background: transparent;
      border: none;
      border-radius: 4px;
      color: var(--vsc-foreground-dim);
      font-size: 18px;
      cursor: pointer;
      
      &:hover {
        background: var(--vsc-list-hover-background);
        color: var(--vsc-foreground-bright);
      }
    }

    .fb-dialog-body {
      padding: var(--vsc-padding-lg);
    }

    .fb-form-group {
      margin-bottom: var(--vsc-padding-lg);
      
      label {
        display: block;
        margin-bottom: var(--vsc-padding-xs);
        font-size: var(--vsc-font-size-small);
        color: var(--vsc-foreground-dim);
      }
    }

    .fb-input {
      width: 100%;
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-foreground);
      font-size: var(--vsc-font-size);
      
      &:focus {
        outline: none;
        border-color: var(--vsc-focusBorder, var(--vsc-accent-blue));
      }
      
      &.fb-input-small {
        width: 60px;
      }
    }

    .fb-hint {
      margin-top: var(--vsc-padding-xs);
      font-size: 11px;
      color: var(--vsc-foreground-dim);
    }

    .fb-preview {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-sm);
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-border-subtle);
      border-radius: var(--vsc-border-radius-sm);
      
      .fb-preview-icon {
        font-size: 16px;
      }
      
      .fb-preview-name {
        font-weight: 600;
        color: var(--vsc-foreground-bright);
      }
      
      .fb-preview-pattern {
        color: var(--vsc-foreground-dim);
        font-size: var(--vsc-font-size-small);
      }
    }

    .fb-dialog-footer {
      display: flex;
      justify-content: flex-end;
      gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-lg);
      border-top: 1px solid var(--vsc-border-subtle);
    }

    .fb-btn {
      padding: var(--vsc-padding-sm) var(--vsc-padding-lg);
      border: 1px solid transparent;
      border-radius: var(--vsc-border-radius-sm);
      font-size: var(--vsc-font-size);
      cursor: pointer;
      transition: all var(--vsc-transition-fast);
      
      &:disabled {
        opacity: 0.5;
        cursor: not-allowed;
      }
    }

    .fb-btn-primary {
      background: var(--vsc-button-background);
      color: var(--vsc-button-foreground);
      
      &:hover:not(:disabled) {
        background: var(--vsc-button-hover-background);
      }
    }

    .fb-btn-secondary {
      background: transparent;
      border-color: var(--vsc-border);
      color: var(--vsc-foreground);
      
      &:hover {
        background: var(--vsc-list-hover-background);
      }
    }
  `]
})
export class FilterBarCustomCategoryDialogComponent implements OnInit {
  @Input() category: FilterCategory | null = null;
  @Input() left = 0;
  @Input() top = 0;
  
  @Output() save = new EventEmitter<CustomCategoryData>();
  @Output() cancel = new EventEmitter<void>();

  name = '';
  icon = '📁';
  pattern = '';

  ngOnInit(): void {
    if (this.category) {
      this.name = this.category.label;
      this.icon = this.category.icon || '📁';
      this.pattern = this.category.pattern || '';
    }
  }

  get isValid(): boolean {
    return this.name.trim().length > 0 && this.pattern.trim().length > 0;
  }

  onOverlayClick(event: MouseEvent): void {
    if ((event.target as HTMLElement).classList.contains('fb-dialog-overlay')) {
      this.cancel.emit();
    }
  }

  onSave(): void {
    this.save.emit({
      label: this.name.trim(),
      icon: this.icon || '📁',
      pattern: this.pattern.trim()
    });
  }
}