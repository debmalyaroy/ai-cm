import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Actions from './Actions';

vi.mock('./CreateActionModal', () => ({
  default: ({ onClose }: { onClose: () => void }) => (
    <div data-testid="create-action-modal">
      <button onClick={onClose}>Close Modal</button>
    </div>
  ),
}));

const mockFns = vi.hoisted(() => ({
  getActions: vi.fn(),
  approveAction: vi.fn(),
  rejectAction: vi.fn(),
  revertAction: vi.fn(),
  addAction: vi.fn(),
  generateActions: vi.fn(),
  draftAction: vi.fn(),
  getComments: vi.fn(),
  addComment: vi.fn(),
  updateAction: vi.fn(),
}));

vi.mock('@/lib/api', () => ({
  actionsAPI: {
    getActions: mockFns.getActions,
    approveAction: mockFns.approveAction,
    rejectAction: mockFns.rejectAction,
    revertAction: mockFns.revertAction,
    addAction: mockFns.addAction,
    generateActions: mockFns.generateActions,
    draftAction: mockFns.draftAction,
    getComments: mockFns.getComments,
    addComment: mockFns.addComment,
    updateAction: mockFns.updateAction,
  },
}));

// ---------------------------------------------------------------------------
// Test data
// ---------------------------------------------------------------------------

const actionsData = [
  {
    id: 'act-1',
    title: 'Restock Pampers Classic',
    description: 'Inventory below reorder point. Recommend restocking 500 units.',
    action_type: 'restock',
    category: 'Diapers',
    confidence_score: 0.92,
    status: 'pending',
    created_at: '2026-03-07T09:00:00Z',
    updated_at: '2026-03-07T09:00:00Z',
    product_name: 'Pampers Classic',
    priority: 'high' as const,
    expected_impact: '+15% availability',
  },
  {
    id: 'act-2',
    title: 'Price Match Huggies Wipes',
    description: 'Competitor dropped price; recommend matching to retain market share.',
    action_type: 'price_match',
    category: 'Wipes',
    confidence_score: 0.78,
    status: 'approved',
    created_at: '2026-03-06T12:00:00Z',
    updated_at: '2026-03-07T08:00:00Z',
    product_name: 'Huggies Wipes',
    priority: 'medium' as const,
    expected_impact: '+5% revenue',
  },
  {
    id: 'act-3',
    title: 'Delist Slow SKU',
    description: 'SKU has not moved in 90 days.',
    action_type: 'delist',
    category: 'Misc',
    confidence_score: 0.55,
    status: 'rejected',
    created_at: '2026-03-05T10:00:00Z',
    updated_at: '2026-03-05T11:00:00Z',
    product_name: 'Slow Product',
    priority: 'low' as const,
    expected_impact: '-costs',
  },
];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderActions() {
  return render(
    <MemoryRouter>
      <Actions />
    </MemoryRouter>,
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('Actions page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFns.getActions.mockResolvedValue(actionsData);
    mockFns.approveAction.mockResolvedValue({ message: 'approved' });
    mockFns.rejectAction.mockResolvedValue({ message: 'rejected' });
    mockFns.revertAction.mockResolvedValue({ message: 'reverted' });
    mockFns.getComments.mockResolvedValue([]);
    mockFns.updateAction.mockResolvedValue({ message: 'updated' });
  });

  it('shows loading state initially', () => {
    mockFns.getActions.mockReturnValue(new Promise(() => {}));
    renderActions();
    expect(screen.getByText(/loading actions/i)).toBeInTheDocument();
  });

  it('renders page header after data loads', async () => {
    renderActions();
    await waitFor(() => {
      expect(screen.getByText('Action Center')).toBeInTheDocument();
    });
    expect(screen.getByText(/AI-recommended actions/i)).toBeInTheDocument();
  });

  it('renders action titles in list', async () => {
    renderActions();
    await waitFor(() => {
      expect(screen.getByText('Restock Pampers Classic')).toBeInTheDocument();
    });
    expect(screen.getByText('Price Match Huggies Wipes')).toBeInTheDocument();
    expect(screen.getByText('Delist Slow SKU')).toBeInTheDocument();
  });

  it('renders status summary counts', async () => {
    renderActions();
    await waitFor(() => {
      // 'Pending' appears in both the stats summary and the filter buttons
      expect(screen.getAllByText('Pending').length).toBeGreaterThan(0);
    });
    expect(screen.getAllByText('Approved').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Rejected').length).toBeGreaterThan(0);
  });

  it('renders filter buttons', async () => {
    renderActions();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /^all$/i })).toBeInTheDocument();
    });
  });

  it('renders view mode toggle buttons', async () => {
    renderActions();
    await waitFor(() => {
      expect(screen.getByTitle('Grid')).toBeInTheDocument();
    });
    expect(screen.getByTitle('List')).toBeInTheDocument();
    expect(screen.getByTitle('Details')).toBeInTheDocument();
  });

  it('renders Create New Action button', async () => {
    renderActions();
    await waitFor(() => {
      expect(screen.getByText(/create new action/i)).toBeInTheDocument();
    });
  });

  it('opens CreateActionModal when Create New Action is clicked', async () => {
    renderActions();
    await waitFor(() => {
      expect(screen.getByText(/create new action/i)).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText(/create new action/i));
    expect(screen.getByTestId('create-action-modal')).toBeInTheDocument();
  });

  it('closes CreateActionModal when its close button is clicked', async () => {
    renderActions();
    await waitFor(() => {
      expect(screen.getByText(/create new action/i)).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText(/create new action/i));
    expect(screen.getByTestId('create-action-modal')).toBeInTheDocument();
    fireEvent.click(screen.getByText('Close Modal'));
    expect(screen.queryByTestId('create-action-modal')).not.toBeInTheDocument();
  });

  it('shows error state when API fails', async () => {
    mockFns.getActions.mockRejectedValue(new Error('Failed to load actions'));
    renderActions();
    await waitFor(() => {
      expect(screen.getByText('Connection Error')).toBeInTheDocument();
    });
    expect(screen.getByText('Failed to load actions')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });

  it('shows empty state when no actions exist', async () => {
    mockFns.getActions.mockResolvedValue([]);
    renderActions();
    await waitFor(() => {
      expect(screen.getByText('No actions found')).toBeInTheDocument();
    });
  });

  it('switches to grid view when Grid button is clicked', async () => {
    renderActions();
    await waitFor(() => {
      expect(screen.getByTitle('Grid')).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTitle('Grid'));
    await waitFor(() => {
      expect(screen.getByText('Restock Pampers Classic')).toBeInTheDocument();
    });
  });

  it('filters actions by status when Pending filter is clicked', async () => {
    renderActions();
    await waitFor(() => {
      expect(screen.getAllByRole('button', { name: /^pending$/i }).length).toBeGreaterThan(0);
    });
    const filterButtons = screen.getAllByRole('button', { name: /^pending$/i });
    fireEvent.click(filterButtons[0]);
    await waitFor(() => {
      expect(mockFns.getActions).toHaveBeenCalledWith('pending');
    });
  });
});
