const RESET_ROOTS = ['/reset', '/reset-password'];

export interface ResetRoute {
  isReset: boolean;
  token: string | null;
}

interface ResetLocation {
  pathname: string;
  search: string;
  hash: string;
}

function normalizePath(path: string): string {
  if (!path) return '';
  if (!path.startsWith('/')) {
    return `/${path}`;
  }
  return path;
}

function splitHash(hash: string): { path: string; query: string } {
  if (!hash) return { path: '', query: '' };
  const raw = hash.startsWith('#') ? hash.slice(1) : hash;
  if (!raw) return { path: '', query: '' };
  const [path, query = ''] = raw.split('?');
  return { path: normalizePath(path), query };
}

function isResetPath(path: string): boolean {
  if (!path) return false;
  return RESET_ROOTS.some((root) => path === root || path.startsWith(`${root}/`));
}

function decodePathToken(value: string): string | null {
  if (!value) return null;
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

function tokenFromPath(path: string): string | null {
  if (!path) return null;
  for (const root of RESET_ROOTS) {
    const prefix = `${root}/`;
    if (path.startsWith(prefix)) {
      const tokenPart = path.slice(prefix.length).split('/')[0];
      return decodePathToken(tokenPart);
    }
  }
  return null;
}

function tokenFromQuery(query: string): string | null {
  if (!query) return null;
  const normalized = query.startsWith('?') ? query : `?${query}`;
  const params = new URLSearchParams(normalized);
  const token = params.get('token');
  return token && token.trim().length > 0 ? token : null;
}

export function parseResetRoute(location: ResetLocation): ResetRoute {
  const hashParts = splitHash(location.hash);
  const searchToken = tokenFromQuery(location.search);
  const hashToken = tokenFromQuery(hashParts.query);
  const pathToken = tokenFromPath(normalizePath(location.pathname));
  const hashPathToken = tokenFromPath(hashParts.path);

  const token = searchToken ?? hashToken ?? pathToken ?? hashPathToken ?? null;
  const resetPathMatch = isResetPath(normalizePath(location.pathname)) || isResetPath(hashParts.path);
  const isReset = resetPathMatch || Boolean(token);

  return { isReset, token };
}

export function getResetTokenFromLocation(location: ResetLocation): string | null {
  return parseResetRoute(location).token;
}
