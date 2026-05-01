declare global {
  interface Window {
    dataLayer?: unknown[];
    gtag?: (...args: unknown[]) => void;
  }
}

if (typeof window !== 'undefined' && typeof window.gtag !== 'function') {
  window.dataLayer = window.dataLayer ?? [];
  // Docusaurus route tracking assumes window.gtag exists; provide a no-op fallback for local/dev cases.
  window.gtag = (...args: unknown[]) => {
    window.dataLayer?.push(args);
  };
}

export {};
