let lockCount = 0;

function setBodyLocked(locked: boolean) {
  if (typeof document === 'undefined') {
    return;
  }
  document.body.style.overflow = locked ? 'hidden' : '';
}

export function lockBodyScroll() {
  lockCount += 1;
  if (lockCount === 1) {
    setBodyLocked(true);
  }
}

export function unlockBodyScroll() {
  if (lockCount === 0) {
    return;
  }
  lockCount -= 1;
  if (lockCount === 0) {
    setBodyLocked(false);
  }
}

export function resetBodyScrollLockForTests() {
  lockCount = 0;
  setBodyLocked(false);
}
