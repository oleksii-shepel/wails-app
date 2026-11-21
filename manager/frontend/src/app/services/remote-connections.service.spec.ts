import '@angular/compiler';
import type { contracts } from '../../../wailsjs/go/models';
import type { ApiService } from './api.service';
import { RemoteConnectionsService } from './remote-connections.service';

const mount = (name: string, publicPath: string): contracts.RemoteMount =>
  ({ name, mountPoint: name, publicPath, protocol: 's3' } as contracts.RemoteMount);

describe('RemoteConnectionsService', () => {
  it('resolves the deepest mounted root and preserves case-sensitive cloud paths', async () => {
    const parent = mount('Archive', 's3://bucket');
    const nested = mount('Reports', 's3://bucket/Reports');
    const service = new RemoteConnectionsService({
      remoteListConnections: async () => [parent, nested],
    } as unknown as ApiService);
    await service.refresh();

    expect(service.findByUri('S3://bucket/Reports/2026')).toBe(nested);
    expect(service.findByUri('s3://bucket/reports/2026')).toBe(parent);
  });

  it('keeps a successfully disconnected mount out of the list when refresh fails', async () => {
    const removed = mount('Archive', 's3://archive');
    const remaining = mount('Reports', 's3://reports');
    const list = jasmine.createSpy('remoteListConnections').and.resolveTo([removed, remaining]);
    const service = new RemoteConnectionsService({
      remoteListConnections: list,
      remoteDisconnect: async () => {},
    } as unknown as ApiService);
    await service.refresh();
    list.and.rejectWith(new Error('Offline'));

    expect(await service.disconnect(removed)).toBeTrue();
    expect(service.mounts).toEqual([remaining]);
    expect(service.state$.value.error).toBe('Offline');
  });
});
