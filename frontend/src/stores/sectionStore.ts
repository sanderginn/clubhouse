import { writable, derived } from 'svelte/store';
import { api } from '../services/api';
import { slugifySectionName } from '../services/sectionSlug';

export interface Section {
  id: string;
  name: string;
  type: SectionType;
  icon: string;
  slug: string;
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

function isGeneralSection(section: { type?: SectionType; name?: string }): boolean {
  if (section.type) {
    return section.type === 'general';
  }
  return section.name?.toLowerCase() === 'general';
}

function orderSections<T extends { type?: SectionType; name?: string }>(sections: T[]): T[] {
  const general = sections.filter((section) => isGeneralSection(section));
  const rest = sections.filter((section) => !isGeneralSection(section));
  return [...general, ...rest];
}

function createSectionStore() {
  const { subscribe, update } = writable<SectionState>({
    sections: [],
    activeSection: null,
    isLoading: false,
  });

  return {
    subscribe,
    setSections: (sections: Section[]) =>
      update((state) => {
        const ordered = orderSections(sections);
        const mapped = ordered.map((section) => ({
          ...section,
          icon: sectionIcons[section.type] || '📁',
          slug: slugifySectionName(section.name) || section.id,
        }));
        let active = null;
        if (state.activeSection) {
          const match = mapped.find((section) => section.id === state.activeSection?.id);
          if (match) {
            active = match;
          }
        }
        if (!active) {
          active = mapped[0] ?? null;
        }
        return {
          ...state,
          sections: mapped,
          activeSection: active,
          isLoading: false,
        };
      }),
    setActiveSection: (section: Section | null) =>
      update((state) => ({ ...state, activeSection: section })),
    setLoading: (isLoading: boolean) => update((state) => ({ ...state, isLoading })),
    loadSections: async (preferredSectionId?: string | null) => {
      update((state) => ({ ...state, isLoading: true }));
      try {
        const response = await api.get<{ sections: ApiSection[] }>('/sections');
        const sections = orderSections(response.sections ?? []).map((section) => ({
          id: section.id,
          name: section.name,
          type: section.type,
          icon: sectionIcons[section.type] || '📁',
          slug: slugifySectionName(section.name) || section.id,
        }));
        const preferred =
          preferredSectionId && sections.length > 0
            ? sections.find((section) => section.id === preferredSectionId) ?? null
            : null;
        update((state) => ({
          ...state,
          sections,
          activeSection: preferred ?? sections[0] ?? null,
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
