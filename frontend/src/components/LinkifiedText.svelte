<script lang="ts" context="module">
  const mentionValidationCache = new Map<string, boolean>();
  const mentionValidationInflight = new Map<string, Promise<void>>();
</script>

<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../services/api';
  import { buildProfileHref, handleProfileNavigation } from '../services/profileNavigation';

  export let text = '';
  export let className = '';
  export let linkClassName = 'text-blue-600 hover:text-blue-800 underline';
  export let mentionClassName =
    'text-indigo-600 hover:text-indigo-800 font-medium bg-indigo-50 rounded px-[1px]';
  export let highlightQuery = '';
  export let validMentions: string[] | null = null;
  export let validateMentions = true;

  type TextPart = { type: 'text'; value: string };
  type LinkPart = { type: 'link'; value: string };
  type MentionPart = { type: 'mention'; value: string; username: string };
  type Part = TextPart | LinkPart | MentionPart;
  type HighlightPart = { text: string; isMatch: boolean };

  const URL_REGEX = /https?:\/\/[^\s<>"{}|\\^`[\]]+/gi;

  function isUsernameChar(char: string): boolean {
    return /[A-Za-z0-9_]/.test(char);
  }

  function collectMentionUsernames(input: string): string[] {
    if (!input) return [];

    const usernames = new Set<string>();
    const regex = new RegExp(URL_REGEX);
    let lastIndex = 0;
    let match: RegExpExecArray | null;

    const collectFromSegment = (segment: string) => {
      for (let i = 0; i < segment.length; i += 1) {
        const char = segment[i];

        if (char === '\\' && segment[i + 1] === '@') {
          i += 1;
          continue;
        }

        if (char !== '@') {
          continue;
        }

        if (i > 0 && isUsernameChar(segment[i - 1])) {
          continue;
        }

        let end = i + 1;
        while (end < segment.length && isUsernameChar(segment[end])) {
          end += 1;
        }

        const username = segment.slice(i + 1, end);
        if (username.length < 3 || username.length > 50) {
          i = end - 1;
          continue;
        }

        usernames.add(username);
        i = end - 1;
      }
    };

    while ((match = regex.exec(input)) !== null) {
      const start = match.index;
      if (start > lastIndex) {
        collectFromSegment(input.slice(lastIndex, start));
      }
      lastIndex = start + match[0].length;
    }

    if (lastIndex < input.length) {
      collectFromSegment(input.slice(lastIndex));
    }

    return [...usernames];
  }

  async function ensureMentionValidation(usernames: string[]): Promise<void> {
    const tasks = usernames
      .filter((username) => !mentionValidationCache.has(username))
      .map((username) => {
        const existing = mentionValidationInflight.get(username);
        if (existing) {
          return existing;
        }
        const task = api
          .lookupUserByUsername(username)
          .then(() => {
            mentionValidationCache.set(username, true);
          })
          .catch(() => {
            mentionValidationCache.set(username, false);
          })
          .finally(() => {
            mentionValidationInflight.delete(username);
          });
        mentionValidationInflight.set(username, task);
        return task;
      });

    if (tasks.length === 0) {
      return;
    }

    await Promise.all(tasks);
    validationTick += 1;
  }

  function shouldLinkMention(username: string, allowList: Set<string> | null): boolean {
    if (allowList) {
      return allowList.has(username);
    }
    if (!validateMentions) {
      return true;
    }
    return mentionValidationCache.get(username) === true;
  }

  function splitMentions(input: string, allowList: Set<string> | null): Part[] {
    if (!input) return [];

    const parts: Part[] = [];
    let buffer = '';

    const flushBuffer = () => {
      if (buffer.length > 0) {
        parts.push({ type: 'text', value: buffer });
        buffer = '';
      }
    };

    for (let i = 0; i < input.length; i += 1) {
      const char = input[i];

      if (char === '\\' && input[i + 1] === '@') {
        buffer += '@';
        i += 1;
        continue;
      }

      if (char !== '@') {
        buffer += char;
        continue;
      }

      if (i > 0 && isUsernameChar(input[i - 1])) {
        buffer += char;
        continue;
      }

      let end = i + 1;
      while (end < input.length && isUsernameChar(input[end])) {
        end += 1;
      }

      const username = input.slice(i + 1, end);
      if (username.length < 3 || username.length > 50) {
        buffer += char;
        continue;
      }

      if (shouldLinkMention(username, allowList)) {
        flushBuffer();
        parts.push({ type: 'mention', value: `@${username}`, username });
      } else {
        buffer += `@${username}`;
      }

      i = end - 1;
    }

    flushBuffer();
    return parts;
  }

  function splitText(input: string, allowList: Set<string> | null, _tick = 0): Part[] {
    if (!input) return [];

    const parts: Part[] = [];
    const regex = new RegExp(URL_REGEX);
    let lastIndex = 0;
    let match: RegExpExecArray | null;

    while ((match = regex.exec(input)) !== null) {
      const url = match[0];
      const start = match.index;

      if (start > lastIndex) {
        parts.push(...splitMentions(input.slice(lastIndex, start), allowList));
      }

      parts.push({ type: 'link', value: url });
      lastIndex = start + url.length;
    }

    if (lastIndex < input.length) {
      parts.push(...splitMentions(input.slice(lastIndex), allowList));
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

  let isMounted = false;
  let validationTick = 0;
  let mentionAllowList: Set<string> | null = null;

  $: mentionAllowList = validMentions ? new Set(validMentions) : null;

  $: if (isMounted && validateMentions && !mentionAllowList) {
    const candidates = collectMentionUsernames(text);
    if (candidates.length > 0) {
      void ensureMentionValidation(candidates);
    }
  }

  $: parts = splitText(text, mentionAllowList, validationTick);

  onMount(() => {
    isMounted = true;
    if (validateMentions && !mentionAllowList) {
      const candidates = collectMentionUsernames(text);
      if (candidates.length > 0) {
        void ensureMentionValidation(candidates);
      }
    }
  });
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
          <mark class="rounded bg-amber-100 px-[1px] text-gray-900 ring-1 ring-amber-200">
            {highlight.text}
          </mark>
        {:else}
          {highlight.text}
        {/if}
      {/each}
    {/if}
  {/each}
</p>
