import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Config from './Config';

const mockFns = vi.hoisted(() => ({
  get: vi.fn(),
  save: vi.fn(),
}));

vi.mock('@/lib/api', () => ({
  preferencesAPI: {
    get: mockFns.get,
    save: mockFns.save,
  },
}));

// ---------------------------------------------------------------------------
// Test data
// ---------------------------------------------------------------------------

const prefsData: Record<string, string> = {
  llm_temperature: '0.7',
  confidence_threshold: '0.7',
  auto_approve: 'false',
  max_retries: '3',
  price_drop_critical: '20',
  price_drop_warning: '15',
  stockout_days: '7',
  excess_inventory_days: '60',
  email_alerts: 'true',
  dashboard_refresh: '60',
  alert_sound: 'false',
  chat_dock_position: 'right',
  show_reasoning: 'true',
  show_confidence: 'true',
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderConfig() {
  return render(
    <MemoryRouter>
      <Config />
    </MemoryRouter>,
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('Config page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFns.get.mockResolvedValue(prefsData);
    mockFns.save.mockResolvedValue({ message: 'saved' });
  });

  it('shows loading state initially', () => {
    mockFns.get.mockReturnValue(new Promise(() => {}));
    renderConfig();
    expect(screen.getByText(/loading preferences/i)).toBeInTheDocument();
  });

  it('renders page title after data loads', async () => {
    renderConfig();
    await waitFor(() => {
      expect(screen.getByText('Configuration')).toBeInTheDocument();
    });
  });

  it('renders page subtitle', async () => {
    renderConfig();
    await waitFor(() => {
      expect(screen.getByText(/system configuration and user preferences/i)).toBeInTheDocument();
    });
  });

  it('renders all config section titles', async () => {
    renderConfig();
    await waitFor(() => {
      expect(screen.getByText('AI Agent Settings')).toBeInTheDocument();
    });
    expect(screen.getByText('Watchdog Configuration')).toBeInTheDocument();
    expect(screen.getByText('Notification Preferences')).toBeInTheDocument();
    expect(screen.getByText('UI Preferences')).toBeInTheDocument();
  });

  it('renders field labels within sections', async () => {
    renderConfig();
    await waitFor(() => {
      expect(screen.getByText('LLM Temperature')).toBeInTheDocument();
    });
    expect(screen.getByText('Confidence Threshold')).toBeInTheDocument();
    expect(screen.getByText('Auto-approve High Confidence Actions')).toBeInTheDocument();
  });

  it('renders Save Changes button', async () => {
    renderConfig();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /save changes/i })).toBeInTheDocument();
    });
  });

  it('calls preferencesAPI.save when Save Changes is clicked', async () => {
    renderConfig();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /save changes/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('button', { name: /save changes/i }));
    await waitFor(() => {
      expect(mockFns.save).toHaveBeenCalledTimes(1);
    });
    const callArgs = mockFns.save.mock.calls[0][0] as Record<string, string>;
    expect(typeof callArgs).toBe('object');
    expect(callArgs.llm_temperature).toBeDefined();
  });

  it('shows "Saved" confirmation after successful save', async () => {
    renderConfig();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /save changes/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('button', { name: /save changes/i }));
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /✓ saved/i })).toBeInTheDocument();
    });
  });

  it('loads preferences from API on mount', async () => {
    renderConfig();
    await waitFor(() => {
      expect(mockFns.get).toHaveBeenCalledTimes(1);
    });
  });

  it('uses default values when API fails', async () => {
    mockFns.get.mockRejectedValue(new Error('Network error'));
    renderConfig();
    await waitFor(() => {
      expect(screen.getByText('Configuration')).toBeInTheDocument();
    });
    expect(screen.getByText('AI Agent Settings')).toBeInTheDocument();
  });

  it('renders Save Changes button is clickable', async () => {
    renderConfig();
    await waitFor(() => {
      expect(screen.getByText('Auto-approve High Confidence Actions')).toBeInTheDocument();
    });
    const saveBtn = screen.getByRole('button', { name: /save changes/i });
    expect(saveBtn).toBeInTheDocument();
  });

  it('renders select dropdown for chat_dock_position', async () => {
    renderConfig();
    await waitFor(() => {
      expect(screen.getByText('Default Chat Panel Position')).toBeInTheDocument();
    });
    const select = screen.getByDisplayValue('right');
    expect(select).toBeInTheDocument();
    expect(select.tagName).toBe('SELECT');
  });

  it('changing select value updates config', async () => {
    renderConfig();
    await waitFor(() => {
      expect(screen.getByDisplayValue('right')).toBeInTheDocument();
    });
    fireEvent.change(screen.getByDisplayValue('right'), { target: { value: 'left' } });
    expect(screen.getByDisplayValue('left')).toBeInTheDocument();
  });
});
