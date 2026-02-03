import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import type { Post } from '../../stores/postStore';
import type { RecipeCategory, SavedRecipe } from '../../stores/recipeStore';
import { postStore } from '../../stores/postStore';
import { recipeStore } from '../../stores/recipeStore';

const pushPath = vi.fn();

vi.mock('../../services/routeNavigation', () => ({
  buildStandaloneThreadHref: (postId: string) => `/posts/${postId}`,
  pushPath,
}));

const { default: Cookbook } = await import('../recipes/Cookbook.svelte');

const categories: RecipeCategory[] = [
  {
    id: 'cat-1',
    userId: 'user-1',
    name: 'Favorites',
    position: 1,
    createdAt: '2025-01-01T00:00:00Z',
  },
];

const postOne: Post & {
  cookInfo: { avgRating: number; cookCount: number };
  saveInfo: { saveCount: number };
} = {
  id: 'post-1',
  userId: 'user-1',
  sectionId: 'section-1',
  content: 'Best pasta',
  createdAt: '2025-01-01T00:00:00Z',
  links: [
    {
      url: 'https://example.com/pasta',
      metadata: {
        title: 'Pasta Primavera',
        image: 'https://example.com/pasta.jpg',
        type: 'recipe',
        recipe: {
          name: 'Pasta Primavera',
          image: 'https://example.com/pasta.jpg',
        },
      },
    },
  ],
  user: {
    id: 'user-1',
    username: 'Sander',
  },
  cookInfo: { avgRating: 4.8, cookCount: 12 },
  saveInfo: { saveCount: 7 },
};

const postTwo: Post & {
  cookInfo: { avgRating: number; cookCount: number };
  saveInfo: { saveCount: number };
} = {
  id: 'post-2',
  userId: 'user-2',
  sectionId: 'section-1',
  content: 'Salad time',
  createdAt: '2025-01-03T00:00:00Z',
  links: [
    {
      url: 'https://example.com/salad',
      metadata: {
        title: 'Citrus Salad',
        image: 'https://example.com/salad.jpg',
        type: 'recipe',
        recipe: {
          name: 'Citrus Salad',
          image: 'https://example.com/salad.jpg',
        },
      },
    },
  ],
  user: {
    id: 'user-2',
    username: 'Casey',
  },
  cookInfo: { avgRating: 3.6, cookCount: 4 },
  saveInfo: { saveCount: 2 },
};

const savedRecipes = new Map<string, SavedRecipe[]>([
  [
    'Favorites',
    [
      {
        id: 'saved-1',
        userId: 'user-1',
        postId: 'post-1',
        category: 'Favorites',
        createdAt: '2025-01-02T00:00:00Z',
        post: postOne,
      },
    ],
  ],
]);

beforeEach(() => {
  postStore.reset();
  recipeStore.reset();
  recipeStore.setCategories(categories);
  recipeStore.setSavedRecipes(new Map(savedRecipes));
  postStore.setPosts([postOne, postTwo], null, false);

  vi.spyOn(recipeStore, 'loadCategories').mockResolvedValue();
  vi.spyOn(recipeStore, 'loadSavedRecipes').mockResolvedValue();
  vi.spyOn(recipeStore, 'loadCookLogs').mockResolvedValue();
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  pushPath.mockReset();
  postStore.reset();
  recipeStore.reset();
});

describe('Cookbook', () => {
  it('shows my recipes by default and navigates on click', async () => {
    render(Cookbook);

    const myTab = screen.getByTestId('cookbook-tab-my');
    expect(myTab).toHaveAttribute('aria-selected', 'true');

    const item = screen.getByTestId('my-recipe-item-post-1');
    await fireEvent.click(item);

    expect(pushPath).toHaveBeenCalledWith('/posts/post-1');
  });

  it('switches to all recipes and sorts by rating', async () => {
    render(Cookbook);

    await fireEvent.click(screen.getByTestId('cookbook-tab-all'));

    const items = screen.getAllByTestId(/all-recipe-item-/);
    expect(items[0]).toHaveTextContent('Pasta Primavera');

    await fireEvent.change(screen.getByTestId('cookbook-sort'), {
      target: { value: 'date' },
    });

    const reordered = screen.getAllByTestId(/all-recipe-item-/);
    expect(reordered[0]).toHaveTextContent('Citrus Salad');
  });

  it('shows empty state when no categories', () => {
    recipeStore.reset();

    render(Cookbook);

    expect(screen.getByText('Create your first category')).toBeInTheDocument();
    expect(screen.getByText('No recipes saved here yet')).toBeInTheDocument();
  });
});
