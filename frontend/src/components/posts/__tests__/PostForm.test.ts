import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';
import { authStore, sectionStore, postStore } from '../../../stores';
import { afterEach } from 'vitest';

const createPost = vi.hoisted(() => vi.fn());
const previewLink = vi.hoisted(() => vi.fn());
const parseRecipe = vi.hoisted(() => vi.fn());
const uploadImage = vi.hoisted(() => vi.fn());
const loadSectionLinks = vi.hoisted(() => vi.fn());

vi.mock('../../../services/api', () => ({
  api: {
    createPost,
    previewLink,
    parseRecipe,
    uploadImage,
  },
}));

vi.mock('../../../stores/sectionLinksFeedStore', () => ({
  loadSectionLinks,
}));

const { default: PostForm } = await import('../PostForm.svelte');

function setAuthenticated() {
  authStore.setUser({
    id: 'user-1',
    username: 'sander',
    email: 'sander@example.com',
    isAdmin: false,
    totpEnabled: false,
  });
}

function setActiveSection(type: 'music' | 'general' | 'recipe' | 'podcast' = 'music') {
  const byType = {
    music: { name: 'Music', icon: '🎵', slug: 'music' },
    recipe: { name: 'Recipes', icon: '🍲', slug: 'recipes' },
    podcast: { name: 'Podcasts', icon: '🎙️', slug: 'podcasts' },
    general: { name: 'General', icon: '💬', slug: 'general' },
  };
  const active = byType[type];
  sectionStore.setActiveSection({
    id: 'section-1',
    name: active.name,
    type,
    icon: active.icon,
    slug: active.slug,
  });
}

