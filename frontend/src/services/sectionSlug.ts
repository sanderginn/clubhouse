import type { Section } from '../stores/sectionStore';

const NON_SLUG_CHARS = /[^a-z0-9]+/g;
const TRIM_HYPHENS = /^-+|-+$/g;

export function slugifySectionName(name: string): string {
  return name
    .trim()
    .toLowerCase()
    .replace(NON_SLUG_CHARS, '-')
    .replace(TRIM_HYPHENS, '');
}

export function getSectionSlug(section: Pick<Section, 'name' | 'slug'>): string {
  return section.slug || slugifySectionName(section.name);
}

export function findSectionByIdentifier(
  sections: Section[],
  identifier: string
): Section | null {
  const trimmed = identifier.trim();
  if (!trimmed) return null;
  const normalized = trimmed.toLowerCase();
  return (
    sections.find((section) => section.slug === normalized) ??
    sections.find((section) => section.id === trimmed) ??
    null
  );
}

export function getSectionSlugById(sections: Section[], sectionId: string): string | null {
  const match = sections.find((section) => section.id === sectionId);
  return match ? getSectionSlug(match) : null;
}
