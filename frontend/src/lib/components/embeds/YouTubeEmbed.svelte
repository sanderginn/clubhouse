<script lang="ts">
  export let embedUrl: string;
  export let title: string = 'YouTube video';
</script>

<div class="relative w-full overflow-hidden rounded-lg bg-black" style="padding-top: 56.25%;">
  <iframe
    class="absolute inset-0 h-full w-full"
    src={embedUrl}
    title={title}
    allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
    allowfullscreen
    referrerpolicy="strict-origin-when-cross-origin"
    data-testid="youtube-embed-frame"
  ></iframe>
</div>
