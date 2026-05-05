import { afterEach, beforeEach, describe, it, expect, vi } from 'vitest';
import { render, waitFor, cleanup, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import App from './App';
import { api } from './api/client';
import { useSnapshotStore } from './store/snapshot';

// Mock the stream module so we can assert connect() call count + cleanup.
// Each call returns a fresh teardown fn so we can verify it's invoked on unmount.
const teardown = vi.fn();
const connectMock = vi.fn(async (_opts: unknown) => teardown);

vi.mock('./api/stream', () => ({
  connect: (opts: unknown) => connectMock(opts),
}));

// Mock the typed API client so the header tag tests get a deterministic
// version string regardless of the global fetch stub. Implementations are
// (re)installed in beforeEach because vi.restoreAllMocks() in afterEach
// clears mockResolvedValue from vi.fn() instances between tests.
vi.mock('./api/client', () => ({
  api: {
    uiconfig: vi.fn(),
    version: vi.fn(),
  },
}));

function installApiMocks() {
  vi.mocked(api.uiconfig).mockResolvedValue({
    defaultTheme: 'dark',
    defaultLanding: '/',
    enableHistory: true,
  } as Awaited<ReturnType<typeof api.uiconfig>>);
  vi.mocked(api.version).mockResolvedValue({
    version: 'v0.4.1',
    commit: 'abc123',
    date: '2026-05-05',
  } as Awaited<ReturnType<typeof api.version>>);
}

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
    installApiMocks();
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

describe('App header tag', () => {
  beforeEach(() => {
    connectMock.mockClear();
    teardown.mockClear();
    stubBrowserGlobals();
    installApiMocks();
  });
  afterEach(async () => {
    await new Promise((r) => setTimeout(r, 0));
    cleanup();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    useSnapshotStore.setState({ snapshot: null, connection: 'connecting' });
  });

  it('renders the build-version tag once /api/version resolves', async () => {
    render(<MemoryRouter><App initialThemeMode="dark" /></MemoryRouter>);
    // The version string appears in both the header tag and the footer line;
    // assert the header `<Tag>` contains it specifically (the deprecated tag
    // had no version text, so finding "cwd v0.4.1" inside an ant-tag span is
    // the load-bearing assertion).
    await waitFor(() => {
      const matches = screen.getAllByText(/cwd v0\.4\.1/);
      const inTag = matches.some((el) =>
        el.closest('.ant-tag') !== null,
      );
      expect(inTag).toBe(true);
    });
  });

  it('does not render the deprecated "Phase 0 — skeleton" tag', () => {
    render(<MemoryRouter><App initialThemeMode="dark" /></MemoryRouter>);
    expect(screen.queryByText(/Phase 0 — skeleton/)).toBeNull();
  });
});
