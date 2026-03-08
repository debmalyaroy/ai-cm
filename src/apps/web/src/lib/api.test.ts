import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  fetchAPI,
  dashboardAPI,
  chatAPI,
  actionsAPI,
  alertsAPI,
  preferencesAPI,
  streamChat,
  graphqlQuery,
} from './api';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function mockFetch(data: unknown, ok = true, status = 200) {
  const response = {
    ok,
    status,
    statusText: ok ? 'OK' : 'Bad Request',
    json: vi.fn().mockResolvedValue(data),
    blob: vi.fn().mockResolvedValue(new Blob(['csv,data'], { type: 'text/csv' })),
    headers: new Headers({ 'Content-Disposition': 'attachment; filename=report.csv' }),
    body: null,
  };
  global.fetch = vi.fn().mockResolvedValue(response);
  return response;
}

// ---------------------------------------------------------------------------
// fetchAPI
// ---------------------------------------------------------------------------

describe('fetchAPI', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('makes a GET request and returns parsed JSON', async () => {
    mockFetch({ value: 42 });
    const result = await fetchAPI<{ value: number }>('/api/test', { method: 'GET' });
    expect(result).toEqual({ value: 42 });
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/test',
      expect.objectContaining({ method: 'GET' }),
    );
  });

  it('makes a POST request with body and returns parsed JSON', async () => {
    mockFetch({ id: '1' });
    const result = await fetchAPI<{ id: string }>('/api/test', {
      method: 'POST',
      body: JSON.stringify({ name: 'test' }),
    });
    expect(result).toEqual({ id: '1' });
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/test',
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ name: 'test' }) }),
    );
  });

  it('defaults to POST when no method is provided', async () => {
    mockFetch({});
    await fetchAPI('/api/test');
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/test',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('includes Content-Type: application/json header', async () => {
    mockFetch({});
    await fetchAPI('/api/test', { method: 'GET' });
    const [, options] = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(options.headers['Content-Type']).toBe('application/json');
  });

  it('throws an error when response is not ok', async () => {
    mockFetch({ error: 'Not Found' }, false, 404);
    await expect(fetchAPI('/api/missing', { method: 'GET' })).rejects.toThrow('Not Found');
  });

  it('falls back to statusText when error body cannot be parsed', async () => {
    const response = {
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: vi.fn().mockRejectedValue(new Error('bad json')),
      headers: new Headers(),
    };
    global.fetch = vi.fn().mockResolvedValue(response);
    await expect(fetchAPI('/api/broken', { method: 'GET' })).rejects.toThrow(
      'Internal Server Error',
    );
  });
});

// ---------------------------------------------------------------------------
// dashboardAPI
// ---------------------------------------------------------------------------

describe('dashboardAPI', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('getKPIs calls /api/dashboard/kpis with POST', async () => {
    const kpis = {
      total_gmv: 1000000,
      avg_margin_pct: 22,
      total_units: 5000,
      active_skus: 200,
      gmv_change_pct: 5,
      margin_change_pct: 2,
    };
    mockFetch(kpis);
    const result = await dashboardAPI.getKPIs();
    expect(result).toEqual(kpis);
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/dashboard/kpis',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('getSalesTrend calls /api/dashboard/sales-trend', async () => {
    mockFetch([{ month: '2026-01', revenue: 500000, margin: 100000, units: 2000 }]);
    const result = await dashboardAPI.getSalesTrend();
    expect(Array.isArray(result)).toBe(true);
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/dashboard/sales-trend',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('getCategoryBreakdown calls /api/dashboard/category-breakdown', async () => {
    mockFetch([{ name: 'Diapers', revenue: 300000, margin: 60000, units: 1200, sku_count: 30 }]);
    const result = await dashboardAPI.getCategoryBreakdown();
    expect(Array.isArray(result)).toBe(true);
  });

  it('getRegionalPerformance calls /api/dashboard/regional-performance', async () => {
    mockFetch([{ name: 'North', revenue: 200000, margin: 40000, units: 800, avg_discount: 5 }]);
    const result = await dashboardAPI.getRegionalPerformance();
    expect(Array.isArray(result)).toBe(true);
  });

  it('getTopProducts calls /api/dashboard/top-products', async () => {
    mockFetch([
      { name: 'Product A', category: 'Diapers', brand: 'Brand X', revenue: 50000, units: 200, margin_pct: 28 },
    ]);
    const result = await dashboardAPI.getTopProducts();
    expect(Array.isArray(result)).toBe(true);
  });

  it('explainCard calls /api/dashboard/explain with POST', async () => {
    mockFetch({ explanation: 'Sales are growing.' });
    const result = await dashboardAPI.explainCard('Total GMV', { total_gmv: 1000 });
    expect(result).toEqual({ explanation: 'Sales are growing.' });
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/dashboard/explain',
      expect.objectContaining({ method: 'POST' }),
    );
  });
});

