const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export async function fetchAPI<T>(endpoint: string, options?: RequestInit): Promise<T> {
    const method = options?.method || 'POST';
    const reqOptions: RequestInit = {
        ...options,
        method,
        headers: {
            'Content-Type': 'application/json',
            ...options?.headers,
        },
    };

    if (method !== 'GET' && method !== 'HEAD') {
        reqOptions.body = options?.body || '{}';
    } else {
        delete reqOptions.body;
    }

    const res = await fetch(`${API_BASE}${endpoint}`, reqOptions);
    if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(err.error || `API error: ${res.status}`);
    }
    return res.json();
}

// Dashboard API
export interface KPIs {
    total_gmv: number;
    avg_margin_pct: number;
    total_units: number;
    active_skus: number;
    gmv_change_pct: number;
    margin_change_pct: number;
}

export interface SalesTrendPoint {
    month: string;
    revenue: number;
    margin: number;
    units: number;
}

export interface CategoryData {
    name: string;
    revenue: number;
    margin: number;
    units: number;
    sku_count: number;
}

export interface RegionData {
    name: string;
    revenue: number;
    margin: number;
    units: number;
    avg_discount: number;
}

export interface TopProduct {
    name: string;
    category: string;
    brand: string;
    revenue: number;
    units: number;
    margin_pct: number;
}

export const dashboardAPI = {
    getKPIs: () => fetchAPI<KPIs>('/api/dashboard/kpis'),
    getSalesTrend: () => fetchAPI<SalesTrendPoint[]>('/api/dashboard/sales-trend'),
    getCategoryBreakdown: () => fetchAPI<CategoryData[]>('/api/dashboard/category-breakdown'),
    getRegionalPerformance: () => fetchAPI<RegionData[]>('/api/dashboard/regional-performance'),
    getTopProducts: () => fetchAPI<TopProduct[]>('/api/dashboard/top-products'),
    explainCard: (cardType: string, cardData: unknown) =>
        fetchAPI<{ explanation: string }>('/api/dashboard/explain', {
            body: JSON.stringify({ card_type: cardType, card_data: cardData }),
        }),
};

// Chat Streaming API (SSE — kept for real-time streaming)
export interface ChatSSEEvent {
    event: string;
    data: Record<string, unknown>;
}

