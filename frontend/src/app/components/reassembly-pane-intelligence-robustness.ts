export interface PaneHealthInput {
  readonly leftAvailable: boolean;
  readonly rightAvailable: boolean;
  readonly leftRootAvailable: boolean;
  readonly rightRootAvailable: boolean;
  readonly linkedNavigation: boolean;
}

export interface PaneHealthDecision {
  readonly keepLink: boolean;
  readonly keepFollow: boolean;
  readonly keepScroll: boolean;
  readonly message: string;
}

export function decidePaneHealth(
  input: PaneHealthInput,
): PaneHealthDecision {
  if (!input.leftAvailable && !input.rightAvailable) {
    return {
      keepLink: false,
      keepFollow: false,
      keepScroll: false,
      message: 'both pane locations unavailable',
    };
  }

  if (!input.leftAvailable) {
    return {
      keepLink: false,
      keepFollow: false,
      keepScroll: false,
      message: 'left pane location unavailable',
    };
  }

  if (!input.rightAvailable) {
    return {
      keepLink: false,
      keepFollow: false,
      keepScroll: false,
      message: 'right pane location unavailable',
    };
  }

  if (
    input.linkedNavigation &&
    (!input.leftRootAvailable || !input.rightRootAvailable)
  ) {
    return {
      keepLink: false,
      keepFollow: true,
      keepScroll: true,
      message: !input.leftRootAvailable && !input.rightRootAvailable
        ? 'both linked roots unavailable'
        : !input.leftRootAvailable
          ? 'left linked root unavailable'
          : 'right linked root unavailable',
    };
  }

  return {
    keepLink: input.linkedNavigation,
    keepFollow: true,
    keepScroll: true,
    message: 'healthy',
  };
}

export function comparisonSignature(
  leftPath: string,
  rightPath: string,
  leftCount: number,
  rightCount: number,
): string {
  return [
    leftPath,
    rightPath,
    Math.max(0, leftCount),
    Math.max(0, rightCount),
  ].join('|');
}
