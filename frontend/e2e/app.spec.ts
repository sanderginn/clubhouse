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
  const nav = page.getByRole('navigation', { name: 'Main navigation' });
  const sectionButton = nav.getByRole('button', { name: sectionName });
  await expect(sectionButton).toBeVisible();
  await sectionButton.click();
  await expect(page.getByLabel('Post content')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Post' })).toBeVisible();
});

test('create text-only post', async ({ page }) => {
  await login(page);
  const nav = page.getByRole('navigation', { name: 'Main navigation' });
  const sectionButton = nav.getByRole('button', { name: sectionName });
  await expect(sectionButton).toBeVisible();
  await sectionButton.click();

  const content = `E2E post ${Date.now()}`;
  await page.getByLabel('Post content').fill(content);
  await page.getByRole('button', { name: 'Post' }).click();

  await expect(page.getByText(content)).toBeVisible();
});
