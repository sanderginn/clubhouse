<script lang="ts">
  import ErrorBoundary from '../ErrorBoundary.svelte';

  export let title = 'Oops';
  export let message = 'Try again';
</script>

<ErrorBoundary {title} {message}>
  <div>App Content</div>
</ErrorBoundary>
