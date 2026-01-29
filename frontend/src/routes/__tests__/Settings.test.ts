import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';

const { default: Settings } = await import('../Settings.svelte');

describe('Settings', () => {
  it('renders the settings layout and placeholders', () => {
    render(Settings);

    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument();
    expect(screen.getByText('Account')).toBeInTheDocument();
    expect(screen.getByText('Profile picture')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Upload photo' })).toBeInTheDocument();
    expect(screen.getByText('Security')).toBeInTheDocument();
    expect(screen.getByText('Notifications')).toBeInTheDocument();
  });
});
