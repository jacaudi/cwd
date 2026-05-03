import { afterEach, beforeEach, describe, it, expect, vi } from 'vitest';
import { render, waitFor, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import App from './App';
import { useSnapshotStore } from './store/snapshot';

// Mock the stream module so we can assert connect() call count + cleanup.
// Each call returns a fresh teardown fn so we can verify it's invoked on unmount.
const teardown = vi.fn();
const connectMock = vi.fn(async (_opts: unknown) => teardown);

vi.mock('./api/stream', () => ({
  connect: (opts: unknown) => connectMock(opts),
}));

// AntD ProLayout pulls in icons/CSS-in-JS that can be noisy under jsdom; the
// matchMedia stub in setup.ts already covers the main offender. We also stub
// fetch + EventSource so any code path that bypassed the mock fails loudly.
function stubBrowserGlobals() {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ defaultTheme: 'dark', version: 'test' }),
    }),
  );
  class FakeES {
    addEventListener() {}
    close() {}
    onerror: any = null;
  }
  vi.stubGlobal('EventSource', FakeES as any);
  // Belt-and-suspenders: jsdom provides localStorage, but the App's first-paint
  // effect reads it asynchronously. If a stale promise resolves after vitest
  // has torn down the document/window the call would NPE, so make it resilient.
  if (typeof localStorage === 'undefined' || typeof localStorage.getItem !== 'function') {
    const store = new Map<string, string>();
    vi.stubGlobal('localStorage', {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => { store.set(k, v); },
      removeItem: (k: string) => { store.delete(k); },
      clear: () => { store.clear(); },
      key: () => null,
      length: 0,
    });
  }
}

describe('App SSE lifecycle', () => {
  beforeEach(() => {
    connectMock.mockClear();
    teardown.mockClear();
    stubBrowserGlobals();
  });
  afterEach(async () => {
    // Drain pending microtasks so the App's first-paint Promise.allSettled
    // resolves against the still-stubbed fetch, not the next test's stub.
    await new Promise((r) => setTimeout(r, 0));
    cleanup();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    useSnapshotStore.setState({ snapshot: null, connection: 'connecting' });
  });

  it('opens the SSE stream exactly once when mounted on Overview (/)', async () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <App initialThemeMode="dark" />
      </MemoryRouter>,
    );
    await waitFor(() => expect(connectMock).toHaveBeenCalledTimes(1));
    // Sanity: opts shape carries the snapshot/update/error handlers.
    const opts = connectMock.mock.calls[0][0] as Record<string, unknown>;
    expect(typeof opts.onSnapshot).toBe('function');
    expect(typeof opts.onUpdate).toBe('function');
    expect(typeof opts.onError).toBe('function');
  });

  it('opens the SSE stream when deep-linking to /space', async () => {
    render(
      <MemoryRouter initialEntries={['/space']}>
        <App initialThemeMode="dark" />
      </MemoryRouter>,
    );
    await waitFor(() => expect(connectMock).toHaveBeenCalledTimes(1));
  });

  it('opens the SSE stream when deep-linking to /events', async () => {
    render(
      <MemoryRouter initialEntries={['/events']}>
        <App initialThemeMode="dark" />
      </MemoryRouter>,
    );
    await waitFor(() => expect(connectMock).toHaveBeenCalledTimes(1));
  });

  it('opens the SSE stream when deep-linking to /hazards', async () => {
    render(
      <MemoryRouter initialEntries={['/hazards']}>
        <App initialThemeMode="dark" />
      </MemoryRouter>,
    );
    await waitFor(() => expect(connectMock).toHaveBeenCalledTimes(1));
  });

  it('tears down the SSE stream on App unmount', async () => {
    const { unmount } = render(
      <MemoryRouter initialEntries={['/']}>
        <App initialThemeMode="dark" />
      </MemoryRouter>,
    );
    await waitFor(() => expect(connectMock).toHaveBeenCalledTimes(1));
    // Wait for the connect promise to resolve so the cleanup capture runs.
    await waitFor(() => expect(teardown).toHaveBeenCalledTimes(0));
    unmount();
    await waitFor(() => expect(teardown).toHaveBeenCalledTimes(1));
  });
});
