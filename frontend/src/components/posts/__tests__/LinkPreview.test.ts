import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';

const { default: LinkPreview } = await import('../LinkPreview.svelte');

afterEach(() => {
  cleanup();
});

describe('LinkPreview', () => {
  it('renders image when metadata.image is set', () => {
    const { getByAltText } = render(LinkPreview, {
      metadata: {
        url: 'https://example.com',
        title: 'Example',
        image: 'https://example.com/image.png',
      },
    });

    expect(getByAltText('Example')).toBeInTheDocument();
  });

  it('renders fallback when no image', () => {
    const { queryByAltText } = render(LinkPreview, {
      metadata: {
        url: 'https://example.com',
        title: 'Example',
      },
    });

    expect(queryByAltText('Example')).not.toBeInTheDocument();
  });

  it('calls remove callback', async () => {
    const onRemove = vi.fn();
    const { getByLabelText } = render(LinkPreview, {
      metadata: {
        url: 'https://example.com',
        title: 'Example',
      },
      onRemove,
    });

    await fireEvent.click(getByLabelText('Remove link'));
    expect(onRemove).toHaveBeenCalled();
  });
});
