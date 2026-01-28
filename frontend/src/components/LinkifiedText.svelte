<script lang="ts">
  export let text = '';
  export let className = '';
  export let linkClassName = 'text-blue-600 hover:text-blue-800 underline';
  export let highlightQuery = '';

  type TextPart = { type: 'text'; value: string };
  type LinkPart = { type: 'link'; value: string };
  type Part = TextPart | LinkPart;
  type HighlightPart = { text: string; isMatch: boolean };

  const URL_REGEX = /https?:\/\/[^\s<>"{}|\\^`[\]]+/gi;

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
        parts.push({ type: 'text', value: input.slice(lastIndex, start) });
      }

      parts.push({ type: 'link', value: url });
      lastIndex = start + url.length;
    }

    if (lastIndex < input.length) {
      parts.push({ type: 'text', value: input.slice(lastIndex) });
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
