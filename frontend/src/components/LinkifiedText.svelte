<script lang="ts">
  import { buildProfileHref, handleProfileNavigation } from '../services/profileNavigation';

  export let text = '';
  export let className = '';
  export let linkClassName = 'text-blue-600 hover:text-blue-800 underline';
  export let mentionClassName =
    'text-indigo-600 hover:text-indigo-800 font-medium bg-indigo-50 rounded px-1';
  export let highlightQuery = '';

  type TextPart = { type: 'text'; value: string };
  type LinkPart = { type: 'link'; value: string };
  type MentionPart = { type: 'mention'; value: string; username: string };
  type Part = TextPart | LinkPart | MentionPart;
  type HighlightPart = { text: string; isMatch: boolean };

  const URL_REGEX = /https?:\/\/[^\s<>"{}|\\^`[\]]+/gi;
  const MENTION_REGEX = /(^|[^A-Za-z0-9_])@([A-Za-z0-9_]{3,50})/g;

  function splitMentions(input: string): Part[] {
    if (!input) return [];
    MENTION_REGEX.lastIndex = 0;
    const parts: Part[] = [];
    let lastIndex = 0;
    let match: RegExpExecArray | null;

    while ((match = MENTION_REGEX.exec(input)) !== null) {
      const prefix = match[1] ?? '';
      const username = match[2];
      const matchStart = match.index;
      const matchLength = match[0].length;
      if (matchStart > lastIndex) {
        parts.push({ type: 'text', value: input.slice(lastIndex, matchStart) });
      }

      if (prefix) {
        parts.push({ type: 'text', value: prefix });
      }

      parts.push({ type: 'mention', value: `@${username}`, username });
      lastIndex = matchStart + matchLength;
    }

    if (lastIndex < input.length) {
      parts.push({ type: 'text', value: input.slice(lastIndex) });
    }

    return parts;
  }

  function splitText(input: string): Part[] {
    if (!input) return [];

    const parts: Part[] = [];
    const regex = new RegExp(URL_REGEX);
    let lastIndex = 0;
    let match: RegExpExecArray | null;

    while ((match = regex.exec(input)) !== null) {
      const url = match[0];
      const start = match.index;

      if (start > lastIndex) {
        parts.push(...splitMentions(input.slice(lastIndex, start)));
      }

      parts.push({ type: 'link', value: url });
      lastIndex = start + url.length;
    }

    if (lastIndex < input.length) {
      parts.push(...splitMentions(input.slice(lastIndex)));
    }

    return parts;
  }

  function escapeRegExp(value: string): string {
    return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  }

  function buildHighlightParts(input: string, query: string): HighlightPart[] {
    const trimmed = query.trim();
    if (!trimmed) {
      return [{ text: input, isMatch: false }];
    }
    const tokens = trimmed.split(/\s+/).filter(Boolean).map(escapeRegExp);
    if (tokens.length === 0) {
      return [{ text: input, isMatch: false }];
    }
    const splitPattern = new RegExp(`(${tokens.join('|')})`, 'gi');
    const matchPattern = new RegExp(`^(${tokens.join('|')})$`, 'i');
    return input
      .split(splitPattern)
      .filter((part) => part.length > 0)
      .map((part) => ({
        text: part,
        isMatch: matchPattern.test(part),
      }));
  }

  $: parts = splitText(text);
</script>

<p class={className}>
  {#each parts as part, index (index)}
    {#if part.type === 'link'}
      <a href={part.value} target="_blank" rel="noopener noreferrer" class={linkClassName}>
        {part.value}
      </a>
    {:else if part.type === 'mention'}
      <a
        href={buildProfileHref(part.username)}
        class={mentionClassName}
        on:click={(event) => handleProfileNavigation(event, part.username)}
      >
        {part.value}
      </a>
    {:else}
      {#each buildHighlightParts(part.value, highlightQuery) as highlight, highlightIndex (highlightIndex)}
        {#if highlight.isMatch}
          <mark class="rounded bg-amber-100 px-0.5 text-gray-900 ring-1 ring-amber-200">
            {highlight.text}
          </mark>
        {:else}
          {highlight.text}
        {/if}
      {/each}
    {/if}
  {/each}
</p>
