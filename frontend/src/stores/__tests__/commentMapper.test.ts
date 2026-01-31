import { describe, expect, it } from 'vitest';
import { mapApiComment } from '../commentMapper';

const apiComment = {
  id: 'comment-1',
  user_id: 'user-1',
  post_id: 'post-1',
  parent_comment_id: null,
  image_id: 'image-1',
  content: 'Hello',
  created_at: '2025-01-01T00:00:00Z',
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
        title: 'Example',
        description: 'Example link',
        image: 'https://example.com/image.png',
        provider: 'example',
      },
    },
  ],
  replies: [
    {
      id: 'reply-1',
      user_id: 'user-2',
      post_id: 'post-1',
      parent_comment_id: 'comment-1',
      content: 'Reply',
      created_at: '2025-01-01T01:00:00Z',
      user: {
        id: 'user-2',
        username: 'alex',
      },
    },
  ],
};

describe('mapApiComment', () => {
  it('maps API comment to UI comment with replies and metadata', () => {
    const comment = mapApiComment(apiComment);

    expect(comment.id).toBe('comment-1');
    expect(comment.user?.username).toBe('sander');
    expect(comment.links?.[0].metadata?.title).toBe('Example');
    expect(comment.links?.[0].metadata?.url).toBe('https://example.com');
    expect(comment.replies).toHaveLength(1);
    expect(comment.replies?.[0].parentCommentId).toBe('comment-1');
    expect(comment.imageId).toBe('image-1');
  });

  it('maps API comment without metadata', () => {
    const comment = mapApiComment({
      id: 'comment-2',
      user_id: 'user-3',
      post_id: 'post-2',
      content: 'No metadata',
      created_at: '2025-01-02T00:00:00Z',
      links: [
        {
          id: 'link-2',
          url: 'https://example.com/no-meta',
        },
      ],
    });

    expect(comment.links?.[0].metadata).toBeUndefined();
  });
});
