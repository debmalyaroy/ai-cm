import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Alerts from './Alerts';

const mockFns = vi.hoisted(() => ({
  getAlerts: vi.fn(),
  acknowledgeAlert: vi.fn(),
  addAlert: vi.fn(),
}));

vi.mock('@/lib/api', () => ({
  alertsAPI: {
    getAlerts: mockFns.getAlerts,
    acknowledgeAlert: mockFns.acknowledgeAlert,
    addAlert: mockFns.addAlert,
  },
}));

// ---------------------------------------------------------------------------
// Test data
// ---------------------------------------------------------------------------

const alertsData = [
  {
    id: 'a1',
    title: 'Critical Stock Depletion',
    severity: 'critical',
    category: 'Diapers',
    message: 'Pampers Classic will be out of stock in 2 days.',
    created_at: '2026-03-07T10:00:00Z',
    acknowledged: false,
  },
  {
    id: 'a2',
    title: 'Competitor Price Drop',
    severity: 'warning',
    category: 'Wipes',
    message: 'Huggies Wipes dropped price by 18%.',
    created_at: '2026-03-07T08:00:00Z',
    acknowledged: true,
  },
  {
    id: 'a3',
    title: 'New Product Launch',
    severity: 'info',
    category: 'Formula',
    message: 'Competitor launched new organic formula.',
    created_at: '2026-03-06T12:00:00Z',
    acknowledged: false,
  },
];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderAlerts() {
  return render(
    <MemoryRouter>
      <Alerts />
    </MemoryRouter>,
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('Alerts page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFns.getAlerts.mockResolvedValue(alertsData);
    mockFns.acknowledgeAlert.mockResolvedValue({ message: 'acknowledged' });
    mockFns.addAlert.mockResolvedValue({ message: 'created' });
  });

  it('shows loading state initially', () => {
    mockFns.getAlerts.mockReturnValue(new Promise(() => {}));
    renderAlerts();
    expect(screen.getByText(/loading alerts/i)).toBeInTheDocument();
  });

  it('renders page title after data loads', async () => {
    renderAlerts();
    await waitFor(() => {
      expect(screen.getByText(/alerts & notifications/i)).toBeInTheDocument();
    });
  });

  it('renders subtitle text', async () => {
    renderAlerts();
    await waitFor(() => {
      expect(screen.getByText(/real-time alerts from watchdog agent/i)).toBeInTheDocument();
    });
  });

  it('shows unacknowledged count badge', async () => {
    renderAlerts();
    await waitFor(() => {
      expect(screen.getByText('2 new')).toBeInTheDocument();
    });
  });

  it('renders alert titles after data loads', async () => {
    renderAlerts();
    await waitFor(() => {
      expect(screen.getByText('Critical Stock Depletion')).toBeInTheDocument();
    });
    expect(screen.getByText('Competitor Price Drop')).toBeInTheDocument();
    expect(screen.getByText('New Product Launch')).toBeInTheDocument();
  });

  it('renders alert messages', async () => {
    renderAlerts();
    await waitFor(() => {
      expect(screen.getByText(/pampers classic will be out of stock/i)).toBeInTheDocument();
    });
  });

  it('renders filter buttons', async () => {
    renderAlerts();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /^all$/i })).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /^unacknowledged$/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^critical$/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^warning$/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^info$/i })).toBeInTheDocument();
  });

  it('shows Acknowledge buttons only for unacknowledged alerts', async () => {
    renderAlerts();
    await waitFor(() => {
      expect(screen.getByText('Critical Stock Depletion')).toBeInTheDocument();
    });
    // a1 and a3 are unacknowledged — 2 "✓ Acknowledge" action buttons
    const ackButtons = screen.getAllByText('✓ Acknowledge');
    expect(ackButtons).toHaveLength(2);
  });

  it('calls acknowledgeAlert when Acknowledge is clicked', async () => {
    renderAlerts();
    await waitFor(() => {
      expect(screen.getByText('Critical Stock Depletion')).toBeInTheDocument();
    });
    const ackButtons = screen.getAllByText('✓ Acknowledge');
    fireEvent.click(ackButtons[0]);
    await waitFor(() => {
      expect(mockFns.acknowledgeAlert).toHaveBeenCalled();
    });
  });

  it('filters to unacknowledged only when filter is clicked', async () => {
    renderAlerts();
    await waitFor(() => {
      expect(screen.getByText('Critical Stock Depletion')).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('button', { name: /unacknowledged/i }));
    await waitFor(() => {
      expect(screen.queryByText('Competitor Price Drop')).not.toBeInTheDocument();
    });
    expect(screen.getByText('Critical Stock Depletion')).toBeInTheDocument();
  });

  it('filters by severity when critical filter is clicked', async () => {
    renderAlerts();
    await waitFor(() => {
      expect(screen.getByText('Critical Stock Depletion')).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('button', { name: /critical/i }));
    await waitFor(() => {
      expect(screen.queryByText('New Product Launch')).not.toBeInTheDocument();
    });
    expect(screen.getByText('Critical Stock Depletion')).toBeInTheDocument();
  });

  it('shows empty state when no alerts match filter', async () => {
    mockFns.getAlerts.mockResolvedValue([]);
    renderAlerts();
    await waitFor(() => {
      expect(screen.getByText('No alerts found')).toBeInTheDocument();
    });
  });
});
