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

  it('linkifies @mentions to profiles', () => {
    render(LinkifiedText, {
      text: 'Hello @sander and @alex_2',
      validMentions: ['sander', 'alex_2'],
    });

    const mentionLink = screen.getByRole('link', { name: '@sander' });
    expect(mentionLink).toHaveAttribute('href', '/users/sander');
    expect(screen.getByRole('link', { name: '@alex_2' })).toHaveAttribute('href', '/users/alex_2');
  });

  it('renders invalid @mentions as plain text', () => {
    render(LinkifiedText, {
      text: 'Hello @sander and @ghost',
      validMentions: ['sander'],
    });

    expect(screen.getByRole('link', { name: '@sander' })).toHaveAttribute('href', '/users/sander');
    expect(screen.queryByRole('link', { name: '@ghost' })).toBeNull();
    expect(screen.getByText(/@ghost/)).toBeInTheDocument();
  });

  it('renders escaped @mentions as plain text', () => {
    render(LinkifiedText, {
      text: 'Use \\@here to avoid mentions',
      validMentions: ['here'],
    });

    expect(screen.queryByRole('link', { name: '@here' })).toBeNull();
    expect(screen.getByText(/@here/)).toBeInTheDocument();
  });
});
