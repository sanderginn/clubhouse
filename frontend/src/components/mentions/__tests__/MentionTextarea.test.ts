import { describe, it, expect, afterEach, vi, beforeEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/svelte';
import { writable } from 'svelte/store';

const storeRefs = {
  currentUser: writable<{ id: string; username: string } | null>(null),
  api: {
    searchUsers: vi.fn(),
  },
};

vi.mock('../../../services/api', () => ({
  api: storeRefs.api,
}));

vi.mock('../../../stores/authStore', () => ({
  currentUser: storeRefs.currentUser,
}));

const { default: MentionTextarea } = await import('../MentionTextarea.svelte');

describe('MentionTextarea', () => {
  beforeEach(() => {
    storeRefs.currentUser.set({ id: 'current-user-id', username: 'currentuser' });
  });

  afterEach(() => {
    cleanup();
    storeRefs.api.searchUsers.mockReset();
    storeRefs.currentUser.set(null);
  });

  it('excludes current user from mention suggestions', async () => {
    storeRefs.api.searchUsers.mockResolvedValue({
      users: [
        { id: 'current-user-id', username: 'currentuser', profile_picture_url: null },
        { id: 'other-user-id', username: 'otheruser', profile_picture_url: null },
        { id: 'third-user-id', username: 'thirduser', profile_picture_url: null },
      ],
    });

    render(MentionTextarea, { value: '' });

    const textarea = screen.getByRole('textbox');
    await fireEvent.input(textarea, { target: { value: '@' } });

    await waitFor(() => {
      expect(storeRefs.api.searchUsers).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(screen.queryByText('@currentuser')).not.toBeInTheDocument();
      expect(screen.getByText('@otheruser')).toBeInTheDocument();
      expect(screen.getByText('@thirduser')).toBeInTheDocument();
    });
  });

  it('shows other users when searching for mentions', async () => {
    storeRefs.api.searchUsers.mockResolvedValue({
      users: [
        { id: 'user-1', username: 'alice', profile_picture_url: null },
        { id: 'user-2', username: 'bob', profile_picture_url: null },
      ],
    });

    render(MentionTextarea, { value: '' });

    const textarea = screen.getByRole('textbox');
    await fireEvent.input(textarea, { target: { value: '@al' } });

    await waitFor(() => {
      expect(storeRefs.api.searchUsers).toHaveBeenCalledWith('al', 8);
    });

    await waitFor(() => {
      expect(screen.getByText('@alice')).toBeInTheDocument();
      expect(screen.getByText('@bob')).toBeInTheDocument();
    });
  });

  it('allows keyboard navigation of suggestions', async () => {
    storeRefs.api.searchUsers.mockResolvedValue({
      users: [
        { id: 'user-1', username: 'alice', profile_picture_url: null },
        { id: 'user-2', username: 'bob', profile_picture_url: null },
      ],
    });

    render(MentionTextarea, { value: '' });

    const textarea = screen.getByRole('textbox');
    await fireEvent.input(textarea, { target: { value: '@' } });

    await waitFor(() => {
      expect(screen.getByText('@alice')).toBeInTheDocument();
    });

    const aliceOption = screen.getByRole('option', { name: /@alice/i });
    expect(aliceOption).toHaveAttribute('aria-selected', 'true');

    await fireEvent.keyDown(textarea, { key: 'ArrowDown' });

    const bobOption = screen.getByRole('option', { name: /@bob/i });
    expect(bobOption).toHaveAttribute('aria-selected', 'true');
    expect(aliceOption).toHaveAttribute('aria-selected', 'false');
  });
});
