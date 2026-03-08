import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Dashboard from './Dashboard';

// vi.hoisted ensures these fns are available when vi.mock factory runs (which is hoisted above imports)
const mockFns = vi.hoisted(() => ({
  getKPIs: vi.fn(),
  getSalesTrend: vi.fn(),
  getCategoryBreakdown: vi.fn(),
  getRegionalPerformance: vi.fn(),
  getTopProducts: vi.fn(),
  explainCard: vi.fn(),
  getCardActions: vi.fn(),
  addAction: vi.fn(),
  getActions: vi.fn(),
}));

vi.mock('@/lib/api', () => ({
  dashboardAPI: {
    getKPIs: mockFns.getKPIs,
    getSalesTrend: mockFns.getSalesTrend,
    getCategoryBreakdown: mockFns.getCategoryBreakdown,
    getRegionalPerformance: mockFns.getRegionalPerformance,
    getTopProducts: mockFns.getTopProducts,
    explainCard: mockFns.explainCard,
    getCardActions: mockFns.getCardActions,
  },
  actionsAPI: {
    addAction: mockFns.addAction,
    getActions: mockFns.getActions,
  },
}));

// ---------------------------------------------------------------------------
// Test data
// ---------------------------------------------------------------------------

const kpisData = {
  total_gmv: 5000000,
  avg_margin_pct: 22.5,
  total_units: 12000,
  active_skus: 200,
  gmv_change_pct: 8.3,
  margin_change_pct: 1.5,
};

const salesTrendData = [
  { month: '2025-12', revenue: 1500000, margin: 300000, units: 4000 },
  { month: '2026-01', revenue: 1800000, margin: 360000, units: 4800 },
  { month: '2026-02', revenue: 1700000, margin: 340000, units: 4500 },
];

const categoriesData = [
  { name: 'Diapers', revenue: 2000000, margin: 400000, units: 6000, sku_count: 50 },
  { name: 'Wipes', revenue: 800000, margin: 160000, units: 3000, sku_count: 20 },
];

const regionsData = [
  { name: 'North', revenue: 1200000, margin: 240000, units: 3200, avg_discount: 5.0 },
  { name: 'South', revenue: 900000, margin: 180000, units: 2400, avg_discount: 6.5 },
];

const topProductsData = [
  { name: 'Pampers Classic', category: 'Diapers', brand: 'Pampers', revenue: 400000, units: 1200, margin_pct: 32.5 },
  { name: 'Huggies Soft', category: 'Diapers', brand: 'Huggies', revenue: 350000, units: 1000, margin_pct: 28.0 },
];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function setupDefaultMocks() {
  mockFns.getKPIs.mockResolvedValue(kpisData);
  mockFns.getSalesTrend.mockResolvedValue(salesTrendData);
  mockFns.getCategoryBreakdown.mockResolvedValue(categoriesData);
  mockFns.getRegionalPerformance.mockResolvedValue(regionsData);
  mockFns.getTopProducts.mockResolvedValue(topProductsData);
  mockFns.explainCard.mockResolvedValue({ explanation: 'Great performance!' });
  mockFns.getCardActions.mockResolvedValue({
    actions: ['Reorder best-selling SKUs', 'Launch flash sale', 'Negotiate supplier margins'],
  });
  mockFns.addAction.mockResolvedValue({ message: 'created', id: 'act-new' });
  mockFns.getActions.mockResolvedValue([]);
}

