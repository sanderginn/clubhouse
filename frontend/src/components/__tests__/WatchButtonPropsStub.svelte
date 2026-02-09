<script lang="ts">
  export let postId = '';
  export let initialWatched = false;
  export let initialRating: number | null = null;
  export let watchCount = 0;
</script>

<div
  data-testid="watch-button-stub"
  data-post-id={postId}
  data-initial-watched={initialWatched}
  data-initial-rating={initialRating}
  data-watch-count={watchCount}
>
  WatchButton
</div>
