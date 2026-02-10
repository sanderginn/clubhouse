<script lang="ts">
  type BookData = {
    title?: string;
    authors?: string[];
    description?: string;
    coverUrl?: string;
    cover_url?: string;
    pageCount?: number;
    page_count?: number;
    genres?: string[];
    publishDate?: string;
    publish_date?: string;
    openLibraryKey?: string;
    open_library_key?: string;
    goodreadsUrl?: string;
    goodreads_url?: string;
  };

  export let bookData: BookData = {};
  export let compact = false;
  export let threadHref: string | null = null;

  let descriptionExpanded = false;
  let previousDescription = '';

  $: title = bookData.title?.trim() || 'Untitled book';
  $: authors = (bookData.authors ?? []).filter(
    (author): author is string => typeof author === 'string' && author.trim().length > 0
  );
  $: authorsLabel = authors.length > 0 ? authors.join(', ') : 'Unknown author';
  $: description = bookData.description?.trim() ?? '';
  $: if (description !== previousDescription) {
    previousDescription = description;
    descriptionExpanded = false;
  }
  $: descriptionLineClampClass = descriptionExpanded ? '' : compact ? 'line-clamp-2' : 'line-clamp-3';
  $: showDescriptionToggle = description.length > (compact ? 120 : 180);
  $: coverUrl = normalizeURL(bookData.coverUrl ?? bookData.cover_url);
  $: pageCount = normalizePositiveInteger(bookData.pageCount ?? bookData.page_count);
  $: publishYear = extractPublishYear(bookData.publishDate ?? bookData.publish_date);
  $: details = [
    typeof pageCount === 'number' ? `${pageCount} pages` : null,
    publishYear,
  ].filter((entry): entry is string => typeof entry === 'string' && entry.length > 0);
  $: visibleGenres = (bookData.genres ?? [])
    .filter((genre): genre is string => typeof genre === 'string' && genre.trim().length > 0)
    .slice(0, 3);
  $: goodreadsUrl = normalizeURLForHosts(bookData.goodreadsUrl ?? bookData.goodreads_url, [
    'goodreads.com',
  ]);
  $: openLibraryUrl = resolveOpenLibraryURL(bookData.openLibraryKey ?? bookData.open_library_key);
  $: normalizedThreadHref = normalizeThreadHref(threadHref);

  function normalizeURL(value?: string): string | null {
    const parsed = parseAbsoluteHTTPURL(value);
    if (!parsed) {
      return null;
    }
    return parsed.toString();
  }

  function normalizeURLForHosts(value: string | undefined, hosts: string[]): string | null {
    const parsed = parseAbsoluteHTTPURL(value);
    if (!parsed) {
      return null;
    }
    const hostname = parsed.hostname.toLowerCase();
    const hasAllowedHost = hosts.some((host) => hostname === host || hostname.endsWith(`.${host}`));
    if (!hasAllowedHost) {
      return null;
    }
    return parsed.toString();
  }

  function parseAbsoluteHTTPURL(value?: string): URL | null {
    if (typeof value !== 'string') {
      return null;
    }
    const trimmed = value.trim();
    if (!trimmed) {
      return null;
    }
    try {
      const parsed = new URL(trimmed);
      if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
        return null;
      }
      return parsed;
    } catch {
      return null;
    }
  }

  function normalizeThreadHref(value?: string | null): string | null {
    if (typeof value !== 'string') {
      return null;
    }
    const trimmed = value.trim();
    if (!trimmed) {
      return null;
    }
    if (trimmed.startsWith('/')) {
      return trimmed;
    }
    return normalizeURL(trimmed);
  }

  function normalizePositiveInteger(value: unknown): number | null {
    if (typeof value !== 'number' || !Number.isFinite(value)) {
      return null;
    }
    const rounded = Math.round(value);
    return rounded > 0 ? rounded : null;
  }

  function extractPublishYear(value?: string): string | null {
    if (typeof value !== 'string') {
      return null;
    }
    const match = value.trim().match(/\b(\d{4})\b/);
    return match ? match[1] : null;
  }

  function resolveOpenLibraryURL(openLibraryKey?: string): string | null {
    if (typeof openLibraryKey !== 'string') {
      return null;
    }
    const trimmed = openLibraryKey.trim();
    if (!trimmed) {
      return null;
    }
    if (trimmed.startsWith('http://') || trimmed.startsWith('https://')) {
      return normalizeURLForHosts(trimmed, ['openlibrary.org']);
    }

    const prefixed = trimmed.startsWith('/') ? trimmed : `/${trimmed}`;
    if (/^\/works\/ol[0-9a-z]+w$/i.test(prefixed) || /^\/books\/ol[0-9a-z]+m$/i.test(prefixed)) {
      return `https://openlibrary.org${prefixed}`;
    }
    if (/^ol[0-9a-z]+w$/i.test(trimmed)) {
      return `https://openlibrary.org/works/${trimmed.toUpperCase()}`;
    }
    if (/^ol[0-9a-z]+m$/i.test(trimmed)) {
      return `https://openlibrary.org/books/${trimmed.toUpperCase()}`;
    }
    return `https://openlibrary.org${prefixed}`;
  }
