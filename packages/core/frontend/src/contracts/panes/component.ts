import type { PaneCommands, PaneViewerCommands } from './commands.js';
import type { PaneNavigation, PaneTabs } from './navigation.js';
import type { PanePresentationPort, PaneViewportPort } from './presentation.js';
import type { PaneSelection } from './selection.js';
import type { PaneState } from './state.js';

export interface PaneComponent
  extends PaneState,
    PaneNavigation,
    PaneSelection,
    PanePresentationPort,
    PaneViewportPort,
    PaneTabs,
    PaneViewerCommands,
    PaneCommands {}
