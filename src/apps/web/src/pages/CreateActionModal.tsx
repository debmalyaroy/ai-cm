import { useState } from 'react';
import { actionsAPI, type Action } from '@/lib/api';

export default function CreateActionModal({
    onClose,
    onSuccess,
}: {
    onClose: () => void;
    onSuccess: (action: Action) => void;
}) {
    const [heading, setHeading] = useState('');
    const [details, setDetails] = useState('');
    const [isDrafting, setIsDrafting] = useState(false);
    const [isSaving, setIsSaving] = useState(false);
    const [draft, setDraft] = useState<Partial<Action> | null>(null);
    const [error, setError] = useState<string | null>(null);

    const handleDraft = async () => {
        if (!heading.trim()) return;
        setIsDrafting(true);
        setError(null);
        try {
            const input = details.trim()
                ? `${heading.trim()}\n\nContext: ${details.trim()}`
                : heading.trim();
            const result = await actionsAPI.draftAction(input);
            // Pre-fill title from heading if LLM didn't provide one
            if (!result.title) result.title = heading;
            setDraft(result);
        } catch (e) {
            setError(e instanceof Error ? e.message : 'Draft failed. Please try again.');
        } finally {
            setIsDrafting(false);
        }
    };

    const handleSave = async () => {
        if (!draft) return;
        setIsSaving(true);
        setError(null);
        try {
            const res = await actionsAPI.addAction({ ...draft, status: 'pending' });
            const now = new Date().toISOString();
            onSuccess({
                ...draft,
                id: res.id,
                status: 'pending',
                created_at: now,
                updated_at: now,
            } as Action);
            onClose();
        } catch (e) {
            setError(e instanceof Error ? e.message : 'Failed to create action.');
        } finally {
            setIsSaving(false);
        }
    };

    const confidencePct = draft?.confidence_score ? Math.round((draft.confidence_score as number) * 100) : null;
    const confidenceColor = confidencePct
        ? confidencePct >= 80 ? 'var(--color-success)' : confidencePct >= 60 ? 'var(--color-warning)' : 'var(--color-danger)'
        : 'var(--color-text-secondary)';

    return (
        <div onClick={onClose} style={{
            position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
            background: 'rgba(0,0,0,0.5)', zIndex: 1000,
            display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 20
        }}>
            <div className="card" onClick={e => e.stopPropagation()} style={{
                width: '100%', maxWidth: 600, padding: 24,
                display: 'flex', flexDirection: 'column', gap: 20, maxHeight: '90vh', overflowY: 'auto'
            }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div>
                        <h2 style={{ margin: 0, fontSize: 18 }}>Create New Action</h2>
                        <p style={{ margin: '4px 0 0', fontSize: 13, color: 'var(--color-text-secondary)' }}>
                            {draft ? 'Review the AI-drafted proposal and confirm.' : 'Describe the action and let AI draft a proposal.'}
                        </p>
                    </div>
                    <button className="btn btn-ghost" onClick={onClose} style={{ padding: '4px 8px', flexShrink: 0 }}>✕</button>
                </div>

                {!draft ? (
                    /* Step 1: Input form */
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
                        <div>
                            <label style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-secondary)', display: 'block', marginBottom: 6 }}>
                                HEADING <span style={{ color: 'var(--color-danger)' }}>*</span>
                            </label>
                            <input
                                value={heading}
                                onChange={(e) => setHeading(e.target.value)}
                                placeholder="e.g. Run 10% discount on Smart TVs in North region"
                                style={{
                                    width: '100%', padding: '10px 12px', borderRadius: 8,
                                    border: '1px solid var(--color-border)',
                                    background: 'var(--color-bg)', color: 'var(--color-text)', fontSize: 14,
                                    boxSizing: 'border-box'
                                }}
                                onKeyDown={(e) => e.key === 'Enter' && !e.shiftKey && heading.trim() && handleDraft()}
                            />
                        </div>
                        <div>
                            <label style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-secondary)', display: 'block', marginBottom: 6 }}>
                                DETAILS <span style={{ color: 'var(--color-text-secondary)', fontWeight: 400 }}>(optional — provide context for better AI suggestions)</span>
                            </label>
                            <textarea
                                value={details}
                                onChange={(e) => setDetails(e.target.value)}
                                placeholder="e.g. We have excess inventory (150+ days of supply). Competitors have cut prices by 8%. Suggest a promotion to clear stock before end of quarter."
                                rows={4}
                                style={{
                                    width: '100%', padding: '10px 12px', borderRadius: 8,
                                    border: '1px solid var(--color-border)',
                                    background: 'var(--color-bg)', color: 'var(--color-text)',
                                    resize: 'vertical', fontSize: 13, boxSizing: 'border-box'
                                }}
                            />
                        </div>
                        {error && <div style={{ color: 'var(--color-danger)', fontSize: 13, padding: '8px 12px', background: 'rgba(239,68,68,0.1)', borderRadius: 8 }}>{error}</div>}
                        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
                            <button className="btn btn-ghost" onClick={onClose}>Cancel</button>
                            <button
                                className="btn btn-primary"
                                onClick={handleDraft}
                                disabled={!heading.trim() || isDrafting}
                                style={{ minWidth: 140 }}
                            >
                                {isDrafting ? (
                                    <>
                                        <span style={{ display: 'inline-block', animation: 'thinking 1.4s ease-in-out infinite' }}>✨</span>
                                        {' '}Drafting...
                                    </>
                                ) : '✨ Draft with AI'}
                            </button>
                        </div>
                    </div>
                ) : (
                    /* Step 2: Preview & confirm */
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
                        {/* Confidence badge */}
                        {confidencePct !== null && (
                            <div style={{
                                display: 'flex', alignItems: 'center', gap: 10, padding: '10px 14px',
                                background: `${confidenceColor}15`, border: `1px solid ${confidenceColor}40`,
                                borderRadius: 8
                            }}>
                                <span style={{ fontSize: 18 }}>🤖</span>
                                <div>
                                    <div style={{ fontSize: 12, fontWeight: 600, color: confidenceColor }}>
                                        AI Confidence: {confidencePct}%
                                    </div>
                                    <div style={{ fontSize: 11, color: 'var(--color-text-secondary)' }}>
                                        Review and adjust the draft before confirming.
                                    </div>
                                </div>
                            </div>
                        )}

                        <div>
                            <label style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-secondary)', display: 'block', marginBottom: 4 }}>TITLE</label>
                            <input
                                value={draft.title || ''}
                                onChange={(e) => setDraft({ ...draft, title: e.target.value })}
                                style={{
                                    width: '100%', padding: '8px 12px', borderRadius: 8,
                                    border: '1px solid var(--color-border)',
                                    background: 'var(--color-bg)', color: 'var(--color-text)', boxSizing: 'border-box'
                                }}
                            />
                        </div>

                        <div style={{ display: 'flex', gap: 12 }}>
                            <div style={{ flex: 1 }}>
                                <label style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-secondary)', display: 'block', marginBottom: 4 }}>ACTION TYPE</label>
                                <select
                                    value={draft.action_type || 'manual_execution'}
                                    onChange={(e) => setDraft({ ...draft, action_type: e.target.value })}
                                    style={{
                                        width: '100%', padding: '8px 12px', borderRadius: 8,
                                        border: '1px solid var(--color-border)',
                                        background: 'var(--color-bg)', color: 'var(--color-text)', boxSizing: 'border-box'
                                    }}
                                >
                                    <option value="price_match">Price Match</option>
                                    <option value="restock">Restock</option>
                                    <option value="promotion">Promotion</option>
                                    <option value="delist">Delist</option>
                                    <option value="manual_execution">Manual Execution</option>
                                </select>
                            </div>
                            <div style={{ flex: 1 }}>
                                <label style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-secondary)', display: 'block', marginBottom: 4 }}>CATEGORY</label>
                                <input
                                    value={draft.category || ''}
                                    onChange={(e) => setDraft({ ...draft, category: e.target.value })}
                                    style={{
                                        width: '100%', padding: '8px 12px', borderRadius: 8,
                                        border: '1px solid var(--color-border)',
                                        background: 'var(--color-bg)', color: 'var(--color-text)', boxSizing: 'border-box'
                                    }}
                                />
                            </div>
                        </div>

                        <div>
                            <label style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-secondary)', display: 'block', marginBottom: 4 }}>DESCRIPTION</label>
                            <textarea
                                value={draft.description || ''}
                                onChange={(e) => setDraft({ ...draft, description: e.target.value })}
                                rows={5}
                                style={{
                                    width: '100%', padding: '8px 12px', borderRadius: 8,
                                    border: '1px solid var(--color-border)',
                                    background: 'var(--color-bg)', color: 'var(--color-text)',
                                    resize: 'vertical', fontSize: 13, boxSizing: 'border-box'
                                }}
                            />
                        </div>

                        {error && <div style={{ color: 'var(--color-danger)', fontSize: 13, padding: '8px 12px', background: 'rgba(239,68,68,0.1)', borderRadius: 8 }}>{error}</div>}

                        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 4 }}>
                            <button className="btn btn-ghost" onClick={() => { setDraft(null); setError(null); }}>← Edit Prompt</button>
                            <button className="btn btn-primary" onClick={handleSave} disabled={isSaving || !draft.title?.trim()} style={{ minWidth: 160 }}>
                                {isSaving ? 'Creating...' : '✓ Confirm & Create'}
                            </button>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}