export function streamChat(
    message: string,
    sessionId: string | null,
    onEvent: (event: ChatSSEEvent) => void,
    onDone: () => void,
    onError: (err: Error) => void
): AbortController {
    const controller = new AbortController();

    fetch(`${API_BASE}/api/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message, session_id: sessionId || '' }),
        signal: controller.signal,
    })
        .then(async (res) => {
            if (!res.ok) throw new Error(`Chat error: ${res.status}`);
            const reader = res.body?.getReader();
            if (!reader) throw new Error('No reader');

            const decoder = new TextDecoder();
            let buffer = '';

            while (true) {
                const { done, value } = await reader.read();
                if (done) break;

                buffer += decoder.decode(value, { stream: true });
                const lines = buffer.split('\n');
                buffer = lines.pop() || '';

                let currentEvent = '';
                for (const line of lines) {
                    if (line.startsWith('event: ')) {
                        currentEvent = line.slice(7).trim();
                    } else if (line.startsWith('data: ') && currentEvent) {
                        try {
                            const data = JSON.parse(line.slice(6));
                            onEvent({ event: currentEvent, data });
                            if (currentEvent === 'done') {
                                onDone();
                                return;
                            }
                        } catch {
                            // Skip malformed JSON
                        }
                    }
                }
            }
            onDone();
        })
        .catch((err) => {
            if (err.name !== 'AbortError') onError(err);
        });

    return controller;
}

// Chat History API (REST — for session management)
export interface ChatSession {
    id: string;
    created_at: string;
    first_message: string;
}

export interface ChatHistoryMessage {
    id: string;
    role: string;
    content: string;
    metadata: string;
    created_at: string;
}

export const chatAPI = {
    getSessions: () => fetchAPI<ChatSession[]>('/api/chat/sessions', { method: 'GET' }),
    getMessages: (sessionId: string) =>
        fetchAPI<ChatHistoryMessage[]>(`/api/chat/sessions/${sessionId}/messages`, { method: 'GET' }),
};

// GraphQL API (for chat operations)
export async function graphqlQuery<T>(query: string, variables?: Record<string, unknown>): Promise<T> {
    const res = await fetch(`${API_BASE}/api/graphql`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query, variables }),
    });
    if (!res.ok) throw new Error(`GraphQL error: ${res.status}`);
    const json = await res.json();
    if (json.errors) throw new Error(json.errors[0].message);
    return json.data;
}

// Actions API
export interface Action {
    id: string;
    title: string;
    description: string;
    action_type: string;
    category: string;
    confidence_score: number;
    status: string;
    created_at: string;
    updated_at: string;
    product_name: string;
}

export interface ActionComment {
    id: string;
    comment_text: string;
    created_by: string;
    created_at: string;
}

export const actionsAPI = {
    getActions: (status?: string) =>
        fetchAPI<Action[]>(`/api/actions${status ? `?status=${status}` : ''}`, {
            method: 'GET',
        }),
    approveAction: (id: string) =>
        fetchAPI<{ message: string }>(`/api/actions/${id}/approve`, { method: 'POST', body: '{}' }),
    rejectAction: (id: string) =>
        fetchAPI<{ message: string }>(`/api/actions/${id}/reject`, { method: 'POST', body: '{}' }),
    revertAction: (id: string) =>
        fetchAPI<{ message: string }>(`/api/actions/${id}/revert`, { method: 'POST', body: '{}' }),
    addAction: (data: Partial<Action>) =>
        fetchAPI<{ message: string; id: string }>('/api/actions', {
            method: 'POST',
            body: JSON.stringify(data),
        }),
    generateActions: () => fetchAPI<{ message: string }>('/api/actions/generate', { method: 'POST' }),
    draftAction: (input: string) =>
        fetchAPI<Partial<Action>>('/api/actions/draft', {
            method: 'POST',
            body: JSON.stringify({ input }),
        }),
    getComments: (id: string) =>
        fetchAPI<ActionComment[]>(`/api/actions/${id}/comments`, { method: 'GET' }),
    addComment: (id: string, text: string) =>
        fetchAPI<{ message: string; id: string }>(`/api/actions/${id}/comments`, {
            method: 'POST',
            body: JSON.stringify({ comment_text: text }),
        }),
    updateAction: (id: string, data: { title?: string; description?: string }) =>
        fetchAPI<{ message: string }>(`/api/actions/${id}`, {
            method: 'PATCH',
            body: JSON.stringify(data),
        }),
};

// Alerts API
export interface Alert {
    id: string;
    title: string;
    severity: "critical" | "warning" | "info";
    category: string;
    message: string;
    created_at: string;
    acknowledged: boolean;
}

export const alertsAPI = {
    getAlerts: () => fetchAPI<Alert[]>('/api/alerts', { method: 'GET' }),
    acknowledgeAlert: (id: string) =>
        fetchAPI<{ message: string }>(`/api/alerts/${id}/acknowledge`, { body: '{}' }),
    addAlert: (data: Partial<Alert>) =>
        fetchAPI<{ message: string }>('/api/alerts', {
            body: JSON.stringify(data),
        }),
};

// Reports API
export const reportsAPI = {
    downloadReport: async () => {
        const res = await fetch(`${API_BASE}/api/reports/download`, { method: 'GET' });
        if (!res.ok) throw new Error(`Download error: ${res.status}`);
        const blob = await res.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = res.headers.get('Content-Disposition')?.split('filename=')[1] || 'report.csv';
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(url);
    },
};
