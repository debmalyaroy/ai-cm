import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Sidebar from './Sidebar';

// ---------------------------------------------------------------------------
// localStorage mock — jsdom provides it but we want clean state per test
// ---------------------------------------------------------------------------

beforeEach(() => {
  localStorage.clear();
  vi.clearAllMocks();
});

afterEach(() => {
  localStorage.clear();
});

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderSidebar(path = '/dashboard') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Sidebar />
    </MemoryRouter>,
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('Sidebar', () => {
  it('renders the AI-CM logo text', () => {
    renderSidebar();
    expect(screen.getByText('AI-CM')).toBeInTheDocument();
  });

  it('renders the logo subtitle "Category Copilot"', () => {
    renderSidebar();
    expect(screen.getByText('Category Copilot')).toBeInTheDocument();
  });

  it('renders all navigation links', () => {
    renderSidebar();
    expect(screen.getByText('Dashboard')).toBeInTheDocument();
    expect(screen.getByText('News')).toBeInTheDocument();
    expect(screen.getByText('Approvals')).toBeInTheDocument();
    expect(screen.getByText('Reports')).toBeInTheDocument();
    expect(screen.getByText('Alerts')).toBeInTheDocument();
    expect(screen.getByText('Config')).toBeInTheDocument();
  });

  it('navigation links have correct href attributes', () => {
    renderSidebar();
    expect(screen.getByRole('link', { name: /dashboard/i })).toHaveAttribute('href', '/dashboard');
    expect(screen.getByRole('link', { name: /news/i })).toHaveAttribute('href', '/news');
    expect(screen.getByRole('link', { name: /approvals/i })).toHaveAttribute('href', '/actions');
    expect(screen.getByRole('link', { name: /reports/i })).toHaveAttribute('href', '/reports');
    expect(screen.getByRole('link', { name: /alerts/i })).toHaveAttribute('href', '/alerts');
    expect(screen.getByRole('link', { name: /config/i })).toHaveAttribute('href', '/config');
  });

  it('applies active class to the link matching the current path', () => {
    renderSidebar('/dashboard');
    const dashboardLink = screen.getByRole('link', { name: /dashboard/i });
    expect(dashboardLink).toHaveClass('active');
  });

  it('does not apply active class to non-current links', () => {
    renderSidebar('/dashboard');
    const newsLink = screen.getByRole('link', { name: /news/i });
    expect(newsLink).not.toHaveClass('active');
  });

  it('renders theme toggle button', () => {
    renderSidebar();
    // The theme-toggle button has an emoji as text content — find it by class
    const themeBtn = document.querySelector('.theme-toggle');
    expect(themeBtn).not.toBeNull();
  });

  it('renders collapse toggle button', () => {
    renderSidebar();
    const collapseBtn = screen.getByTitle('Collapse');
    expect(collapseBtn).toBeInTheDocument();
  });

  it('renders user info in footer when expanded', () => {
    renderSidebar();
    expect(screen.getByText('Demo User')).toBeInTheDocument();
    expect(screen.getByText('Category Manager')).toBeInTheDocument();
  });

  it('renders "AI" initials in the logo box', () => {
    renderSidebar();
    expect(screen.getByText('AI')).toBeInTheDocument();
  });

  it('starts in dark mode by default (no saved theme)', () => {
    // localStorage has no aicm-theme → defaults to dark
    renderSidebar();
    // The theme toggle shows moon emoji in dark mode
    expect(screen.getByText('🌙')).toBeInTheDocument();
  });

  it('starts in light mode when localStorage has aicm-theme = light', () => {
    localStorage.setItem('aicm-theme', 'light');
    renderSidebar();
    expect(screen.getByText('☀️')).toBeInTheDocument();
  });

  it('collapses sidebar when collapse button is clicked', () => {
    renderSidebar();
    const collapseBtn = screen.getByTitle('Collapse');
    fireEvent.click(collapseBtn);
    // After collapse, nav link text is hidden but "Expand" title appears
    expect(screen.getByTitle('Expand')).toBeInTheDocument();
    // Nav labels should no longer be visible (collapsed mode hides text)
    expect(screen.queryByText('Dashboard')).not.toBeInTheDocument();
  });

  it('persists collapsed state to localStorage', () => {
    renderSidebar();
    const collapseBtn = screen.getByTitle('Collapse');
    fireEvent.click(collapseBtn);
    expect(localStorage.getItem('aicm-sidebar-collapsed')).toBe('true');
  });

  it('expands sidebar when Expand button is clicked after collapse', () => {
    renderSidebar();
    const collapseBtn = screen.getByTitle('Collapse');
    fireEvent.click(collapseBtn);
    expect(screen.queryByText('Dashboard')).not.toBeInTheDocument();

    const expandBtn = screen.getByTitle('Expand');
    fireEvent.click(expandBtn);
    expect(screen.getByText('Dashboard')).toBeInTheDocument();
  });

  it('starts collapsed when localStorage has aicm-sidebar-collapsed = true', () => {
    localStorage.setItem('aicm-sidebar-collapsed', 'true');
    renderSidebar();
    expect(screen.queryByText('Dashboard')).not.toBeInTheDocument();
    expect(screen.getByTitle('Expand')).toBeInTheDocument();
  });

  it('toggles theme when theme button is clicked', () => {
    renderSidebar();
    // Default is dark → moon emoji shown
    expect(screen.getByText('🌙')).toBeInTheDocument();

    // The theme toggle button — find by presence of moon emoji in dark mode
    const darkText = screen.getByText('Dark');
    // Click the parent theme toggle button
    fireEvent.click(darkText.closest('button')!);

    // Should switch to light mode
    expect(screen.getByText('☀️')).toBeInTheDocument();
    expect(localStorage.getItem('aicm-theme')).toBe('light');
  });

  it('persists light theme to localStorage after toggle', () => {
    renderSidebar();
    const darkLabel = screen.getByText('Dark');
    fireEvent.click(darkLabel.closest('button')!);
    expect(localStorage.getItem('aicm-theme')).toBe('light');
  });
});
