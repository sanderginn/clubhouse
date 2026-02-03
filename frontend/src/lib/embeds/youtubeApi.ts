export type YouTubePlayer = {
  seekTo: (seconds: number, allowSeekAhead: boolean) => void;
  playVideo: () => void;
  destroy: () => void;
};

export type YouTubeApi = {
  Player: new (element: HTMLElement, options: { events?: { onReady?: () => void } }) => YouTubePlayer;
};

let apiPromise: Promise<YouTubeApi> | null = null;

const YOUTUBE_IFRAME_API_SRC = 'https://www.youtube.com/iframe_api';

export const loadYouTubeApi = (): Promise<YouTubeApi> => {
  if (apiPromise) return apiPromise;

  apiPromise = new Promise((resolve, reject) => {
    if (typeof window === 'undefined') {
      reject(new Error('YouTube API can only be loaded in the browser.'));
      return;
    }

    if (window.YT?.Player) {
      resolve(window.YT as YouTubeApi);
      return;
    }

    const existingScript = document.querySelector<HTMLScriptElement>(
      `script[data-youtube-iframe-api="true"]`
    );

    const previousHandler = window.onYouTubeIframeAPIReady;
    window.onYouTubeIframeAPIReady = () => {
      if (typeof previousHandler === 'function') {
        previousHandler();
      }
      if (window.YT?.Player) {
        resolve(window.YT as YouTubeApi);
      } else {
        reject(new Error('YouTube API failed to initialize.'));
      }
    };

    if (!existingScript) {
      const script = document.createElement('script');
      script.src = YOUTUBE_IFRAME_API_SRC;
      script.async = true;
      script.defer = true;
      script.dataset.youtubeIframeApi = 'true';
      script.onerror = () => reject(new Error('Failed to load YouTube API.'));
      document.head.appendChild(script);
    }
  });

  return apiPromise;
};
