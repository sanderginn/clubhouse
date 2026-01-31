<script lang="ts">
  import { createEventDispatcher, tick } from 'svelte';
  import { api, type ApiUserSummary } from '../../services/api';

  export let value = '';
  export let id = '';
  export let rows = 3;
  export let placeholder = '';
  export let disabled = false;
  export let className = '';
  export let ariaLabel = '';
  export let mentionClassName =
    'text-indigo-600 hover:text-indigo-800 font-medium bg-indigo-50 rounded px-[1px]';

  const dispatch = createEventDispatcher<{
    input: Event;
    keydown: KeyboardEvent;
    focus: FocusEvent;
    blur: FocusEvent;
  }>();

  let textarea: HTMLTextAreaElement | null = null;
  let showSuggestions = false;
  let suggestions: ApiUserSummary[] = [];
  let highlightedIndex = 0;
  let mentionStart = -1;
  let mentionQuery = '';
  let isLoading = false;
  let requestToken = 0;
  let debounceId: ReturnType<typeof setTimeout> | null = null;

  function isUsernameChar(char: string): boolean {
    return /[A-Za-z0-9_]/.test(char);
  }

  function findMentionContext(text: string, cursor: number): { start: number; query: string } | null {
    if (cursor <= 0) return null;

    const beforeCursor = text.slice(0, cursor);
    const segment = beforeCursor.split(/\s/).pop() ?? '';
    const atIndex = segment.lastIndexOf('@');

    if (atIndex === -1) return null;
    if (atIndex > 0 && segment[atIndex - 1] === '\\') return null;
    if (atIndex > 0 && isUsernameChar(segment[atIndex - 1])) return null;

    const query = segment.slice(atIndex + 1);
    if (!/^[A-Za-z0-9_]*$/.test(query)) return null;

    const segmentStart = beforeCursor.length - segment.length;
    const start = segmentStart + atIndex;

    return { start, query };
  }

  function clearSuggestions() {
    showSuggestions = false;
    suggestions = [];
    highlightedIndex = 0;
    mentionStart = -1;
    mentionQuery = '';
  }

  async function fetchSuggestions(query: string) {
    const token = (requestToken += 1);
    isLoading = true;
    try {
      const response = await api.searchUsers(query, 8);
      if (token !== requestToken) return;
      suggestions = response.users ?? [];
      highlightedIndex = 0;
    } catch {
      if (token !== requestToken) return;
      suggestions = [];
      highlightedIndex = 0;
    } finally {
      if (token === requestToken) {
        isLoading = false;
      }
    }
  }

  function queueSuggestions(query: string) {
    if (debounceId) {
      clearTimeout(debounceId);
    }
    debounceId = setTimeout(() => {
      void fetchSuggestions(query);
    }, 150);
  }

  function updateMentionContext() {
    if (!textarea || disabled) {
      clearSuggestions();
      return;
    }
    const cursor = textarea.selectionStart ?? 0;
    const context = findMentionContext(value, cursor);
    if (!context) {
      clearSuggestions();
      return;
    }

    mentionStart = context.start;
    mentionQuery = context.query;
    showSuggestions = true;
    queueSuggestions(mentionQuery);
  }

  function selectSuggestion(user: ApiUserSummary) {
    if (!textarea || mentionStart < 0) {
      return;
    }
    const cursor = textarea.selectionStart ?? value.length;
    const before = value.slice(0, mentionStart + 1);
    const after = value.slice(cursor);
    const spacer = after.startsWith(' ') || after.startsWith('\n') || after.length === 0 ? '' : ' ';
    value = `${before}${user.username}${spacer}${after}`;
    showSuggestions = false;
    suggestions = [];
    highlightedIndex = 0;

    void tick().then(() => {
      if (!textarea) return;
      const nextCursor = before.length + user.username.length + spacer.length;
      textarea.focus();
      textarea.setSelectionRange(nextCursor, nextCursor);
      dispatch('input', new Event('input'));
    });
  }

  function handleInput(event: Event) {
    dispatch('input', event);
    updateMentionContext();
  }

  function handleKeydown(event: KeyboardEvent) {
    if (showSuggestions) {
      if (event.key === 'ArrowDown') {
        event.preventDefault();
        if (suggestions.length > 0) {
          highlightedIndex = (highlightedIndex + 1) % suggestions.length;
        }
      } else if (event.key === 'ArrowUp') {
        event.preventDefault();
        if (suggestions.length > 0) {
          highlightedIndex = (highlightedIndex - 1 + suggestions.length) % suggestions.length;
        }
      } else if (event.key === 'Enter' || event.key === 'Tab') {
        if (suggestions.length > 0) {
          event.preventDefault();
          selectSuggestion(suggestions[highlightedIndex]);
        }
      } else if (event.key === 'Escape') {
        event.preventDefault();
        clearSuggestions();
      }
    }
    dispatch('keydown', event);
  }

  function handleFocus(event: FocusEvent) {
    dispatch('focus', event);
  }

  function handleBlur(event: FocusEvent) {
    dispatch('blur', event);
    clearSuggestions();
  }

  function handleClick() {
    updateMentionContext();
  }
</script>

<div class="relative">
  <textarea
    bind:this={textarea}
    bind:value
    id={id}
    rows={rows}
    placeholder={placeholder}
    disabled={disabled}
    class={className}
    aria-label={ariaLabel}
    on:input={handleInput}
    on:keydown={handleKeydown}
    on:keyup={updateMentionContext}
    on:click={handleClick}
    on:focus={handleFocus}
    on:blur={handleBlur}
  ></textarea>

  {#if showSuggestions}
    <div
      class="absolute z-20 mt-1 w-full rounded-lg border border-gray-200 bg-white shadow-lg"
      role="listbox"
      aria-label="Mention suggestions"
    >
      {#if isLoading}
        <div class="px-3 py-2 text-sm text-gray-500">Loading users...</div>
      {:else if suggestions.length === 0}
        <div class="px-3 py-2 text-sm text-gray-500">No matches found.</div>
      {:else}
        {#each suggestions as user, index (user.id)}
          <button
            type="button"
            class={`flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-gray-50 ${
              index === highlightedIndex ? 'bg-gray-50' : ''
            }`}
            role="option"
            aria-selected={index === highlightedIndex}
            on:mousedown|preventDefault={() => selectSuggestion(user)}
          >
            {#if user.profile_picture_url}
              <img
                src={user.profile_picture_url}
                alt={user.username}
                class="h-6 w-6 rounded-full object-cover"
              />
            {:else}
              <div class="h-6 w-6 rounded-full bg-gray-200 flex items-center justify-center">
                <span class="text-xs text-gray-500 font-medium">
                  {user.username.charAt(0).toUpperCase()}
                </span>
              </div>
            {/if}
            <span class={mentionClassName}>@{user.username}</span>
          </button>
        {/each}
      {/if}
    </div>
  {/if}
</div>
