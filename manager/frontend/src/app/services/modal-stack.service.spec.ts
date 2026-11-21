import { ModalStackService } from './modal-stack.service';

describe('ModalStackService', () => {
  it('dismisses only the topmost modal', () => {
    const service = new ModalStackService();
    const closed: string[] = [];

    const first = document.createElement('section');
    const second = document.createElement('section');
    document.body.append(first, second);

    service.register({
      id: 'first',
      element: first,
      dismiss: () => closed.push('first'),
      dismissible: () => true,
    });
    service.register({
      id: 'second',
      element: second,
      dismiss: () => closed.push('second'),
      dismissible: () => true,
    });

    expect(service.dismissTopmost()).toBeTrue();
    expect(closed).toEqual(['second']);

    first.remove();
    second.remove();
  });

  it('does not fall through a non-dismissible topmost modal', () => {
    const service = new ModalStackService();
    const closed: string[] = [];

    const first = document.createElement('section');
    const second = document.createElement('section');
    document.body.append(first, second);

    service.register({
      id: 'first',
      element: first,
      dismiss: () => closed.push('first'),
      dismissible: () => true,
    });
    service.register({
      id: 'second',
      element: second,
      dismiss: () => closed.push('second'),
      dismissible: () => false,
    });

    expect(service.dismissTopmost()).toBeFalse();
    expect(closed).toEqual([]);

    first.remove();
    second.remove();
  });

  it('prunes destroyed modal elements', () => {
    const service = new ModalStackService();
    const closed: string[] = [];

    const first = document.createElement('section');
    const second = document.createElement('section');
    document.body.append(first, second);

    service.register({
      id: 'first',
      element: first,
      dismiss: () => closed.push('first'),
      dismissible: () => true,
    });
    service.register({
      id: 'second',
      element: second,
      dismiss: () => closed.push('second'),
      dismissible: () => true,
    });

    second.remove();

    expect(service.dismissTopmost()).toBeTrue();
    expect(closed).toEqual(['first']);

    first.remove();
  });

  it('tracks topmost, isTopmost, contextId, and bringToFront', () => {
    const service = new ModalStackService();
    const closed: string[] = [];

    const first = document.createElement('section');
    const second = document.createElement('section');
    document.body.append(first, second);

    service.register({
      id: 'first',
      element: first,
      contextId: 'overlay.First',
      dismiss: () => closed.push('first'),
      dismissible: () => true,
    });
    service.register({
      id: 'second',
      element: second,
      dismiss: () => closed.push('second'),
      dismissible: () => true,
    });

    expect(service.topmost()?.id).toBe('second');
    expect(service.isTopmost('second')).toBeTrue();
    expect(service.isTopmost('first')).toBeFalse();

    service.bringToFront('first');

    expect(service.topmost()?.id).toBe('first');
    expect(service.topmost()?.contextId).toBe('overlay.First');
    expect(service.isTopmost('first')).toBeTrue();

    expect(service.dismissTopmost()).toBeTrue();
    expect(closed).toEqual(['first']);

    first.remove();
    second.remove();
  });
});