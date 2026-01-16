import { writable, derived } from 'svelte/store';

export interface Section {
  id: string;
  name: string;
  type: SectionType;
  icon: string;
}

export type SectionType =
  | 'music'
  | 'photo'
  | 'event'
  | 'recipe'
  | 'book'
  | 'movie'
  | 'general';

const sectionIcons: Record<SectionType, string> = {
  music: '🎵',
  photo: '📷',
  event: '📅',
  recipe: '🍳',
  book: '📚',
  movie: '🎬',
  general: '💬',
};

const defaultSections: Section[] = [
  { id: '1', name: 'Music', type: 'music', icon: sectionIcons.music },
  { id: '2', name: 'Photos', type: 'photo', icon: sectionIcons.photo },
  { id: '3', name: 'Events', type: 'event', icon: sectionIcons.event },
  { id: '4', name: 'Recipes', type: 'recipe', icon: sectionIcons.recipe },
  { id: '5', name: 'Books', type: 'book', icon: sectionIcons.book },
  { id: '6', name: 'Movies', type: 'movie', icon: sectionIcons.movie },
  { id: '7', name: 'General', type: 'general', icon: sectionIcons.general },
];

interface SectionState {
  sections: Section[];
  activeSection: Section | null;
  isLoading: boolean;
}

function createSectionStore() {
  const { subscribe, set, update } = writable<SectionState>({
    sections: defaultSections,
    activeSection: defaultSections[0],
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
        isLoading: false,
      })),
    setActiveSection: (section: Section | null) =>
      update((state) => ({ ...state, activeSection: section })),
    setLoading: (isLoading: boolean) =>
      update((state) => ({ ...state, isLoading })),
  };
}

export const sectionStore = createSectionStore();

export const sections = derived(
  sectionStore,
  ($sectionStore) => $sectionStore.sections
);

export const activeSection = derived(
  sectionStore,
  ($sectionStore) => $sectionStore.activeSection
);
