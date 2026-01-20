import { writable, derived } from 'svelte/store';
import { api } from '../services/api';

export interface Section {
  id: string;
  name: string;
  type: SectionType;
  icon: string;
}

export type SectionType = 'music' | 'photo' | 'event' | 'recipe' | 'book' | 'movie' | 'general';

const sectionIcons: Record<SectionType, string> = {
  music: '🎵',
  photo: '📷',
  event: '📅',
  recipe: '🍳',
  book: '📚',
  movie: '🎬',
  general: '💬',
};

interface ApiSection {
  id: string;
  name: string;
  type: SectionType;
}

interface SectionState {
  sections: Section[];
  activeSection: Section | null;
  isLoading: boolean;
}

function createSectionStore() {
  const { subscribe, set, update } = writable<SectionState>({
    sections: [],
    activeSection: null,
    isLoading: false,
  });

  return {
    subscribe,
    setSections: (sections: Section[]) =>
      update((state) => ({
        ...state,
        sections: sections.map((s) => ({
          ...s,
          icon: sectionIcons[s.type] || '📁',
        })),
        activeSection:
          state.activeSection && sections.some((section) => section.id === state.activeSection?.id)
            ? state.activeSection
            : sections[0] ?? null,
        isLoading: false,
      })),
    setActiveSection: (section: Section | null) =>
      update((state) => ({ ...state, activeSection: section })),
    setLoading: (isLoading: boolean) => update((state) => ({ ...state, isLoading })),
    loadSections: async () => {
      update((state) => ({ ...state, isLoading: true }));
      try {
        const response = await api.get<{ sections: ApiSection[] }>('/sections');
        const sections =
          response.sections?.map((section) => ({
            id: section.id,
            name: section.name,
            type: section.type,
            icon: sectionIcons[section.type] || '📁',
          })) ?? [];
        update((state) => ({
          ...state,
          sections,
          activeSection: sections[0] ?? null,
          isLoading: false,
        }));
      } catch {
        update((state) => ({ ...state, isLoading: false }));
      }
    },
  };
}

export const sectionStore = createSectionStore();

export const sections = derived(sectionStore, ($sectionStore) => $sectionStore.sections);

export const activeSection = derived(sectionStore, ($sectionStore) => $sectionStore.activeSection);
