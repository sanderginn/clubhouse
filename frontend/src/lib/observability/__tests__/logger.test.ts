import { describe, it, expect, vi, beforeEach } from 'vitest';
import { logError, logInfo, logWarn } from '../logger';
import { captureError, captureMessage } from '../errorTracker';

vi.mock('../errorTracker', () => ({
  captureError: vi.fn(),
  captureMessage: vi.fn(),
}));

describe('logger', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(console, 'info').mockImplementation(() => {});
    vi.spyOn(console, 'warn').mockImplementation(() => {});
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  it('logs info without sending to error tracker', () => {
    logInfo('Hello', { view: 'feed' });

    expect(captureError).not.toHaveBeenCalled();
    expect(captureMessage).not.toHaveBeenCalled();
    expect(console.info).toHaveBeenCalled();
  });

  it('logs warn and reports message to error tracker', () => {
    logWarn('Heads up', { view: 'feed' });

    expect(captureMessage).toHaveBeenCalledWith('Heads up', {
      level: 'warning',
      extra: { view: 'feed' },
    });
  });

  it('logs error and reports exception to error tracker', () => {
    const error = new Error('Boom');
    logError('Something broke', { view: 'feed' }, error);

    expect(captureError).toHaveBeenCalledWith(error, {
      level: 'error',
      extra: { view: 'feed' },
    });
  });
});
