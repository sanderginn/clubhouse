let apiPromise: Promise<NonNullable<Window['SC']>> | null = null;

const SOUNDCLOUD_WIDGET_API_SRC = 'https://w.soundcloud.com/player/api.js';

export const loadSoundCloudApi = (): Promise<NonNullable<Window['SC']>> => {
  if (apiPromise) return apiPromise;

  apiPromise = new Promise((resolve, reject) => {
    if (typeof window === 'undefined') {
      reject(new Error('SoundCloud API can only be loaded in the browser.'));
      return;
    }

    if (window.SC?.Widget) {
      resolve(window.SC);
      return;
    }

    const existingScript = document.querySelector<HTMLScriptElement>(
      `script[data-soundcloud-widget-api="true"]`
    );

    if (existingScript) {
      existingScript.addEventListener('load', () => resolve(window.SC as NonNullable<Window['SC']>));
      existingScript.addEventListener('error', () => reject(new Error('Failed to load SoundCloud API.')));
      return;
    }

    const script = document.createElement('script');
    script.src = SOUNDCLOUD_WIDGET_API_SRC;
    script.async = true;
    script.defer = true;
    script.dataset.soundcloudWidgetApi = 'true';
    script.onload = () => resolve(window.SC as NonNullable<Window['SC']>);
    script.onerror = () => reject(new Error('Failed to load SoundCloud API.'));
    document.head.appendChild(script);
  });

  return apiPromise;
};
