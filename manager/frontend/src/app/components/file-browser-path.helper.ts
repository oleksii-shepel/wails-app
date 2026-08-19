import type { ApiService } from '../services/api.service';

export class FileBrowserPathHelper {
  constructor(private readonly apiService: ApiService) {}

  async buildDestinationPath(sourcePath: string, targetDir: string): Promise<string> {
    const fileName = await this.getFileName(sourcePath);
    return await this.apiService.joinPaths([targetDir, fileName]);
  }

  private async getFileName(path: string): Promise<string> {
    try {
      const splitResult = await this.apiService.splitPath(path);
      if (splitResult.base) {
        return splitResult.base;
      }
    } catch (error) {
      console.warn('Error splitting path via API, falling back to manual split:', error);
    }

    const normalized = path.replace(/\\/g, '/');
    const parts = normalized.split('/').filter(part => part.length > 0);
    return parts.length > 0 ? parts[parts.length - 1] : 'file';
  }
}