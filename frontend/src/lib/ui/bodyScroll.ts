let lockCount = 0;
let previousOverflow = '';

function canUseDocument(): boolean {
  return typeof document !== 'undefined' && Boolean(document.body);
}

export function lockBodyScroll(): void {
  if (!canUseDocument()) {
    return;
  }

  if (lockCount === 0) {
    previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
  }

  lockCount += 1;
}

export function unlockBodyScroll(): void {
  if (!canUseDocument()) {
    return;
  }

  if (lockCount === 0) {
    return;
  }

  lockCount -= 1;
  if (lockCount === 0) {
    document.body.style.overflow = previousOverflow;
  }
}

export function resetBodyScrollLock(): void {
  if (!canUseDocument()) {
    lockCount = 0;
    previousOverflow = '';
    return;
  }

  lockCount = 0;
  previousOverflow = '';
  document.body.style.overflow = '';
}
