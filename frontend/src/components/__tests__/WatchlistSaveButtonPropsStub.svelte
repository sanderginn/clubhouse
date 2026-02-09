<script lang="ts">
  export let postId = '';
  export let initialSaved = false;
  export let initialCategories: string[] = [];
  export let saveCount = 0;
</script>

<div
  data-testid="watchlist-save-button-stub"
  data-post-id={postId}
  data-initial-saved={initialSaved}
  data-categories={initialCategories.join(',')}
  data-save-count={saveCount}
>
  WatchlistSaveButton
</div>
