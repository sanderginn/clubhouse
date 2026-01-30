import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { uiStore, authStore } from '../../stores';

const { default: Sidebar } = await import('../Sidebar.svelte');
const { default: Header } = await import('../Header.svelte');

beforeEach(() => {
  uiStore.setSidebarOpen(true);
  uiStore.setIsMobile(false);
  authStore.setUser(null);
});

afterEach(() => {
  cleanup();
});

describe('Sidebar', () => {
  it('overlay click closes sidebar on mobile', async () => {
    uiStore.setIsMobile(true);
    uiStore.setSidebarOpen(true);
    const setSidebarSpy = vi.spyOn(uiStore, 'setSidebarOpen');

    render(Sidebar);

    const [overlay] = screen.getAllByLabelText('Close sidebar');
    await fireEvent.click(overlay);

    expect(setSidebarSpy).toHaveBeenCalledWith(false);
  });
});

describe('Header', () => {
  it('toggle button calls uiStore.toggleSidebar', async () => {
    const toggleSpy = vi.spyOn(uiStore, 'toggleSidebar');
    render(Header);

    const toggleButton = screen.getByLabelText('Toggle sidebar');
    await fireEvent.click(toggleButton);

    expect(toggleSpy).toHaveBeenCalled();
  });

  it('logout button calls authStore.logout', async () => {
    authStore.setUser({
      id: 'user-1',
      username: 'Sander',
      email: 'sander@example.com',
      isAdmin: false,
      totpEnabled: false,
    });
    const logoutSpy = vi.spyOn(authStore, 'logout').mockResolvedValue();

    render(Header);

    const menuButton = screen.getByRole('button', { name: /Sander/i });
    await fireEvent.click(menuButton);

    const logoutButton = screen.getByRole('menuitem', { name: 'Log out' });
    await fireEvent.click(logoutButton);

    expect(logoutSpy).toHaveBeenCalled();
  });
});
