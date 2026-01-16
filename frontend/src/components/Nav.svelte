<script lang="ts">
  import { sections, activeSection, sectionStore } from '../stores';
  import type { Section } from '../stores/sectionStore';

  function handleSectionClick(section: Section) {
    sectionStore.setActiveSection(section);
  }
</script>

<nav class="flex flex-col h-full" aria-label="Main navigation">
  <div class="flex-1 overflow-y-auto py-4">
    <div class="px-3 mb-2">
      <h2 class="text-xs font-semibold text-gray-500 uppercase tracking-wider">Sections</h2>
    </div>
    <ul class="space-y-1 px-2">
      {#each $sections as section (section.id)}
        <li>
          <button
            on:click={() => handleSectionClick(section)}
            class="w-full flex items-center gap-3 px-3 py-2 text-sm font-medium rounded-lg transition-colors
              {$activeSection?.id === section.id
              ? 'bg-primary text-white'
              : 'text-gray-700 hover:bg-gray-100'}"
            aria-current={$activeSection?.id === section.id ? 'page' : undefined}
          >
            <span class="text-lg" aria-hidden="true">{section.icon}</span>
            <span>{section.name}</span>
          </button>
        </li>
      {/each}
    </ul>
  </div>
</nav>
