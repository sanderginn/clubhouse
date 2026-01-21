import '@testing-library/jest-dom/vitest';
import { vi } from 'vitest';

if (!window.matchMedia) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => {
    let listeners: Array<(event: MediaQueryListEvent) => void> = [];
    const mediaQueryList: { matches: boolean } & Omit<MediaQueryList, 'matches'> = {
      matches: false,
      media: query,
      onchange: null,
      addEventListener: (_event: string, handler: EventListenerOrEventListenerObject | null) => {
        listeners.push(handler as (event: MediaQueryListEvent) => void);
      },
      removeEventListener: (_event: string, handler: EventListenerOrEventListenerObject | null) => {
        listeners = listeners.filter((item) => item !== handler);
      },
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => true,
    };
    (mediaQueryList as unknown as { trigger: (matches: boolean) => void }).trigger = (matches: boolean) => {
      mediaQueryList.matches = matches;
      listeners.forEach((handler) => handler({ matches } as MediaQueryListEvent));
    };
    return mediaQueryList as MediaQueryList;
  }) as unknown as typeof window.matchMedia;
}

if (!('IntersectionObserver' in window)) {
  class MockIntersectionObserver {
    callback: IntersectionObserverCallback;
    elements: Element[] = [];

    constructor(callback: IntersectionObserverCallback) {
      this.callback = callback;
      (globalThis as { __lastObserver?: MockIntersectionObserver }).__lastObserver = this;
    }

    observe = (element: Element) => {
      this.elements.push(element);
    };

    unobserve = (element: Element) => {
      this.elements = this.elements.filter((item) => item !== element);
    };

    disconnect = () => {
      this.elements = [];
    };

    trigger = (isIntersecting: boolean) => {
      const entries = this.elements.map((target) => ({
        isIntersecting,
        target,
        intersectionRatio: isIntersecting ? 1 : 0,
        boundingClientRect: target.getBoundingClientRect(),
        intersectionRect: target.getBoundingClientRect(),
        rootBounds: null,
        time: Date.now(),
      }));
      this.callback(entries as IntersectionObserverEntry[], this as unknown as IntersectionObserver);
    };
  }

  (globalThis as { IntersectionObserver?: typeof IntersectionObserver }).IntersectionObserver =
    MockIntersectionObserver as unknown as typeof IntersectionObserver;
}