function renderDashboard() {
  return render(
    <MemoryRouter>
      <Dashboard />
    </MemoryRouter>,
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('Dashboard page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setupDefaultMocks();
  });

  it('shows loading state initially', () => {
    mockFns.getKPIs.mockReturnValue(new Promise(() => {}));
    mockFns.getSalesTrend.mockReturnValue(new Promise(() => {}));
    mockFns.getCategoryBreakdown.mockReturnValue(new Promise(() => {}));
    mockFns.getRegionalPerformance.mockReturnValue(new Promise(() => {}));
    mockFns.getTopProducts.mockReturnValue(new Promise(() => {}));
    renderDashboard();
    expect(screen.getByText(/loading dashboard/i)).toBeInTheDocument();
  });

  it('renders dashboard title and subtitle after data loads', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Dashboard')).toBeInTheDocument();
    });
    expect(screen.getByText(/baby.*mother care/i)).toBeInTheDocument();
  });

  it('renders KPI cards after data loads', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Total GMV')).toBeInTheDocument();
    });
    expect(screen.getByText('Avg Margin')).toBeInTheDocument();
    expect(screen.getByText('Units Sold')).toBeInTheDocument();
    expect(screen.getByText('Active SKUs')).toBeInTheDocument();
  });

  it('renders revenue trend chart section', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Revenue Trend (Monthly)')).toBeInTheDocument();
    });
  });

  it('renders category breakdown section', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('By Category')).toBeInTheDocument();
    });
    // 'Diapers' appears in both category breakdown and top products table
    expect(screen.getAllByText('Diapers').length).toBeGreaterThan(0);
  });

  it('renders regional performance section', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Regional Performance')).toBeInTheDocument();
    });
    expect(screen.getByText('North')).toBeInTheDocument();
  });

  it('renders top products table', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Top 10 Products')).toBeInTheDocument();
    });
    expect(screen.getByText('Pampers Classic')).toBeInTheDocument();
  });

  it('shows error state when API fails', async () => {
    mockFns.getKPIs.mockRejectedValue(new Error('Network error'));
    mockFns.getSalesTrend.mockRejectedValue(new Error('Network error'));
    mockFns.getCategoryBreakdown.mockRejectedValue(new Error('Network error'));
    mockFns.getRegionalPerformance.mockRejectedValue(new Error('Network error'));
    mockFns.getTopProducts.mockRejectedValue(new Error('Network error'));
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Connection Error')).toBeInTheDocument();
    });
    expect(screen.getByText('Network error')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });

  it('displays active_skus value from KPI data', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('200')).toBeInTheDocument();
    });
  });

  it('renders top products brand names', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Pampers')).toBeInTheDocument();
    });
    expect(screen.getByText('Huggies')).toBeInTheDocument();
  });

  it('renders South region', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('South')).toBeInTheDocument();
    });
  });

  it('renders Wipes category', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Wipes')).toBeInTheDocument();
    });
  });

  /** Helper: open the ⋮ dropdown on the first KPI card and click Actions */
  async function openActionsModal() {
    await waitFor(() => expect(screen.getByText('Total GMV')).toBeInTheDocument());
    const menuButtons = screen.getAllByTitle('Options');
    fireEvent.click(menuButtons[0]);
    // Scope to the dropdown to avoid matching inline region-card Actions buttons
    await waitFor(() => expect(document.querySelector('.explain-dropdown')).toBeTruthy());
    const dropdown = document.querySelector('.explain-dropdown') as HTMLElement;
    const actionsBtn = within(dropdown).getByText(/Actions/);
    fireEvent.click(actionsBtn);
  }

  it('shows actions modal with 3 LLM-generated actions when Actions button clicked', async () => {
    renderDashboard();
    await openActionsModal();

    // Loading state appears while API resolves
    expect(await screen.findByText(/analyzing/i)).toBeInTheDocument();

    // Actions appear after API resolves
    await waitFor(() => {
      expect(screen.getByText('Reorder best-selling SKUs')).toBeInTheDocument();
    });
    expect(screen.getByText('Launch flash sale')).toBeInTheDocument();
    expect(screen.getByText('Negotiate supplier margins')).toBeInTheDocument();

    // Modal title includes card type
    expect(screen.getByText(/Recommended Actions — Total GMV/)).toBeInTheDocument();
  });

  it('shows fallback actions when getCardActions API fails', async () => {
    mockFns.getCardActions.mockRejectedValue(new Error('LLM unavailable'));
    renderDashboard();
    await openActionsModal();

    await waitFor(() => {
      expect(screen.getByText(/Review pricing strategy/i)).toBeInTheDocument();
    });
  });

  it('closes actions modal when backdrop is clicked', async () => {
    renderDashboard();
    await openActionsModal();

    await waitFor(() => expect(screen.getByText('Reorder best-selling SKUs')).toBeInTheDocument());

    // Click the backdrop (portal renders to document.body)
    const backdrop = document.querySelector('.modal-backdrop') as HTMLElement;
    fireEvent.click(backdrop);

    await waitFor(() => {
      expect(screen.queryByText('Reorder best-selling SKUs')).not.toBeInTheDocument();
    });
  });
});
