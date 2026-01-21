import { writable, derived, get } from 'svelte/store';
import { api } from '../services/api';
import { activeSection } from './sectionStore';
import type { Post, Link } from './postStore';

export type SearchScope = 'section' | 'global';

export interface CommentResult {
  id: string;
  postId: string;
  content: string;
  user?: {
    id: string;
    username: string;
    profilePictureUrl?: string;
  };
  links?: Link[];
  reactionCounts?: Record<string, number>;
  viewerReactions?: string[];
  createdAt: string;
}

export interface SearchResult {
  type: 'post' | 'comment';
  score: number;
  post?: Post;
  comment?: CommentResult;
}

interface ApiUser {
  id: string;
  username: string;
  profile_picture_url?: string;
}

interface ApiLink {
  id?: string;
  url: string;
  metadata?: Record<string, unknown>;
}

interface ApiPost {
  id: string;
  user_id: string;
  section_id: string;
  content: string;
  links?: ApiLink[];
  user?: ApiUser;
  reaction_counts?: Record<string, number>;
  viewer_reactions?: string[];
  comment_count?: number;
  created_at: string;
  updated_at?: string;
}

interface ApiComment {
  id: string;
  user_id: string;
  post_id: string;
  content: string;
  links?: ApiLink[];
  user?: ApiUser;
  reaction_counts?: Record<string, number>;
  viewer_reactions?: string[];
  created_at: string;
  updated_at?: string;
}

interface ApiSearchResult {
  type: 'post' | 'comment';
  score: number;
  post?: ApiPost;
  comment?: ApiComment;
}

interface SearchResponse {
  results: ApiSearchResult[];
}

interface SearchState {
  query: string;
  scope: SearchScope;
  results: SearchResult[];
  isLoading: boolean;
  error: string | null;
  lastSearched: string;
}

function mapApiLink(link: ApiLink): Link {
  const metadata = link.metadata ?? {};
  const hasMetadata = Object.keys(metadata).length > 0;

  return {
    id: link.id,
    url: link.url,
    metadata: hasMetadata
      ? {
          url: (metadata.url as string) ?? link.url,
          provider: metadata.provider as string | undefined,
          title: metadata.title as string | undefined,
          description: metadata.description as string | undefined,
          image: metadata.image as string | undefined,
          author: metadata.author as string | undefined,
          duration: metadata.duration as number | undefined,
          embedUrl: metadata.embedUrl as string | undefined,
        }
      : undefined,
  };
}

function mapApiPost(apiPost: ApiPost): Post {
  return {
    id: apiPost.id,
    userId: apiPost.user_id,
    sectionId: apiPost.section_id,
    content: apiPost.content,
    links: apiPost.links?.map(mapApiLink),
    user: apiPost.user
      ? {
          id: apiPost.user.id,
          username: apiPost.user.username,
          profilePictureUrl: apiPost.user.profile_picture_url,
        }
      : undefined,
    reactionCounts: apiPost.reaction_counts,
    viewerReactions: apiPost.viewer_reactions,
    commentCount: apiPost.comment_count,
    createdAt: apiPost.created_at,
    updatedAt: apiPost.updated_at,
  };
}

function mapApiComment(apiComment: ApiComment): CommentResult {
  return {
    id: apiComment.id,
    postId: apiComment.post_id,
    content: apiComment.content,
    links: apiComment.links?.map(mapApiLink),
    user: apiComment.user
      ? {
          id: apiComment.user.id,
          username: apiComment.user.username,
          profilePictureUrl: apiComment.user.profile_picture_url,
        }
      : undefined,
    reactionCounts: apiComment.reaction_counts,
    viewerReactions: apiComment.viewer_reactions,
    createdAt: apiComment.created_at,
  };
}

function createSearchStore() {
  const store = writable<SearchState>({
    query: '',
    scope: 'section',
    results: [],
    isLoading: false,
    error: null,
    lastSearched: '',
  });
  const { subscribe, update, set } = store;

  return {
    subscribe,
    setQuery: (query: string) =>
      update((state) => ({
        ...state,
        query,
      })),
    setScope: (scope: SearchScope) =>
      update((state) => ({
        ...state,
        scope,
      })),
    clear: () =>
      set({
        query: '',
        scope: 'section',
        results: [],
        isLoading: false,
        error: null,
        lastSearched: '',
      }),
    search: async () => {
      const state = get(store);
      const query = state.query.trim();
      if (!query) {
        update((prev) => ({
          ...prev,
          results: [],
          error: null,
          isLoading: false,
          lastSearched: '',
        }));
        return;
      }

      const scope = state.scope;
      const section = get(activeSection);
      if (scope === 'section' && !section) {
        update((prev) => ({
          ...prev,
          error: 'Select a section to search within.',
        }));
        return;
      }

      update((prev) => ({
        ...prev,
        isLoading: true,
        error: null,
      }));

      try {
        const params = new URLSearchParams({
          q: query,
          scope,
          limit: '20',
        });
        if (scope === 'section' && section) {
          params.set('section_id', section.id);
        }

        const response = await api.get<SearchResponse>(`/search?${params.toString()}`);

        const results = (response.results || []).map((result) => {
          if (result.type === 'post' && result.post) {
            return {
              type: 'post',
              score: result.score,
              post: mapApiPost(result.post),
            } as SearchResult;
          }
          if (result.type === 'comment' && result.comment) {
            return {
              type: 'comment',
              score: result.score,
              comment: mapApiComment(result.comment),
            } as SearchResult;
          }
          return null;
        }).filter((result): result is SearchResult => result !== null);

        update((prev) => ({
          ...prev,
          results,
          isLoading: false,
          error: null,
          lastSearched: query,
        }));
      } catch (err) {
        update((prev) => ({
          ...prev,
          isLoading: false,
          error: err instanceof Error ? err.message : 'Search failed',
        }));
      }
    },
  };
}

export const searchStore = createSearchStore();

export const searchQuery = derived(searchStore, ($searchStore) => $searchStore.query);
export const searchScope = derived(searchStore, ($searchStore) => $searchStore.scope);
export const searchResults = derived(searchStore, ($searchStore) => $searchStore.results);
export const searchError = derived(searchStore, ($searchStore) => $searchStore.error);
export const isSearching = derived(searchStore, ($searchStore) => $searchStore.isLoading);
export const lastSearchQuery = derived(searchStore, ($searchStore) => $searchStore.lastSearched);