// ---------------------------------------------------------------------------
// chatAPI
// ---------------------------------------------------------------------------

describe('chatAPI', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('createSession calls POST /api/chat/sessions', async () => {
    mockFetch({ session_id: 'sess-123' });
    const result = await chatAPI.createSession();
    expect(result).toEqual({ session_id: 'sess-123' });
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/chat/sessions',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('getSessions calls GET /api/chat/sessions', async () => {
    mockFetch([{ id: 'sess-1', updated_at: '2026-03-01', first_message: 'Hello' }]);
    const result = await chatAPI.getSessions();
    expect(Array.isArray(result)).toBe(true);
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/chat/sessions',
      expect.objectContaining({ method: 'GET' }),
    );
  });

  it('getMessages calls GET /api/chat/sessions/:id/messages', async () => {
    mockFetch([{ id: 'msg-1', role: 'user', content: 'hi', metadata: '{}', created_at: '' }]);
    await chatAPI.getMessages('sess-123');
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/chat/sessions/sess-123/messages',
      expect.objectContaining({ method: 'GET' }),
    );
  });

  it('deleteSession calls DELETE /api/chat/sessions/:id', async () => {
    mockFetch({ message: 'deleted' });
    await chatAPI.deleteSession('sess-123');
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/chat/sessions/sess-123',
      expect.objectContaining({ method: 'DELETE' }),
    );
  });
});

// ---------------------------------------------------------------------------
// actionsAPI
// ---------------------------------------------------------------------------

describe('actionsAPI', () => {
  beforeEach(() => vi.restoreAllMocks());

  const sampleAction = {
    id: 'act-1',
    title: 'Restock Diapers',
    description: 'Low inventory',
    action_type: 'restock',
    category: 'Diapers',
    confidence_score: 0.9,
    status: 'pending',
    created_at: '2026-03-01T00:00:00Z',
    updated_at: '2026-03-01T00:00:00Z',
    product_name: 'Pampers',
    priority: 'high' as const,
    expected_impact: '+15% availability',
  };

  it('getActions calls GET /api/actions without status filter', async () => {
    mockFetch([sampleAction]);
    await actionsAPI.getActions();
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/actions',
      expect.objectContaining({ method: 'GET' }),
    );
  });

  it('getActions appends ?status= query param when provided', async () => {
    mockFetch([sampleAction]);
    await actionsAPI.getActions('pending');
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/actions?status=pending',
      expect.objectContaining({ method: 'GET' }),
    );
  });

  it('approveAction calls POST /api/actions/:id/approve', async () => {
    mockFetch({ message: 'approved' });
    await actionsAPI.approveAction('act-1');
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/actions/act-1/approve',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('rejectAction calls POST /api/actions/:id/reject', async () => {
    mockFetch({ message: 'rejected' });
    await actionsAPI.rejectAction('act-1');
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/actions/act-1/reject',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('revertAction calls POST /api/actions/:id/revert', async () => {
    mockFetch({ message: 'reverted' });
    await actionsAPI.revertAction('act-1');
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/actions/act-1/revert',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('addAction calls POST /api/actions with body', async () => {
    mockFetch({ message: 'created', id: 'act-2' });
    await actionsAPI.addAction({ title: 'New', action_type: 'manual_execution' });
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/actions',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('generateActions calls POST /api/actions/generate', async () => {
    mockFetch({ message: 'generated' });
    await actionsAPI.generateActions();
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/actions/generate',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('draftAction calls POST /api/actions/draft', async () => {
    mockFetch({ title: 'Draft', action_type: 'restock' });
    await actionsAPI.draftAction('restock diapers');
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/actions/draft',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('getComments calls GET /api/actions/:id/comments', async () => {
    mockFetch([{ id: 'c1', comment_text: 'ok', created_by: 'user', created_at: '' }]);
    await actionsAPI.getComments('act-1');
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/actions/act-1/comments',
      expect.objectContaining({ method: 'GET' }),
    );
  });

  it('addComment calls POST /api/actions/:id/comments', async () => {
    mockFetch({ message: 'added', id: 'c2' });
    await actionsAPI.addComment('act-1', 'Great idea');
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/actions/act-1/comments',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('updateAction calls PATCH /api/actions/:id', async () => {
    mockFetch({ message: 'updated' });
    await actionsAPI.updateAction('act-1', { title: 'Updated title' });
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/actions/act-1',
      expect.objectContaining({ method: 'PATCH' }),
    );
  });
});

