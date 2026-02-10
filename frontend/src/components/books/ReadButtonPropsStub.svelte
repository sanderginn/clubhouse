<script lang="ts">
  import type { BookStats } from '../../stores/postStore';

  export let postId: string;
  export let bookStats: BookStats;
</script>

<div
  data-testid="read-button-stub"
  data-post-id={postId}
  data-viewer-read={String(bookStats?.viewerRead ?? false)}
  data-viewer-rating={bookStats?.viewerRating == null ? '' : String(bookStats.viewerRating)}
  data-read-count={String(bookStats?.readCount ?? 0)}
></div>
