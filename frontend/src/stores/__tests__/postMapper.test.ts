import { describe, it, expect } from 'vitest';
import { mapApiPost } from '../postMapper';

describe('mapApiPost', () => {
  it('maps ids, user, counts, timestamps, and metadata', () => {
    const post = mapApiPost({
      id: 'post-1',
      user_id: 'user-1',
      section_id: 'section-1',
      content: 'hello',
      comment_count: 2,
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-02T00:00:00Z',
      user: {
        id: 'user-1',
        username: 'sander',
        profile_picture_url: 'https://example.com/avatar.png',
      },
      links: [
        {
          id: 'link-1',
          url: 'https://example.com',
          metadata: {
            url: 'https://example.com',
            provider: 'example',
            title: 'Example',
            description: 'Desc',
            image: 'https://example.com/image.png',
            author: 'Author',
            duration: 120,
            embedUrl: 'https://example.com/embed',
          },
        },
      ],
    });

    expect(post.id).toBe('post-1');
    expect(post.user?.username).toBe('sander');
    expect(post.commentCount).toBe(2);
    expect(post.links?.[0].metadata?.embedUrl).toBe('https://example.com/embed');
    expect(post.links?.[0].metadata?.author).toBe('Author');
    expect(post.links?.[0].metadata?.duration).toBe(120);
  });

  it('handles missing user and links gracefully', () => {
    const post = mapApiPost({
      id: 'post-2',
      user_id: 'user-2',
      section_id: 'section-2',
      content: 'hello',
      created_at: '2025-01-01T00:00:00Z',
    });

    expect(post.user).toBeUndefined();
    expect(post.links).toBeUndefined();
  });

  it('falls back to link url when metadata url is missing', () => {
    const post = mapApiPost({
      id: 'post-3',
      user_id: 'user-3',
      section_id: 'section-3',
      content: 'hello',
      created_at: '2025-01-01T00:00:00Z',
      links: [
        {
          id: 'link-3',
          url: 'https://example.com/photo.png',
          metadata: {
            image: 'https://cdn.example.com/photo.png',
          },
        },
      ],
    });

    expect(post.links?.[0].metadata?.url).toBe('https://example.com/photo.png');
    expect(post.links?.[0].metadata?.image).toBe('https://cdn.example.com/photo.png');
  });

  it('falls back to link url when metadata url is empty', () => {
    const post = mapApiPost({
      id: 'post-4',
      user_id: 'user-4',
      section_id: 'section-4',
      content: 'hello',
      created_at: '2025-01-01T00:00:00Z',
      links: [
        {
          id: 'link-4',
          url: 'https://example.com/photo.png',
          metadata: {
            url: '',
            image: 'https://cdn.example.com/photo.png',
          },
        },
      ],
    });

    expect(post.links?.[0].metadata?.url).toBe('https://example.com/photo.png');
    expect(post.links?.[0].metadata?.image).toBe('https://cdn.example.com/photo.png');
  });
});
