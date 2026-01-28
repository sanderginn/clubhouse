import { describe, it, expect } from 'vitest';
import { parseResetRoute } from '../resetLink';

describe('parseResetRoute', () => {
  it('recognizes reset path with query token', () => {
    const result = parseResetRoute({
      pathname: '/reset',
      search: '?token=abc123',
      hash: '',
    });

    expect(result.isReset).toBe(true);
    expect(result.token).toBe('abc123');
  });

  it('extracts token from reset path segment', () => {
    const result = parseResetRoute({
      pathname: '/reset/abc123',
      search: '',
      hash: '',
    });

    expect(result.isReset).toBe(true);
    expect(result.token).toBe('abc123');
  });

  it('extracts token from reset-password path segment', () => {
    const result = parseResetRoute({
      pathname: '/reset-password/abc123',
      search: '',
      hash: '',
    });

    expect(result.isReset).toBe(true);
    expect(result.token).toBe('abc123');
  });

  it('recognizes reset links routed through hash', () => {
    const result = parseResetRoute({
      pathname: '/',
      search: '',
      hash: '#/reset?token=hash-token',
    });

    expect(result.isReset).toBe(true);
    expect(result.token).toBe('hash-token');
  });

  it('returns non-reset routes without token', () => {
    const result = parseResetRoute({
      pathname: '/login',
      search: '',
      hash: '',
    });

    expect(result.isReset).toBe(false);
    expect(result.token).toBeNull();
  });

  it('does not treat non-reset routes as reset even with token query', () => {
    const result = parseResetRoute({
      pathname: '/',
      search: '?token=fallback-token',
      hash: '',
    });

    expect(result.isReset).toBe(false);
    expect(result.token).toBeNull();
  });
});