</script>

<article class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm" data-testid="book-card">
  <div class={`flex flex-col gap-4 p-4 ${compact ? 'sm:p-4' : 'sm:p-5'} sm:flex-row sm:items-start`}>
    <div
      class={`w-full shrink-0 overflow-hidden rounded-lg bg-slate-100 ${
        compact ? 'h-44 sm:h-36 sm:w-24' : 'h-56 sm:h-44 sm:w-[120px]'
      }`}
    >
      {#if coverUrl}
        <img
          src={coverUrl}
          alt={`${title} cover`}
          class="h-full w-full object-cover"
          loading="lazy"
          data-testid="book-cover"
        />
      {:else}
        <div
          class="flex h-full w-full items-center justify-center px-4 text-center text-sm font-medium text-slate-600"
          data-testid="book-cover-fallback"
        >
          Cover unavailable
        </div>
      {/if}
    </div>

    <div class="min-w-0 flex-1">
      <h3 class={`font-semibold text-slate-900 ${compact ? 'text-base' : 'text-lg'}`} data-testid="book-title">
        {#if normalizedThreadHref}
          <a
            href={normalizedThreadHref}
            class="underline-offset-2 hover:text-slate-700 hover:underline"
            data-testid="book-thread-link"
          >
            {title}
          </a>
        {:else}
          {title}
        {/if}
      </h3>

      <p class="mt-1 text-sm text-slate-700" data-testid="book-authors">{authorsLabel}</p>

      {#if details.length > 0}
        <p class="mt-1 text-xs text-slate-600" data-testid="book-details">{details.join(' · ')}</p>
      {/if}

      {#if description}
        <div class="mt-3">
          <p
            class={`text-sm leading-relaxed text-slate-700 ${descriptionLineClampClass}`}
            data-testid="book-description"
          >
            {description}
          </p>
          {#if showDescriptionToggle}
            <button
              type="button"
              class="mt-1 text-xs font-medium text-slate-600 underline-offset-2 hover:text-slate-800 hover:underline"
              on:click={() => (descriptionExpanded = !descriptionExpanded)}
              data-testid="book-description-toggle"
            >
              {descriptionExpanded ? 'Show less' : 'Show more'}
            </button>
          {/if}
        </div>
      {/if}

      {#if visibleGenres.length > 0}
        <div class="mt-3 flex flex-wrap gap-2" data-testid="book-genres">
          {#each visibleGenres as genre}
            <span class="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-700">
              {genre}
            </span>
          {/each}
        </div>
      {/if}

      {#if goodreadsUrl || openLibraryUrl}
        <div class="mt-4 flex flex-wrap items-center gap-3">
          {#if goodreadsUrl}
            <a
              href={goodreadsUrl}
              target="_blank"
              rel="noopener noreferrer"
              class="rounded-full bg-emerald-600 px-4 py-2 text-xs font-semibold text-white hover:bg-emerald-700"
              data-testid="book-goodreads-link"
            >
              View on Goodreads
            </a>
          {/if}
          {#if openLibraryUrl}
            <a
              href={openLibraryUrl}
              target="_blank"
              rel="noopener noreferrer"
              class="text-xs font-medium text-slate-600 underline-offset-2 hover:text-slate-800 hover:underline"
              data-testid="book-open-library-link"
            >
              View on Open Library
            </a>
          {/if}
        </div>
      {/if}
    </div>
  </div>
</article>
