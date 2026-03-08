import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Reports from './Reports';

const mockFns = vi.hoisted(() => ({
  downloadReport: vi.fn(),
}));

vi.mock('@/lib/api', () => ({
  reportsAPI: {
    downloadReport: mockFns.downloadReport,
  },
}));

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderReports() {
  return render(
    <MemoryRouter>
      <Reports />
    </MemoryRouter>,
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('Reports page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFns.downloadReport.mockResolvedValue(undefined);
  });

  it('renders page title', () => {
    renderReports();
    expect(screen.getByText(/📋 Reports/i)).toBeInTheDocument();
  });

  it('renders page subtitle', () => {
    renderReports();
    expect(screen.getByText(/AI-generated reports and analytics/i)).toBeInTheDocument();
  });

  it('renders Download CSV button', () => {
    renderReports();
    expect(screen.getByRole('button', { name: /download csv/i })).toBeInTheDocument();
  });

  it('renders Generate Report button', () => {
    renderReports();
    expect(screen.getByRole('button', { name: /generate report/i })).toBeInTheDocument();
  });

  it('renders all mock reports', () => {
    renderReports();
    expect(screen.getByText('Monthly Category Performance')).toBeInTheDocument();
    expect(screen.getByText('Competitor Price Intelligence')).toBeInTheDocument();
    expect(screen.getByText('Inventory Health Report')).toBeInTheDocument();
    expect(screen.getByText('Regional Sales Deep Dive')).toBeInTheDocument();
    expect(screen.getByText('February Forecast Report')).toBeInTheDocument();
    expect(screen.getByText('Quarterly Business Review')).toBeInTheDocument();
  });

  it('renders type filter buttons', () => {
    renderReports();
    expect(screen.getByRole('button', { name: /^all$/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /performance/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /competitive/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /operations/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /analytics/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /forecast/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /executive/i })).toBeInTheDocument();
  });

  it('filters reports by type when a type button is clicked', () => {
    renderReports();
    fireEvent.click(screen.getByRole('button', { name: /^performance$/i }));
    expect(screen.getByText('Monthly Category Performance')).toBeInTheDocument();
    expect(screen.queryByText('Competitor Price Intelligence')).not.toBeInTheDocument();
  });

  it('shows all reports when "all" filter is selected', () => {
    renderReports();
    fireEvent.click(screen.getByRole('button', { name: /^performance$/i }));
    fireEvent.click(screen.getByRole('button', { name: /^all$/i }));
    expect(screen.getByText('Monthly Category Performance')).toBeInTheDocument();
    expect(screen.getByText('Competitor Price Intelligence')).toBeInTheDocument();
  });

  it('renders status badges on reports', () => {
    renderReports();
    const readyBadges = screen.getAllByText('Ready');
    expect(readyBadges.length).toBeGreaterThan(0);
    expect(screen.getByText('Generating...')).toBeInTheDocument();
    expect(screen.getByText('Scheduled')).toBeInTheDocument();
  });

  it('renders Download Report buttons for ready reports', () => {
    renderReports();
    const downloadReportBtns = screen.getAllByRole('button', { name: /download report/i });
    expect(downloadReportBtns.length).toBe(4);
  });

  it('calls reportsAPI.downloadReport when Download CSV button is clicked', async () => {
    renderReports();
    fireEvent.click(screen.getByRole('button', { name: /download csv/i }));
    await waitFor(() => {
      expect(mockFns.downloadReport).toHaveBeenCalledTimes(1);
    });
  });

  it('shows Downloading... text while downloading', async () => {
    let resolveDownload!: () => void;
    mockFns.downloadReport.mockReturnValue(
      new Promise<void>((resolve) => { resolveDownload = resolve; }),
    );
    renderReports();
    fireEvent.click(screen.getByRole('button', { name: /download csv/i }));
    await waitFor(() => {
      expect(screen.getAllByText(/downloading/i).length).toBeGreaterThan(0);
    });
    resolveDownload();
  });

  it('disables download button while downloading', async () => {
    let resolveDownload!: () => void;
    mockFns.downloadReport.mockReturnValue(
      new Promise<void>((resolve) => { resolveDownload = resolve; }),
    );
    renderReports();
    const downloadBtn = screen.getByRole('button', { name: /download csv/i });
    fireEvent.click(downloadBtn);
    await waitFor(() => {
      expect(downloadBtn).toBeDisabled();
    });
    resolveDownload();
  });

  it('renders report descriptions', () => {
    renderReports();
    expect(
      screen.getByText(/comprehensive analysis of revenue, margin, and volume trends/i),
    ).toBeInTheDocument();
  });

  it('renders report periods', () => {
    renderReports();
    expect(screen.getAllByText('📅 January 2026').length).toBeGreaterThan(0);
    expect(screen.getAllByText('📅 Q1 2026').length).toBeGreaterThan(0);
  });
});
