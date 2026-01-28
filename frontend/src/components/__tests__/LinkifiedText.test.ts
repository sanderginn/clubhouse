import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/svelte';

const { default: LinkifiedText } = await import('../LinkifiedText.svelte');

afterEach(() => {
  cleanup();
});

describe('LinkifiedText', () => {
  it('renders plain text without links', () => {
    render(LinkifiedText, { text: 'Hello world' });
    expect(screen.getByText('Hello world')).toBeInTheDocument();
  });

  it('linkifies URLs with safe attributes', () => {
    render(LinkifiedText, { text: 'See https://example.com for details' });

    const link = screen.getByRole('link', { name: 'https://example.com' });
    expect(link).toHaveAttribute('href', 'https://example.com');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    expect(link).toHaveAttribute('target', '_blank');
  });

  it('renders multiple links in the same text', () => {
    render(LinkifiedText, { text: 'Go to https://one.com and https://two.com' });

    expect(screen.getByRole('link', { name: 'https://one.com' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'https://two.com' })).toBeInTheDocument();
  });
});
