import { cleanup, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it } from 'vitest';
import { clearFatalError, setFatalError } from '../../observability/errorState';
import ErrorBoundaryTestWrapper from './ErrorBoundaryTestWrapper.svelte';

describe('ErrorBoundary', () => {
  afterEach(() => {
    cleanup();
    clearFatalError();
  });

  it('renders children when no fatal error', () => {
    render(ErrorBoundaryTestWrapper);

    expect(screen.getByText('App Content')).toBeInTheDocument();
  });

  it('renders fallback when fatal error is set', async () => {
    render(ErrorBoundaryTestWrapper, { props: { title: 'Oops', message: 'Try again' } });

    setFatalError({
      message: 'Boom',
      error: new Error('Boom'),
      source: 'window',
      timestamp: new Date(),
    });

    await waitFor(() => {
      expect(screen.getByText('Oops')).toBeInTheDocument();
    });

    expect(screen.queryByText('App Content')).not.toBeInTheDocument();
  });
});
