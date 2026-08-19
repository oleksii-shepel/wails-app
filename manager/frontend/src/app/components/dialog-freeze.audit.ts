export interface DialogFreezeAuditTarget {
  readonly path: string;
  readonly source: string;
}

export interface DialogFreezeAuditResult {
  readonly freezingDialogs: readonly string[];
}

export function auditDialogFreeze(
  targets: readonly DialogFreezeAuditTarget[],
): DialogFreezeAuditResult {
  const freezingDialogs = targets
    .filter(target =>
      /role=["'](?:dialog|alertdialog)["']/.test(target.source) &&
      (
        target.source.includes('NativeOverlayFreezeService') ||
        /(?:overlayFreeze|freeze)\.(?:acquire|release)\(/.test(target.source)
      ),
    )
    .map(target => target.path);

  return { freezingDialogs };
}
