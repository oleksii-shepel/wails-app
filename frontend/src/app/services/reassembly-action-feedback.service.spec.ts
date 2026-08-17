import { ReassemblyActionFeedbackService } from './reassembly-action-feedback.service';

describe('ReassemblyActionFeedbackService', () => {
  it('keeps errors persistent until dismissed', () => {
    const service = new ReassemblyActionFeedbackService();
    const id = service.error('Delete failed', 'Access denied');

    expect(service.items$.value).toEqual([
      jasmine.objectContaining({
        id,
        kind: 'error',
        persistent: true,
        title: 'Delete failed',
      }),
    ]);

    service.dismiss(id);
    expect(service.items$.value).toEqual([]);
  });

  it('caps visible feedback so fast operations cannot flood the UI', () => {
    const service = new ReassemblyActionFeedbackService();

    for (let i = 0; i < 10; i += 1) {
      service.show('info', `Operation ${i}`, undefined, { persistent: true });
    }

    expect(service.items$.value.length).toBe(6);
    expect(service.items$.value[0].title).toBe('Operation 4');
  });
});
