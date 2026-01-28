<script lang="ts">
  export let post: { id?: string } | undefined;
</script>

<div data-testid="post-card">{post?.id}</div>
