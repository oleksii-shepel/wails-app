import type { ViewerFileListItem } from '../shared/protocol-enhanced';
import type { ViewerArchiveContext } from './viewer-archive-context';

export interface ViewerHostTabState {
  targetPath: string;
  fileList: ViewerFileListItem[];
  archiveContext: ViewerArchiveContext | null;
  viewerId: string | null;
  autoFollow: boolean;
  sourcePane: 'left' | 'right' | null;
  sourceTabId: string | null;
  sourceDir: string | null;
  isClosable: boolean;
  isPinned: boolean;
}