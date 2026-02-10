<script lang="ts">
  import type { BookStats } from '../../stores/postStore';

  export let postId: string;
  export let bookStats: BookStats;
</script>

<div
  data-testid="bookshelf-save-button-stub"
  data-post-id={postId}
  data-viewer-on-bookshelf={String(bookStats?.viewerOnBookshelf ?? false)}
  data-categories={(bookStats?.viewerCategories ?? []).join(',')}
  data-bookshelf-count={String(bookStats?.bookshelfCount ?? 0)}
></div>
