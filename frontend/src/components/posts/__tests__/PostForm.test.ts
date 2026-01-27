import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';
import { authStore, sectionStore, postStore } from '../../../stores';
import { afterEach } from 'vitest';

const createPost = vi.hoisted(() => vi.fn());
const previewLink = vi.hoisted(() => vi.fn());

vi.mock('../../../services/api', () => ({
  api: {
    createPost,
    previewLink,
  },
}));

const { default: PostForm } = await import('../PostForm.svelte');

function setAuthenticated() {
  authStore.setUser({
    id: 'user-1',
    username: 'sander',
    email: 'sander@example.com',
    isAdmin: false,
  });
}

function setActiveSection() {
  sectionStore.setActiveSection({
    id: 'section-1',
    name: 'Music',
    type: 'music',
    icon: '🎵',
  });
}

beforeEach(() => {
  createPost.mockReset();
  previewLink.mockReset();
  authStore.setUser(null);
  sectionStore.setActiveSection(null);
  postStore.reset();
});

afterEach(() => {
  cleanup();
});

describe('PostForm', () => {
  it('does not submit when content is empty or missing context', async () => {
    setAuthenticated();
    const { container } = render(PostForm);
    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');

    await fireEvent.submit(form);
    expect(createPost).not.toHaveBeenCalled();

    setActiveSection();
    authStore.setUser(null);
    await fireEvent.submit(form);
    expect(createPost).not.toHaveBeenCalled();
  });

  it('submits successfully and clears input', async () => {
    setAuthenticated();
    setActiveSection();
    createPost.mockResolvedValue({
      post: { id: 'post-1', userId: 'user-1', sectionId: 'section-1', content: 'Hello', createdAt: 'now' },
    });
    const addPostSpy = vi.spyOn(postStore, 'addPost');

    const { container } = render(PostForm);
    const textarea = screen.getByLabelText('Post content') as HTMLTextAreaElement;
    await fireEvent.input(textarea, { target: { value: 'Hello' } });

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);
    await tick();

    expect(createPost).toHaveBeenCalled();
    expect(addPostSpy).toHaveBeenCalled();
    expect(textarea.value).toBe('');
  });

  it('shows error on submit failure', async () => {
    setAuthenticated();
    setActiveSection();
    createPost.mockRejectedValue(new Error('boom'));

    const { container } = render(PostForm);
    const textarea = screen.getByLabelText('Post content');
    await fireEvent.input(textarea, { target: { value: 'Hello' } });

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);
    await tick();

    expect(screen.getByText('boom')).toBeInTheDocument();
  });

  it('detects a link and renders preview', async () => {
    setAuthenticated();
    setActiveSection();
    previewLink.mockResolvedValue({
      metadata: {
        url: 'https://example.com',
        title: 'Example',
      },
    });

    render(PostForm);

    const textarea = screen.getByLabelText('Post content');
    await fireEvent.input(textarea, { target: { value: 'Check https://example.com' } });
    await tick();

    expect(previewLink).toHaveBeenCalledWith('https://example.com');
    expect(screen.getByText('Example')).toBeInTheDocument();
  });

  it('includes link metadata in newly created post', async () => {
    setAuthenticated();
    setActiveSection();
    previewLink.mockResolvedValue({
      metadata: {
        url: 'https://example.com',
        title: 'Example',
      },
    });
    createPost.mockResolvedValue({
      post: { id: 'post-1', userId: 'user-1', sectionId: 'section-1', content: 'Hello', createdAt: 'now' },
    });
    const addPostSpy = vi.spyOn(postStore, 'addPost');

    const { container } = render(PostForm);
    const textarea = screen.getByLabelText('Post content');
    await fireEvent.input(textarea, { target: { value: 'Check https://example.com' } });
    await tick();

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);
    await tick();

    const addedPost = addPostSpy.mock.calls[0][0];
    expect(addedPost.links?.[0].url).toBe('https://example.com');
    expect(addedPost.links?.[0].metadata?.title).toBe('Example');
  });

  it('removes link preview', async () => {
    setAuthenticated();
    setActiveSection();
    previewLink.mockResolvedValue({
      metadata: {
        url: 'https://example.com',
        title: 'Example',
      },
    });

    render(PostForm);

    const textarea = screen.getByLabelText('Post content');
    await fireEvent.input(textarea, { target: { value: 'Check https://example.com' } });
    await tick();

    const removeButton = screen.getByLabelText('Remove link');
    await fireEvent.click(removeButton);

    expect(screen.queryByText('Example')).not.toBeInTheDocument();
  });

  it('adds a link via the inline input', async () => {
    setAuthenticated();
    setActiveSection();
    previewLink.mockResolvedValue({
      metadata: {
        url: 'https://example.com',
        title: 'Example',
      },
    });

    render(PostForm);

    const addLinkButton = screen.getByLabelText('Add link');
    await fireEvent.click(addLinkButton);

    const linkInput = screen.getByLabelText('Link URL');
    await fireEvent.input(linkInput, { target: { value: 'example.com' } });
    await fireEvent.keyDown(linkInput, { key: 'Enter' });
    await tick();

    expect(previewLink).toHaveBeenCalledWith('https://example.com');
    expect(screen.getByText('Example')).toBeInTheDocument();
  });

  it('handles file attachments', async () => {
    setAuthenticated();
    setActiveSection();

    const { container } = render(PostForm);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File(['hello'], 'hello.txt', { type: 'text/plain' });

    await fireEvent.change(fileInput, { target: { files: [file] } });
    expect(screen.getByText('hello.txt')).toBeInTheDocument();

    const removeButtons = screen.getAllByLabelText('Remove file');
    await fireEvent.click(removeButtons[0]);
    expect(screen.queryByText('hello.txt')).not.toBeInTheDocument();
  });
});
