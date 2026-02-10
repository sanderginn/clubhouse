import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { authStore, notificationStore, sectionStore, uiStore } from '../../stores';

const { default: Nav } = await import('../Nav.svelte');

beforeEach(() => {
  uiStore.setActiveView('feed');
  sectionStore.setSections([
    { id: 'section-1', name: 'Music', type: 'music', icon: '🎵', slug: 'music' },
    { id: 'section-2', name: 'Books', type: 'book', icon: '📚', slug: 'books' },
  ]);
  authStore.setUser({
    id: 'admin-1',
    username: 'admin',
    email: 'admin@example.com',
    isAdmin: true,
    totpEnabled: false,
  });
  notificationStore.setNotifications([], null, false, 0);
});

afterEach(() => {
  cleanup();
});

describe('Nav', () => {
  it('clicking section sets active section', async () => {
    const setActiveSpy = vi.spyOn(sectionStore, 'setActiveSection');
    const pushStateSpy = vi.spyOn(window.history, 'pushState');
    render(Nav);

    const button = screen.getByText('Books');
    await fireEvent.click(button);

    expect(setActiveSpy).toHaveBeenCalled();
    const call = setActiveSpy.mock.calls[0]?.[0];
    expect(call?.id).toBe('section-2');
    expect(pushStateSpy).toHaveBeenCalledWith(null, '', '/sections/books');
  });

  it('shows moderation badge for unread registration notifications', () => {
    notificationStore.setNotifications(
      [
        {
          id: 'notif-1',
          type: 'user_registration_pending',
          createdAt: '2026-02-01T00:00:00Z',
          readAt: null,
        },
        {
          id: 'notif-2',
          type: 'new_post',
          createdAt: '2026-02-01T00:00:00Z',
          readAt: null,
        },
      ],
      null,
      false,
      2
    );

    render(Nav);

    expect(screen.getByLabelText('Pending registrations')).toBeInTheDocument();
    expect(screen.getByText('1')).toBeInTheDocument();
  });

  it('does not render a standalone My Movies navigation entry', () => {
    render(Nav);

    expect(screen.queryByRole('button', { name: 'My Movies' })).not.toBeInTheDocument();
  });

  it('does not render a standalone Bookshelf navigation entry', () => {
    render(Nav);

    expect(screen.queryByRole('button', { name: 'Bookshelf' })).not.toBeInTheDocument();
  });
});
