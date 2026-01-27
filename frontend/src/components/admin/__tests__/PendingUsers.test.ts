import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';

const apiGet = vi.hoisted(() => vi.fn());
const apiPatch = vi.hoisted(() => vi.fn());
const apiDelete = vi.hoisted(() => vi.fn());

vi.mock('../../../services/api', () => ({
  api: {
    get: apiGet,
    patch: apiPatch,
    delete: apiDelete,
  },
}));

const { default: PendingUsers } = await import('../PendingUsers.svelte');

const flushPendingUsersLoad = async () => {
  await tick();
  await Promise.resolve();
  await tick();
};

const createDeferred = <T,>() => {
  let resolve: (value: T) => void;
  let reject: (reason?: unknown) => void;
  const promise = new Promise<T>((resolveFn, rejectFn) => {
    resolve = resolveFn;
    reject = rejectFn;
  });
  return { promise, resolve: resolve!, reject: reject! };
};

beforeEach(() => {
  apiGet.mockReset();
  apiPatch.mockReset();
  apiDelete.mockReset();
});

afterEach(() => {
  cleanup();
});

describe('PendingUsers', () => {
  it('renders pending users from API', async () => {
    const deferred = createDeferred<
      Array<{
        id: string;
        username: string;
        email: string;
        created_at: string;
      }>
    >();
    apiGet.mockReturnValue(deferred.promise);

    deferred.resolve([
      {
        id: 'user-1',
        username: 'lena',
        email: 'lena@example.com',
        created_at: '2024-01-01T00:00:00Z',
      },
    ]);

    render(PendingUsers);
    await fireEvent.click(screen.getByText('Refresh'));
    await flushPendingUsersLoad();

    expect(await screen.findByText('lena')).toBeInTheDocument();
    expect(screen.getByText('lena@example.com')).toBeInTheDocument();
    if (typeof AbortController === 'undefined') {
      expect(apiGet).toHaveBeenCalledWith('/admin/users');
    } else {
      expect(apiGet).toHaveBeenCalledWith(
        '/admin/users',
        expect.objectContaining({ signal: expect.any(AbortSignal) })
      );
    }
  });

  it('approves a user and removes them from the list', async () => {
    const deferred = createDeferred<
      Array<{
        id: string;
        username: string;
        email: string;
        created_at: string;
      }>
    >();
    apiGet.mockReturnValue(deferred.promise);
    apiPatch.mockResolvedValue({});

    render(PendingUsers);
    await fireEvent.click(screen.getByText('Refresh'));
    deferred.resolve([
      {
        id: 'user-2',
        username: 'marco',
        email: 'marco@example.com',
        created_at: '2024-02-01T00:00:00Z',
      },
    ]);
    await flushPendingUsersLoad();
    await screen.findByText('marco');

    const approveButton = screen.getByText('Approve');
    await fireEvent.click(approveButton);
    await tick();

    expect(apiPatch).toHaveBeenCalledWith('/admin/users/user-2/approve');
    expect(screen.queryByText('marco')).not.toBeInTheDocument();
  });

  it('rejects a user and removes them from the list', async () => {
    const deferred = createDeferred<
      Array<{
        id: string;
        username: string;
        email: string;
        created_at: string;
      }>
    >();
    apiGet.mockReturnValue(deferred.promise);
    apiDelete.mockResolvedValue({});

    render(PendingUsers);
    await fireEvent.click(screen.getByText('Refresh'));
    deferred.resolve([
      {
        id: 'user-3',
        username: 'sol',
        email: 'sol@example.com',
        created_at: '2024-03-01T00:00:00Z',
      },
    ]);
    await flushPendingUsersLoad();
    await screen.findByText('sol');

    const rejectButton = screen.getByText('Reject');
    await fireEvent.click(rejectButton);
    await tick();

    expect(apiDelete).toHaveBeenCalledWith('/admin/users/user-3');
    expect(screen.queryByText('sol')).not.toBeInTheDocument();
  });

  it('shows a timeout message when the request is aborted', async () => {
    const deferred = createDeferred<never>();
    apiGet.mockReturnValue(deferred.promise);

    render(PendingUsers);
    await fireEvent.click(screen.getByText('Refresh'));
    deferred.reject(Object.assign(new Error('Request aborted'), { name: 'AbortError' }));
    await flushPendingUsersLoad();

    expect(await screen.findByText('Request timed out. Please try again.')).toBeInTheDocument();
  });

  it('treats a null response as no pending users', async () => {
    apiGet.mockResolvedValue(null);

    render(PendingUsers);
    await fireEvent.click(screen.getByText('Refresh'));
    await flushPendingUsersLoad();

    expect(await screen.findByText('All caught up. No pending approvals right now.')).toBeInTheDocument();
  });
});