beforeEach(() => {
  createPost.mockReset();
  previewLink.mockReset();
  parseRecipe.mockReset();
  uploadImage.mockReset();
  loadSectionLinks.mockReset();
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

  it('parses recipe metadata on demand in recipe section', async () => {
    setAuthenticated();
    setActiveSection('recipe');
    previewLink.mockResolvedValue({
      metadata: {
        url: 'https://example.com',
        title: 'Example',
      },
    });
    parseRecipe.mockResolvedValue({
      metadata: {
        url: 'https://example.com',
        recipe: {
          name: 'Test Recipe',
          ingredients: ['1 cup flour'],
        },
      },
    });

    render(PostForm);

    const textarea = screen.getByLabelText('Post content');
    await fireEvent.input(textarea, { target: { value: 'Check https://example.com' } });
    await tick();

    const parseButton = screen.getByRole('button', { name: /parse recipe/i });
    await fireEvent.click(parseButton);
    await tick();

    expect(parseRecipe).toHaveBeenCalledWith('https://example.com');
    expect(screen.getByTestId('recipe-title')).toHaveTextContent('Test Recipe');
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

  it('submits with a link and empty content', async () => {
    setAuthenticated();
    setActiveSection();
    previewLink.mockResolvedValue({
      metadata: {
        url: 'https://example.com',
        title: 'Example',
      },
    });
    createPost.mockResolvedValue({
      post: { id: 'post-1', userId: 'user-1', sectionId: 'section-1', content: '', createdAt: 'now' },
    });

    const { container } = render(PostForm);

    const addLinkButton = screen.getByLabelText('Add link');
    await fireEvent.click(addLinkButton);

    const linkInput = screen.getByLabelText('Link URL');
    await fireEvent.input(linkInput, { target: { value: 'example.com' } });
    await fireEvent.keyDown(linkInput, { key: 'Enter' });
    await tick();

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);
    await tick();

    expect(createPost).toHaveBeenCalledWith({
      sectionId: 'section-1',
      content: '',
      links: [{ url: 'https://example.com' }],
      mentionUsernames: [],
    });
  });

  it('shows highlight editor only for music sections with a link', async () => {
    setAuthenticated();
    setActiveSection('general');
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

    expect(screen.queryByLabelText('Timestamp (mm:ss or hh:mm:ss)')).not.toBeInTheDocument();

    setActiveSection();
    await tick();

    expect(screen.getByLabelText('Timestamp (mm:ss or hh:mm:ss)')).toBeInTheDocument();
  });

  it('includes highlights in link payload when added', async () => {
    setAuthenticated();
    setActiveSection();
    previewLink.mockResolvedValue({
      metadata: {
        url: 'https://example.com',
        title: 'Example',
      },
    });
    createPost.mockResolvedValue({
      post: { id: 'post-1', userId: 'user-1', sectionId: 'section-1', content: '', createdAt: 'now' },
    });

    const { container } = render(PostForm);
    const addLinkButton = screen.getByLabelText('Add link');
    await fireEvent.click(addLinkButton);

    const linkInput = screen.getByLabelText('Link URL');
    await fireEvent.input(linkInput, { target: { value: 'example.com' } });
    await fireEvent.keyDown(linkInput, { key: 'Enter' });
    await tick();

    const timestampInput = screen.getByLabelText('Timestamp (mm:ss or hh:mm:ss)');
    const labelInput = screen.getByLabelText('Label (optional)');
    await fireEvent.input(timestampInput, { target: { value: '01:30' } });
    await fireEvent.input(labelInput, { target: { value: 'Intro' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Add highlight' }));

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);
    await tick();

    expect(createPost).toHaveBeenCalledWith({
      sectionId: 'section-1',
      content: '',
      links: [{ url: 'https://example.com', highlights: [{ timestamp: 90, label: 'Intro' }] }],
      mentionUsernames: [],
    });
  });

  it('preserves highlights when link metadata normalizes the url', async () => {
    setAuthenticated();
    setActiveSection();

    let resolvePreview: ((value: { metadata: { url: string; title: string } }) => void) | null =
      null;
    previewLink.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolvePreview = resolve;
        })
    );

    render(PostForm);

    const addLinkButton = screen.getByLabelText('Add link');
    await fireEvent.click(addLinkButton);

    const linkInput = screen.getByLabelText('Link URL');
    await fireEvent.input(linkInput, { target: { value: 'example.com' } });
    await fireEvent.keyDown(linkInput, { key: 'Enter' });
    await tick();

    const timestampInput = screen.getByLabelText('Timestamp (mm:ss or hh:mm:ss)');
    await fireEvent.input(timestampInput, { target: { value: '00:30' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Add highlight' }));

    if (!resolvePreview) {
      throw new Error('preview resolver not set');
    }
    resolvePreview({ metadata: { url: 'https://example.com/', title: 'Example' } });
    await tick();

    expect(screen.getByText('00:30')).toBeInTheDocument();
  });

  it('shows podcast kind controls and only shows highlighted episodes editor for show posts', async () => {
    setAuthenticated();
    setActiveSection('podcast');
    previewLink.mockResolvedValue({
      metadata: {
        url: 'https://example.com/podcast',
        title: 'Example Podcast',
      },
    });

    render(PostForm);

    const addLinkButton = screen.getByLabelText('Add link');
    await fireEvent.click(addLinkButton);

    const linkInput = screen.getByLabelText('Link URL');
    await fireEvent.input(linkInput, { target: { value: 'example.com/podcast' } });
    await fireEvent.keyDown(linkInput, { key: 'Enter' });
    await tick();

    const kindSelect = screen.getByLabelText('Podcast kind');
    expect(kindSelect).toBeInTheDocument();
    expect(screen.queryByLabelText('Highlight episode title')).not.toBeInTheDocument();

    await fireEvent.change(kindSelect, { target: { value: 'show' } });
    await tick();
    expect(screen.getByLabelText('Highlight episode title')).toBeInTheDocument();

    await fireEvent.change(kindSelect, { target: { value: 'episode' } });
    await tick();
    expect(screen.queryByLabelText('Highlight episode title')).not.toBeInTheDocument();
  });

  it('adds and removes highlighted episodes for podcast show posts', async () => {
    setAuthenticated();
    setActiveSection('podcast');
    previewLink.mockResolvedValue({
      metadata: {
        url: 'https://example.com/podcast',
        title: 'Example Podcast',
      },
    });

    render(PostForm);

    const addLinkButton = screen.getByLabelText('Add link');
    await fireEvent.click(addLinkButton);

    const linkInput = screen.getByLabelText('Link URL');
    await fireEvent.input(linkInput, { target: { value: 'example.com/podcast' } });
    await fireEvent.keyDown(linkInput, { key: 'Enter' });
    await tick();

    const kindSelect = screen.getByLabelText('Podcast kind');
    await fireEvent.change(kindSelect, { target: { value: 'show' } });
    await tick();

    await fireEvent.input(screen.getByLabelText('Highlight episode title'), {
      target: { value: 'Episode 1' },
    });
    await fireEvent.input(screen.getByLabelText('Highlight episode url'), {
      target: { value: 'example.com/episode-1' },
    });
    await fireEvent.input(screen.getByLabelText('Highlight episode note'), {
      target: { value: 'Start here' },
    });
    await fireEvent.click(screen.getByRole('button', { name: 'Add highlighted episode' }));

    expect(screen.getByText('Episode 1')).toBeInTheDocument();
    expect(screen.getByText('https://example.com/episode-1')).toBeInTheDocument();
    expect(screen.getByText('Start here')).toBeInTheDocument();

    await fireEvent.click(screen.getByLabelText('Remove highlighted episode 1'));
    expect(screen.queryByText('Episode 1')).not.toBeInTheDocument();
  });

  it('blocks uncertain podcast kind submissions until user selects a kind', async () => {
    setAuthenticated();
    setActiveSection('podcast');
    previewLink.mockResolvedValue({
      metadata: {
        url: 'https://example.com/podcast',
        title: 'Example Podcast',
      },
    });
    const uncertainError = Object.assign(
      new Error(
        'Could not determine whether this podcast link is a show or an episode. Please select one and try again.'
      ),
      { podcastKindSelectionRequired: true }
    );
    createPost
      .mockRejectedValueOnce(uncertainError)
      .mockResolvedValueOnce({
        post: {
          id: 'post-1',
          userId: 'user-1',
          sectionId: 'section-1',
          content: '',
          createdAt: 'now',
        },
      });

    const { container } = render(PostForm);

    const addLinkButton = screen.getByLabelText('Add link');
    await fireEvent.click(addLinkButton);

    const linkInput = screen.getByLabelText('Link URL');
    await fireEvent.input(linkInput, { target: { value: 'example.com/podcast' } });
    await fireEvent.keyDown(linkInput, { key: 'Enter' });
    await tick();

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);
    await tick();

    expect(createPost).toHaveBeenCalledTimes(1);
    expect(createPost).toHaveBeenNthCalledWith(1, {
      sectionId: 'section-1',
      content: '',
      links: [{ url: 'https://example.com/podcast', podcast: {} }],
      mentionUsernames: [],
    });

    const postButton = screen.getByRole('button', { name: 'Post' });
    expect(postButton).toBeDisabled();
    expect(
      screen.getAllByText(
        'Could not determine whether this podcast link is a show or an episode. Please select one and try again.'
      ).length
    ).toBeGreaterThan(0);

    await fireEvent.submit(form);
    await tick();
    expect(createPost).toHaveBeenCalledTimes(1);

    await fireEvent.change(screen.getByLabelText('Podcast kind'), { target: { value: 'show' } });
    await tick();
    expect(postButton).not.toBeDisabled();

    await fireEvent.submit(form);
    await tick();

    expect(createPost).toHaveBeenCalledTimes(2);
    expect(createPost).toHaveBeenNthCalledWith(2, {
      sectionId: 'section-1',
      content: '',
      links: [{ url: 'https://example.com/podcast', podcast: { kind: 'show' } }],
      mentionUsernames: [],
    });
  });

  it('serializes highlighted podcast episodes into create payload for show posts', async () => {
    setAuthenticated();
    setActiveSection('podcast');
    previewLink.mockResolvedValue({
      metadata: {
        url: 'https://example.com/podcast',
        title: 'Example Podcast',
      },
    });
    createPost.mockResolvedValue({
      post: { id: 'post-1', userId: 'user-1', sectionId: 'section-1', content: '', createdAt: 'now' },
    });

    const { container } = render(PostForm);
    const addLinkButton = screen.getByLabelText('Add link');
    await fireEvent.click(addLinkButton);

    const linkInput = screen.getByLabelText('Link URL');
    await fireEvent.input(linkInput, { target: { value: 'example.com/podcast' } });
    await fireEvent.keyDown(linkInput, { key: 'Enter' });
    await tick();

    await fireEvent.change(screen.getByLabelText('Podcast kind'), { target: { value: 'show' } });
    await tick();

    await fireEvent.input(screen.getByLabelText('Highlight episode title'), {
      target: { value: ' Episode 1 ' },
    });
    await fireEvent.input(screen.getByLabelText('Highlight episode url'), {
      target: { value: ' example.com/episode-1 ' },
    });
    await fireEvent.input(screen.getByLabelText('Highlight episode note'), {
      target: { value: ' Start here ' },
    });
    await fireEvent.click(screen.getByRole('button', { name: 'Add highlighted episode' }));

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);
    await tick();

    expect(createPost).toHaveBeenCalledWith({
      sectionId: 'section-1',
      content: '',
      links: [
        {
          url: 'https://example.com/podcast',
          podcast: {
            kind: 'show',
            highlightEpisodes: [
              {
                title: 'Episode 1',
                url: 'https://example.com/episode-1',
                note: 'Start here',
              },
            ],
          },
        },
      ],
      mentionUsernames: [],
    });
  });

  it('handles file attachments', async () => {
    setAuthenticated();
    setActiveSection();

    const { container } = render(PostForm);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File(['hello'], 'hello.png', { type: 'image/png' });

    await fireEvent.change(fileInput, { target: { files: [file] } });
    expect(screen.getByText('hello.png')).toBeInTheDocument();

    const removeButtons = screen.getAllByLabelText('Remove file');
    await fireEvent.click(removeButtons[0]);
    expect(screen.queryByText('hello.png')).not.toBeInTheDocument();
  });

  it('enforces a maximum image count and shows a warning', async () => {
    setAuthenticated();
    setActiveSection();

    const { container } = render(PostForm);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    const files = Array.from({ length: 11 }, (_, index) => {
      return new File([`image-${index}`], `photo-${index}.png`, { type: 'image/png' });
    });

    await fireEvent.change(fileInput, { target: { files } });

    expect(screen.getByText('You can upload up to 10 images per post.')).toBeInTheDocument();
    expect(screen.getByText('10 of 10 images selected')).toBeInTheDocument();
  });

  it('allows reordering images with move controls', async () => {
    setAuthenticated();
    setActiveSection();

    const { container } = render(PostForm);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    const first = new File(['one'], 'first.png', { type: 'image/png' });
    const second = new File(['two'], 'second.png', { type: 'image/png' });

    await fireEvent.change(fileInput, { target: { files: [first, second] } });

    const moveDownButtons = screen.getAllByLabelText('Move image down');
    await fireEvent.click(moveDownButtons[0]);

    const names = Array.from(container.querySelectorAll('[data-testid="upload-filename"]')).map(
      (node) => node.textContent
    );
    expect(names).toEqual(['second.png', 'first.png']);
  });

  it('blocks non-image files and prevents submit', async () => {
    setAuthenticated();
    setActiveSection();

    const { container } = render(PostForm);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File(['hello'], 'hello.txt', { type: 'text/plain' });

    await fireEvent.change(fileInput, { target: { files: [file] } });
    expect(screen.getByText('Only image files are supported.')).toBeInTheDocument();

    const textarea = screen.getByLabelText('Post content');
    await fireEvent.input(textarea, { target: { value: 'Hello' } });

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);
    await tick();

    expect(screen.getByText('Remove invalid files before posting.')).toBeInTheDocument();
    expect(createPost).not.toHaveBeenCalled();
  });

  it('blocks svg uploads on the client', async () => {
    setAuthenticated();
    setActiveSection();

    const { container } = render(PostForm);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File(['<svg></svg>'], 'icon.svg', { type: 'image/svg+xml' });

    await fireEvent.change(fileInput, { target: { files: [file] } });
    expect(screen.getByText('Only image files are supported.')).toBeInTheDocument();
  });

  it('rejects oversized uploads before submit', async () => {
    setAuthenticated();
    setActiveSection();

    const { container } = render(PostForm);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File(['image'], 'large.png', { type: 'image/png' });
    Object.defineProperty(file, 'size', { value: 11 * 1024 * 1024 });

    await fireEvent.change(fileInput, { target: { files: [file] } });
    expect(screen.getByText('Images must be 10 MB or smaller.')).toBeInTheDocument();
  });

  it('uploads an image and includes it in the post images', async () => {
    setAuthenticated();
    setActiveSection();
    uploadImage.mockImplementation(async (_file: File, onProgress?: (progress: number) => void) => {
      onProgress?.(35);
      onProgress?.(100);
      return { url: '/api/v1/uploads/user-1/photo.png' };
    });
    createPost.mockResolvedValue({
      post: { id: 'post-1', userId: 'user-1', sectionId: 'section-1', content: '', createdAt: 'now' },
    });

    const { container } = render(PostForm);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File(['image'], 'photo.png', { type: 'image/png' });

    await fireEvent.change(fileInput, { target: { files: [file] } });

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);
    await tick();

    expect(uploadImage).toHaveBeenCalled();
    expect(createPost).toHaveBeenCalledWith({
      sectionId: 'section-1',
      content: '',
      images: [{ url: '/api/v1/uploads/user-1/photo.png' }],
      mentionUsernames: [],
    });
    expect(screen.getByText('Uploaded')).toBeInTheDocument();
  });

  it('shows upload progress while uploading', async () => {
    setAuthenticated();
    setActiveSection();

    let resolveUpload: ((value: { url: string }) => void) | null = null;
    uploadImage.mockImplementation(async (_file: File, onProgress?: (progress: number) => void) => {
      onProgress?.(35);
      return new Promise((resolve) => {
        resolveUpload = resolve;
      });
    });
    createPost.mockResolvedValue({
      post: { id: 'post-1', userId: 'user-1', sectionId: 'section-1', content: '', createdAt: 'now' },
    });

    const { container } = render(PostForm);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File(['image'], 'photo.png', { type: 'image/png' });

    await fireEvent.change(fileInput, { target: { files: [file] } });

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);
    await tick();

    expect(screen.getByText('35%')).toBeInTheDocument();

    if (!resolveUpload) {
      throw new Error('upload resolver not set');
    }
    resolveUpload({ url: '/api/v1/uploads/user-1/photo.png' });
    await new Promise((resolve) => setTimeout(resolve, 0));
    await tick();
    await tick();

    expect(createPost).toHaveBeenCalled();
  });

  it('shows upload failure messaging', async () => {
    setAuthenticated();
    setActiveSection();
    uploadImage.mockRejectedValue(new Error('Upload failed'));

    const { container } = render(PostForm);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File(['image'], 'photo.png', { type: 'image/png' });

    await fireEvent.change(fileInput, { target: { files: [file] } });

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);
    await tick();

    expect(screen.getByText('Upload failed')).toBeInTheDocument();
    expect(screen.getByText('Some uploads failed. Remove the failed files and try again.')).toBeInTheDocument();
    expect(createPost).not.toHaveBeenCalled();
  });
});
