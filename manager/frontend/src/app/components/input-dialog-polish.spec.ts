import { readFileSync } from 'node:fs';

const inputCenter = readFileSync(new URL('./input-center.component.ts', import.meta.url), 'utf8');
const keyboardHelpStyles = readFileSync(new URL('./keyboard-help.component.scss', import.meta.url), 'utf8');

describe('main-menu input dialog polish', () => {
  it('keeps Input & Shortcuts mounted for the shared exit animation', () => {
    expect(inputCenter).toContain('*ngIf="renderVisible"');
    expect(inputCenter).toContain('[class.dialog-group--closing]="isClosing"');
    expect(inputCenter).toContain('(animationend)="onBackdropAnimationEnd($event)"');
    expect(inputCenter).toContain('onBackdropAnimationEnd(event: AnimationEvent): void');
  });

  it('uses the standard dialog close button for Input & Shortcuts', () => {
    expect(inputCenter).toContain('class="dialog-close"');
    expect(inputCenter).toContain('title="Close (Esc)"');
  });

  it('does not blur the application behind Keyboard Shortcuts', () => {
    expect(keyboardHelpStyles).not.toContain('backdrop-filter:');
  });
});