<script lang="ts">
  export let post: { id?: string } | undefined;
  export const highlightCommentIds: string[] = [];
</script>

<div data-testid="post-card">{post?.id}</div>
