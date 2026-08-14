/**
 * Renderer-agnostic command presentation.
 *
 * Command metadata is intentionally separate from execution so the same
 * presentation vocabulary can be used by toolbars, menus, palettes and future
 * surfaces such as Console without coupling those renderers to one another.
 */
export type CommandPresentationSource = 'builtin' | 'plugin' | 'registry';

export interface CommandPresentation<TId extends string = string> {
  readonly id: TId;
  readonly label: string;
  readonly icon?: string;
  readonly iconText?: string;
  readonly shortcut?: string;
  readonly enabled: boolean;
  readonly separatorBefore?: boolean;
  readonly source: CommandPresentationSource;
  /** Stable application command ID when this presentation maps to one. */
  readonly commandId?: string;
}

export interface PresentedCommand<TId extends string = string> extends CommandPresentation<TId> {
  execute(): void | Promise<void>;
}

export function bindCommandPresentation<TId extends string>(
  presentation: CommandPresentation<TId>,
  execute: () => void | Promise<void>,
): PresentedCommand<TId> {
  return { ...presentation, execute };
}
