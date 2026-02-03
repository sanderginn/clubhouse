export type EmbedProvider = 'youtube' | 'soundcloud' | 'spotify' | 'bandcamp' | 'unknown';

export type EmbedController = {
  provider: EmbedProvider;
  supportsSeeking: boolean;
  seekTo: (seconds: number) => Promise<void>;
};
