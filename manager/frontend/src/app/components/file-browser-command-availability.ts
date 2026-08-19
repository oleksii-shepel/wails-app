import type { ActionAvailability } from '../services/action-availability';
import { BC } from './file-browser-command-registry';

/**
 * Maps browser command IDs onto the capability-derived action state.
 * Unknown commands remain available; their own context/handler is responsible
 * for any additional applicability rules.
 */
export function isFileBrowserCommandAvailable(
  commandId: string,
  availability: ActionAvailability,
): boolean {
  switch (commandId) {
    case BC.copy: return availability.copySelected;
    case BC.cut: return availability.cutSelected;
    case BC.paste: return availability.paste;
    case BC.deleteSelected: return availability.deleteSelected;
    case BC.newFolder: return availability.createFolder;
    case BC.rename:
    case BC.batchRename: return availability.rename;
    case BC.copyToOtherPane: return availability.copyToOther;
    case BC.moveToOtherPane: return availability.moveToOther;
    default: return true;
  }
}