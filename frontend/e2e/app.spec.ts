import { test, expect, type Page } from '@playwright/test';

async function login(page: Page) {
  const email = process.env.E2E_EMAIL || 'admin@clubhouse.local';
  const password = process.env.E2E_PASSWORD || 'Admin123!';

  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Sign in to Clubhouse' })).toBeVisible();

  await page.getByLabel('Email address').fill(email);
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
}

const sectionName = process.env.E2E_SECTION || 'Music';
const podcastSectionName = process.env.E2E_PODCAST_SECTION || 'Podcasts';

async function openSection(page: Page, name: string) {
  const nav = page.getByRole('navigation', { name: 'Main navigation' });
  const sectionButton = nav.getByRole('button', { name });
  await expect(sectionButton).toBeVisible();
  await sectionButton.click();
  await expect(page.getByLabel('Post content')).toBeVisible();
}

async function addPostLink(page: Page, url: string) {
  await page.getByRole('button', { name: 'Add link' }).click();
  const linkInput = page.getByLabel('Link URL');
  await linkInput.fill(url);
  await linkInput.press('Enter');
}

test('unauthenticated users see login form', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Sign in to Clubhouse' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Sign in' })).toBeVisible();
  await expect(page.getByText('create a new account')).toBeVisible();
});

test('login flow (happy path)', async ({ page }) => {
  await login(page);
  await expect(page.getByLabel('Main navigation')).toBeVisible();
});

test('sections and feed render after login', async ({ page }) => {
  await login(page);
  await openSection(page, sectionName);
  await expect(page.getByRole('button', { name: 'Post' })).toBeVisible();
});

test('create text-only post', async ({ page }) => {
  await login(page);
  await openSection(page, sectionName);

  const content = `E2E post ${Date.now()}`;
  await page.getByLabel('Post content').fill(content);
  await page.getByRole('button', { name: 'Post' }).click();

  await expect(page.getByText(content)).toBeVisible();
});

test('create post with highlights', async ({ page }) => {
  await login(page);
  await openSection(page, sectionName);

  const content = `E2E highlight post ${Date.now()}`;
  await page.getByLabel('Post content').fill(content);

  await addPostLink(page, 'https://example.com');

  const timestampInput = page.getByLabel('Timestamp (mm:ss)');
  await expect(timestampInput).toBeVisible();
  await timestampInput.fill('01:15');
  await page.getByLabel('Label (optional)').fill('Intro');
  await page.getByRole('button', { name: 'Add highlight' }).click();

  await page.getByRole('button', { name: 'Post' }).click();

  await expect(page.getByText(content)).toBeVisible();
  await expect(page.getByText('01:15')).toBeVisible();
  await expect(page.getByText('Intro')).toBeVisible();
});

test('music link appears in the recent links container', async ({ page }) => {
  await login(page);
  await openSection(page, sectionName);

  const linkUrl = `https://example.com/music-${Date.now()}`;
  await addPostLink(page, linkUrl);

  await page.getByRole('button', { name: 'Post' }).click();

  await expect(page.getByRole('button', { name: /Recent Music Links/i })).toBeVisible();
  await expect(page.locator(`a[href=\"${linkUrl}\"]`)).toBeVisible();
});

test('podcast workflow: create show post, save and unsave, and switch top widget modes', async ({ page }) => {
  await login(page);
  await openSection(page, podcastSectionName);

  const showContent = `E2E podcast show ${Date.now()}`;
  const highlightTitle = `Highlight ${Date.now()}`;
  await page.getByLabel('Post content').fill(showContent);
  await addPostLink(page, `https://example.com/podcast-show-${Date.now()}`);
  await page.getByLabel('Podcast kind').selectOption('show');
  await page.getByLabel('Highlight episode title').fill(highlightTitle);
  await page.getByLabel('Highlight episode url').fill(`https://example.com/episodes/${Date.now()}`);
  await page.getByLabel('Highlight episode note').fill('Start here');
  await page.getByRole('button', { name: 'Add highlighted episode' }).click();
  await page.getByRole('button', { name: 'Post' }).click();

  const showCard = page.locator('article').filter({ hasText: showContent }).first();
  await expect(showCard).toBeVisible();
  await expect(showCard.getByTestId('podcast-kind-badge')).toContainText('Show');
  await expect(showCard.getByTestId('podcast-highlight-episodes')).toContainText(highlightTitle);

  const saveButton = showCard.getByRole('button', { name: 'Save podcast for later' });
  await expect(saveButton).toBeVisible();
  await saveButton.click();
  await expect(showCard.getByRole('button', { name: 'Remove podcast from saved for later' })).toBeVisible();

  if (sectionName !== podcastSectionName) {
    await openSection(page, sectionName);
    await openSection(page, podcastSectionName);
  }

  await expect(page.getByTestId('podcasts-top-container')).toBeVisible();
  await page.getByTestId('podcasts-mode-saved').click();
  await expect(page.getByTestId('podcasts-saved-list')).toContainText(showContent);

  const savedCardButton = page
    .getByTestId('podcasts-saved-list')
    .locator('button')
    .filter({ hasText: showContent })
    .first();
  await savedCardButton.click();

  const threadSaveButton = page.getByRole('button', { name: 'Remove podcast from saved for later' }).first();
  await expect(threadSaveButton).toBeVisible();
  await threadSaveButton.click();
  await expect(page.getByRole('button', { name: 'Save podcast for later' }).first()).toBeVisible();

  await page.getByTestId('podcasts-mode-recent').click();
  await expect(page.getByTestId('podcasts-recent-list')).toBeVisible();
});

test('podcast workflow: explicit episode post and uncertain kind validation flow', async ({ page }) => {
  await login(page);
  await openSection(page, podcastSectionName);

  const uncertainContent = `E2E uncertain podcast ${Date.now()}`;
  await page.getByLabel('Post content').fill(uncertainContent);
  await addPostLink(page, `https://example.com/listen-${Date.now()}`);
  await page.getByRole('button', { name: 'Post' }).click();

  const uncertainMessage =
    'Could not determine whether this podcast link is a show or an episode. Please select one and try again.';
  await expect(page.getByText(uncertainMessage)).toBeVisible();
  await expect(page.getByRole('button', { name: 'Post' })).toBeDisabled();

  await page.getByLabel('Podcast kind').selectOption('show');
  await expect(page.getByRole('button', { name: 'Post' })).toBeEnabled();
  await page.getByRole('button', { name: 'Post' }).click();
  await expect(page.getByText(uncertainContent)).toBeVisible();

  const episodeContent = `E2E podcast episode ${Date.now()}`;
  await page.getByLabel('Post content').fill(episodeContent);
  await addPostLink(page, 'https://open.spotify.com/episode/4rOoJ6Egrf8K2IrywzwOMk');
  await page.getByLabel('Podcast kind').selectOption('episode');
  await page.getByRole('button', { name: 'Post' }).click();

  const episodeCard = page.locator('article').filter({ hasText: episodeContent }).first();
  await expect(episodeCard).toBeVisible();
  await expect(episodeCard.getByTestId('podcast-kind-badge')).toContainText('Episode');
});
