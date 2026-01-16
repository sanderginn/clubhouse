<script lang="ts">
  import './styles/globals.css';
  import { Layout } from './components';
  import { activeSection } from './stores';
</script>

<Layout>
  <div class="space-y-6">
    {#if $activeSection}
      <div class="flex items-center gap-3">
        <span class="text-3xl">{$activeSection.icon}</span>
        <h1 class="text-2xl font-bold text-gray-900">{$activeSection.name}</h1>
      </div>

      <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <p class="text-gray-600">
          Welcome to the {$activeSection.name} section. Posts will appear here.
        </p>
      </div>
    {:else}
      <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <h1 class="text-2xl font-bold text-gray-900 mb-4">Welcome to Clubhouse</h1>
        <p class="text-gray-600">
          Select a section from the sidebar to get started.
        </p>
      </div>
    {/if}
  </div>
</Layout>
