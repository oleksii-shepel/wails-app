import { FileType, type FileInfo } from '@reassembly/core/contracts/files';
/**
 * Compatibility names retained for Registry source code and tests.
 * Their identity is owned by @reassembly/core.
 */
export { FileType as RegistryFileType };
export type RegistryFileInfo = FileInfo;
export interface RegistrySourceCapabilities {
    read: boolean;
    create: boolean;
    delete: boolean;
    rename: boolean;
    trash: boolean;
}