// ---------------------------------------------------------------------------
// alertsAPI
// ---------------------------------------------------------------------------

describe('alertsAPI', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('getAlerts calls GET /api/alerts', async () => {
    mockFetch([
      { id: 'a1', title: 'Low Stock', severity: 'warning', category: 'Diapers', message: 'msg', created_at: '', acknowledged: false },
    ]);
    const result = await alertsAPI.getAlerts();
    expect(Array.isArray(result)).toBe(true);
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/alerts',
      expect.objectContaining({ method: 'GET' }),
    );
  });

  it('acknowledgeAlert calls POST /api/alerts/:id/acknowledge', async () => {
    mockFetch({ message: 'acknowledged' });
    await alertsAPI.acknowledgeAlert('a1');
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/alerts/a1/acknowledge',
      expect.anything(),
    );
  });

  it('addAlert calls POST /api/alerts with body', async () => {
    mockFetch({ message: 'created' });
    await alertsAPI.addAlert({ title: 'Test Alert', severity: 'info' });
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/alerts',
      expect.anything(),
    );
  });
});

// ---------------------------------------------------------------------------
// preferencesAPI
// ---------------------------------------------------------------------------

describe('preferencesAPI', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('get calls GET /api/config/preferences', async () => {
    mockFetch({ auto_approve: 'false', llm_temperature: '0.7' });
    const result = await preferencesAPI.get();
    expect(result).toEqual({ auto_approve: 'false', llm_temperature: '0.7' });
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/config/preferences',
      expect.objectContaining({ method: 'GET' }),
    );
  });

  it('save calls PUT /api/config/preferences with prefs body', async () => {
    mockFetch({ message: 'saved' });
    await preferencesAPI.save({ auto_approve: 'true' });
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/config/preferences',
      expect.objectContaining({ method: 'PUT' }),
    );
  });
});

// ---------------------------------------------------------------------------
// graphqlQuery
// ---------------------------------------------------------------------------

describe('graphqlQuery', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('returns data field from GraphQL response', async () => {
    mockFetch({ data: { users: [{ id: '1' }] } });
    const result = await graphqlQuery<{ users: { id: string }[] }>(
      'query { users { id } }',
    );
    expect(result).toEqual({ users: [{ id: '1' }] });
  });

  it('throws when GraphQL response contains errors', async () => {
    mockFetch({ errors: [{ message: 'Unauthorized' }] });
    await expect(
      graphqlQuery('query { secret }'),
    ).rejects.toThrow('Unauthorized');
  });

  it('throws when HTTP response is not ok', async () => {
    mockFetch({}, false, 500);
    await expect(graphqlQuery('query { x }')).rejects.toThrow('GraphQL error: 500');
  });

  it('sends query and variables in POST body', async () => {
    mockFetch({ data: {} });
    await graphqlQuery('query Foo($id: ID!) { node(id: $id) { id } }', { id: '42' });
    const [, options] = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    const body = JSON.parse(options.body);
    expect(body.variables).toEqual({ id: '42' });
  });
});

// ---------------------------------------------------------------------------
// streamChat
// ---------------------------------------------------------------------------

describe('streamChat', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('returns an AbortController', () => {
    // Simulate a fetch that never resolves so we can test the return value immediately
    global.fetch = vi.fn().mockReturnValue(new Promise(() => {}));
    const controller = streamChat(
      'Hello',
      null,
      undefined,
      vi.fn(),
      vi.fn(),
      vi.fn(),
    );
    expect(controller).toBeInstanceOf(AbortController);
  });

  it('calls onDone when SSE stream completes successfully', async () => {
    // Build a minimal ReadableStream that sends a "done" event
    const sseChunk = 'event: done\ndata: {}\n\n';
    const encoder = new TextEncoder();
    const stream = new ReadableStream({
      start(controller) {
        controller.enqueue(encoder.encode(sseChunk));
        controller.close();
      },
    });

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      body: stream,
    });

    const onDone = vi.fn();
    const onError = vi.fn();
    streamChat('hi', 'sess-1', undefined, vi.fn(), onDone, onError);

    // Give microtasks time to complete
    await new Promise((r) => setTimeout(r, 50));
    expect(onError).not.toHaveBeenCalled();
  });

  it('calls onError when fetch throws a non-abort error', async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error('Network failure'));
    const onError = vi.fn();
    streamChat('hi', null, undefined, vi.fn(), vi.fn(), onError);
    await new Promise((r) => setTimeout(r, 50));
    expect(onError).toHaveBeenCalledWith(expect.objectContaining({ message: 'Network failure' }));
  });
});
