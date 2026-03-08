import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import App from './App';

// ---------------------------------------------------------------------------
// Mock all page components and layout components to isolate routing logic
// ---------------------------------------------------------------------------

vi.mock('./pages/Dashboard', () => ({
  default: () => <div data-testid="page-dashboard">Dashboard Page</div>,
}));

vi.mock('./pages/Alerts', () => ({
  default: () => <div data-testid="page-alerts">Alerts Page</div>,
}));

vi.mock('./pages/Actions', () => ({
  default: () => <div data-testid="page-actions">Actions Page</div>,
}));

vi.mock('./pages/News', () => ({
  default: () => <div data-testid="page-news">News Page</div>,
}));

vi.mock('./pages/Reports', () => ({
  default: () => <div data-testid="page-reports">Reports Page</div>,
}));

vi.mock('./pages/Config', () => ({
  default: () => <div data-testid="page-config">Config Page</div>,
}));

vi.mock('./pages/Chat', () => ({
  default: () => <div data-testid="page-chat">Chat Page</div>,
}));

vi.mock('./components/layout/Sidebar', () => ({
  default: () => <nav data-testid="sidebar">Sidebar</nav>,
}));

vi.mock('./components/chat/ChatPanel', () => ({
  default: () => <div data-testid="chat-panel">ChatPanel</div>,
}));

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Renders App with MemoryRouter pre-set to a given path.
 * Note: App itself renders <BrowserRouter>, so we use window.history to
 * navigate rather than wrapping; instead we test via direct path rendering.
 */
function renderAtPath(path: string) {
  // Replace the current URL for BrowserRouter inside App
  window.history.pushState({}, '', path);
  return render(<App />);
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('App routing', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders without crashing', () => {
    renderAtPath('/dashboard');
    expect(screen.getByTestId('sidebar')).toBeInTheDocument();
    expect(screen.getByTestId('chat-panel')).toBeInTheDocument();
  });

  it('redirects "/" to "/dashboard"', () => {
    renderAtPath('/');
    expect(screen.getByTestId('page-dashboard')).toBeInTheDocument();
  });

  it('renders Dashboard page at /dashboard', () => {
    renderAtPath('/dashboard');
    expect(screen.getByTestId('page-dashboard')).toBeInTheDocument();
  });

  it('renders Alerts page at /alerts', () => {
    renderAtPath('/alerts');
    expect(screen.getByTestId('page-alerts')).toBeInTheDocument();
  });

  it('renders Actions page at /actions', () => {
    renderAtPath('/actions');
    expect(screen.getByTestId('page-actions')).toBeInTheDocument();
  });

  it('renders News page at /news', () => {
    renderAtPath('/news');
    expect(screen.getByTestId('page-news')).toBeInTheDocument();
  });

  it('renders Reports page at /reports', () => {
    renderAtPath('/reports');
    expect(screen.getByTestId('page-reports')).toBeInTheDocument();
  });

  it('renders Config page at /config', () => {
    renderAtPath('/config');
    expect(screen.getByTestId('page-config')).toBeInTheDocument();
  });

  it('renders Chat page at /chat', () => {
    renderAtPath('/chat');
    expect(screen.getByTestId('page-chat')).toBeInTheDocument();
  });

  it('renders 404 message for unknown routes', () => {
    renderAtPath('/this-does-not-exist');
    expect(screen.getByText('404 Not Found')).toBeInTheDocument();
  });
});
