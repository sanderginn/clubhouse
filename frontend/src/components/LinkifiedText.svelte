<script lang="ts">
  export let text = '';
  export let className = '';
  export let linkClassName = 'text-blue-600 hover:text-blue-800 underline';

  type TextPart = { type: 'text'; value: string };
  type LinkPart = { type: 'link'; value: string };
  type Part = TextPart | LinkPart;

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

  $: parts = splitText(text);
</script>

<p class={className}>
  {#each parts as part, index (index)}
    {#if part.type === 'link'}
      <a href={part.value} target="_blank" rel="noopener noreferrer" class={linkClassName}>
        {part.value}
      </a>
    {:else}
      {part.value}
    {/if}
  {/each}
</p>
