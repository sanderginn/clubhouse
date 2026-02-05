/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_SENTRY_DSN?: string;
  readonly VITE_APP_VERSION?: string;
  readonly VITE_OTEL_EXPORTER_OTLP_ENDPOINT?: string;
  readonly VITE_OTEL_SERVICE_NAME?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

type SoundCloudWidget = {
  seekTo: (milliseconds: number) => void;
  play: () => void;
  bind: (event: string, handler: () => void) => void;
  unbind: (event: string) => void;
};

interface Window {
  SC?: {
    Widget: (element: HTMLIFrameElement) => SoundCloudWidget;
  };
}
