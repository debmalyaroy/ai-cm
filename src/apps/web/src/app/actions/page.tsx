"use client";

import { useState, useEffect } from "react";
import { actionsAPI, type Action } from "@/lib/api";
import CreateActionModal from "./CreateActionModal";

const actionTypeIcons: Record<string, string> = {
    price_match: "💲",
    restock: "📦",
    promotion: "🎯",
    delist: "🚫",
    manual_execution: "⚡",
};

const actionTypeColors: Record<string, string> = {
    price_match: "var(--color-warning)",
    restock: "var(--color-danger)",
    promotion: "var(--color-primary)",
    delist: "var(--color-text-secondary)",
    manual_execution: "var(--color-info)",
};

const statusBadge: Record<string, { className: string; label: string }> = {
    pending: { className: "badge badge-warning", label: "Pending" },
    approved: { className: "badge badge-success", label: "Approved" },
    rejected: { className: "badge badge-danger", label: "Rejected" },
};

type ViewMode = "grid" | "list" | "details";
type SortField = "newest" | "oldest" | "updated" | "status";

function ConfidenceMeter({ score }: { score: number }) {
    const pct = score * 100;
    const color = pct >= 80 ? "var(--color-success)" : pct >= 60 ? "var(--color-warning)" : "var(--color-danger)";
    return (
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <div style={{ flex: 1, height: 4, borderRadius: 2, background: "var(--color-bg)", overflow: "hidden" }}>
                <div style={{ height: "100%", width: `${pct}%`, borderRadius: 2, background: color, transition: "width 0.5s ease" }} />
            </div>
            <span style={{ fontSize: 12, fontWeight: 600, color, minWidth: 36, textAlign: "right" }}>
                {pct.toFixed(0)}%
            </span>
        </div>
    );
}

function ActionCard({ action, onClick, compact }: { action: Action; onClick: () => void; compact?: boolean }) {
    const badge = statusBadge[action.status] || statusBadge.pending;
    return (
        <div
            className="card animate-fade-in"
            style={{ padding: compact ? 14 : 20, cursor: "pointer", transition: "transform 0.2s, box-shadow 0.2s" }}
            onClick={onClick}
            onMouseEnter={(e) => { e.currentTarget.style.transform = "translateY(-2px)"; e.currentTarget.style.boxShadow = "var(--shadow-md)"; }}
            onMouseLeave={(e) => { e.currentTarget.style.transform = "none"; e.currentTarget.style.boxShadow = "var(--shadow-sm)"; }}
        >
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 10 }}>
                <div style={{ display: "flex", gap: 12, alignItems: "flex-start" }}>
                    <div style={{
                        width: 36, height: 36, borderRadius: 8,
                        background: `${actionTypeColors[action.action_type] || "var(--color-primary)"}15`,
                        display: "flex", alignItems: "center", justifyContent: "center", fontSize: 18, flexShrink: 0,
                    }}>
                        {actionTypeIcons[action.action_type] || "⚡"}
                    </div>
                    <div>
                        <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 2 }}>{action.title}</div>
                        <div style={{ fontSize: 11, color: "var(--color-text-secondary)" }}>
                            {action.action_type.replace(/_/g, " ").toUpperCase()} • Created {new Date(action.created_at).toLocaleString()}
                        </div>
                        {action.updated_at !== action.created_at && (
                            <div style={{ fontSize: 11, color: "var(--color-text-secondary)" }}>
                                Updated {new Date(action.updated_at).toLocaleString()}
                            </div>
                        )}
                    </div>
                </div>
                <span className={badge.className}>{badge.label}</span>
            </div>
            {!compact && (
                <p style={{ fontSize: 13, color: "var(--color-text-secondary)", lineHeight: 1.6, margin: "0 0 10px" }}>
                    {action.description}
                </p>
            )}
            <div>
                <div style={{ fontSize: 11, color: "var(--color-text-secondary)", marginBottom: 4 }}>Confidence</div>
                <ConfidenceMeter score={action.confidence_score} />
            </div>
        </div>
    );
}

function ActionListRow({
    action, onClick, sortField,
    onSortChange, showHeader,
}: {
    action: Action | null;
    onClick?: () => void;
    sortField: SortField;
    onSortChange?: (f: SortField) => void;
    showHeader?: boolean;
}) {
    const badge = action ? (statusBadge[action.status] || statusBadge.pending) : null;
    const colStyle: React.CSSProperties = { padding: "10px 12px", fontSize: 13, verticalAlign: "middle" };
    const headerColStyle: React.CSSProperties = {
        padding: "8px 12px", fontSize: 11, fontWeight: 600, color: "var(--color-text-secondary)",
        cursor: "pointer", userSelect: "none", textTransform: "uppercase", letterSpacing: "0.04em",
        whiteSpace: "nowrap",
    };

    const sortIndicator = (field: SortField) => sortField === field ? " ↕" : "";

    if (showHeader) {
        return (
            <tr style={{ borderBottom: "2px solid var(--color-border)" }}>
                <th style={headerColStyle}></th>
                <th style={headerColStyle} onClick={() => onSortChange?.("newest")}>Title{sortIndicator("newest")}</th>
                <th style={headerColStyle}>Type</th>
                <th style={headerColStyle} onClick={() => onSortChange?.("status")}>Status{sortIndicator("status")}</th>
                <th style={headerColStyle}>Confidence</th>
                <th style={{ ...headerColStyle }} onClick={() => onSortChange?.("newest")}>Created{sortIndicator("newest")}</th>
                <th style={{ ...headerColStyle }} onClick={() => onSortChange?.("updated")}>Updated{sortIndicator("updated")}</th>
            </tr>
        );
    }

    if (!action) return null;
    return (
        <tr
            style={{ borderBottom: "1px solid var(--color-border)", cursor: "pointer", transition: "background 0.15s" }}
            onClick={onClick}
            onMouseEnter={(e) => (e.currentTarget.style.background = "var(--color-surface-hover)")}
            onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
        >
            <td style={colStyle}>{actionTypeIcons[action.action_type] || "⚡"}</td>
            <td style={{ ...colStyle, fontWeight: 500 }}>{action.title}</td>
            <td style={colStyle}><span style={{ fontSize: 11, color: "var(--color-text-secondary)" }}>{action.action_type.replace(/_/g, " ").toUpperCase()}</span></td>
            <td style={colStyle}><span className={badge!.className}>{badge!.label}</span></td>
            <td style={{ ...colStyle, minWidth: 120 }}><ConfidenceMeter score={action.confidence_score} /></td>
            <td style={{ ...colStyle, color: "var(--color-text-secondary)", fontSize: 12, whiteSpace: "nowrap" }}>{new Date(action.created_at).toLocaleString()}</td>
            <td style={{ ...colStyle, color: "var(--color-text-secondary)", fontSize: 12, whiteSpace: "nowrap" }}>{new Date(action.updated_at).toLocaleString()}</td>
        </tr>
    );
}

function ActionDetailsModal({
    action, onClose, onApprove, onReject, onRevert, onUpdate,
}: {
    action: Action | null;
    onClose: () => void;
    onApprove: (id: string) => Promise<void>;
    onReject: (id: string) => Promise<void>;
    onRevert: (id: string) => Promise<void>;
    onUpdate: (id: string, data: { title: string; description: string }) => Promise<void>;
}) {
    const [comments, setComments] = useState<any[]>([]);
    const [newComment, setNewComment] = useState("");
    const [loadingComments, setLoadingComments] = useState(false);
    const [isProcessing, setIsProcessing] = useState(false);
    const [isEditing, setIsEditing] = useState(false);
    const [editTitle, setEditTitle] = useState("");
    const [editDescription, setEditDescription] = useState("");

    useEffect(() => {
        if (!action) return;
        setEditTitle(action.title);
        setEditDescription(action.description || "");
        setIsEditing(false);
        setLoadingComments(true);
        actionsAPI.getComments(action.id)
            .then(setComments)
            .catch(console.error)
            .finally(() => setLoadingComments(false));
    }, [action]);

    if (!action) return null;
    const badge = statusBadge[action.status] || statusBadge.pending;

    const handleAction = async (handler: (id: string) => Promise<void>) => {
        setIsProcessing(true);
        try { await handler(action.id); onClose(); } finally { setIsProcessing(false); }
    };

    const handleSaveEdit = async () => {
        setIsProcessing(true);
        try {
            await onUpdate(action.id, { title: editTitle, description: editDescription });
            setIsEditing(false);
        } catch (e) { console.error(e); } finally { setIsProcessing(false); }
    };

    const handlePostComment = async () => {
        if (!newComment.trim()) return;
        try {
            await actionsAPI.addComment(action.id, newComment);
            const data = await actionsAPI.getComments(action.id);
            setComments(data);
            setNewComment("");
        } catch (e) { console.error(e); }
    };

    return (
        <div onClick={onClose} style={{
            position: "fixed", top: 0, left: 0, right: 0, bottom: 0,
            background: "rgba(0,0,0,0.5)", zIndex: 1000,
            display: "flex", alignItems: "center", justifyContent: "center", padding: 20
        }}>
            <div className="card" onClick={e => e.stopPropagation()} style={{
                width: "100%", maxWidth: 600, maxHeight: "90vh",
                overflowY: "auto", padding: 24, display: "flex", flexDirection: "column", gap: 20
            }}>
                {/* Header */}
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
                    <div style={{ display: "flex", gap: 12, alignItems: "center" }}>
                        <div style={{ fontSize: 24 }}>{actionTypeIcons[action.action_type] || "⚡"}</div>
                        <div>
                            <h2 style={{ margin: 0, fontSize: 18 }}>{action.title}</h2>
                            <div style={{ fontSize: 12, color: "var(--color-text-secondary)", marginTop: 4 }}>
                                {action.action_type.replace(/_/g, " ").toUpperCase()}
                            </div>
                        </div>
                    </div>
                    <button className="btn btn-ghost" onClick={onClose} style={{ padding: "4px 8px" }}>✕</button>
                </div>

                {/* Timestamps + Status */}
                <div style={{ display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap" }}>
                    <span className={badge.className}>{badge.label}</span>
                    <span style={{ fontSize: 12, color: "var(--color-text-secondary)" }}>
                        Created: {new Date(action.created_at).toLocaleString()}
                    </span>
                    {action.updated_at !== action.created_at && (
                        <span style={{ fontSize: 12, color: "var(--color-text-secondary)" }}>
                            Updated: {new Date(action.updated_at).toLocaleString()}
                        </span>
                    )}
                </div>

                {/* Description */}
                <div>
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
                        <h3 style={{ fontSize: 14, margin: 0 }}>Details</h3>
                        {action.status === "pending" && !isEditing && (
                            <button className="btn btn-ghost" onClick={() => setIsEditing(true)} style={{ fontSize: 12, padding: "2px 10px" }}>
                                ✏️ Edit
                            </button>
                        )}
                    </div>
                    {isEditing ? (
                        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                            <input
                                value={editTitle}
                                onChange={(e) => setEditTitle(e.target.value)}
                                placeholder="Title"
                                style={{ padding: "8px 12px", borderRadius: 8, border: "1px solid var(--color-border)", background: "var(--color-bg)", color: "var(--color-text)", fontSize: 13 }}
                            />
                            <textarea
                                value={editDescription}
                                onChange={(e) => setEditDescription(e.target.value)}
                                rows={4}
                                placeholder="Description"
                                style={{ padding: "8px 12px", borderRadius: 8, border: "1px solid var(--color-border)", background: "var(--color-bg)", color: "var(--color-text)", resize: "vertical", fontSize: 13 }}
                            />
                            <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
                                <button className="btn btn-ghost" onClick={() => { setIsEditing(false); setEditTitle(action.title); setEditDescription(action.description || ""); }}>Cancel</button>
                                <button className="btn btn-primary" onClick={handleSaveEdit} disabled={isProcessing || !editTitle.trim()}>
                                    {isProcessing ? "Saving..." : "Save"}
                                </button>
                            </div>
                        </div>
                    ) : (
                        <p style={{ fontSize: 13, color: "var(--color-text-secondary)", lineHeight: 1.6, margin: 0 }}>
                            {action.description}
                        </p>
                    )}
                </div>

                <div>
                    <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 8 }}>Confidence Score</div>
                    <ConfidenceMeter score={action.confidence_score} />
                </div>

                {/* Actions */}
                <div style={{ display: "flex", gap: 12, paddingBottom: 20, borderBottom: "1px solid var(--color-border)" }}>
                    {action.status === "pending" && (
                        <>
                            <button className="btn btn-danger" onClick={() => handleAction(onReject)} disabled={isProcessing} style={{ flex: 1 }}>✕ Reject</button>
                            <button className="btn btn-success" onClick={() => handleAction(onApprove)} disabled={isProcessing} style={{ flex: 1 }}>✓ Approve</button>
                        </>
                    )}
                    {(action.status === "approved" || action.status === "rejected") && (
                        <button className="btn btn-warning" onClick={() => handleAction(onRevert)} disabled={isProcessing} style={{ flex: 1 }}>↩ Revert to Pending</button>
                    )}
                </div>

                {/* Comments */}
                <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
                    <h3 style={{ fontSize: 14, margin: 0 }}>Comments</h3>
                    {loadingComments ? (
                        <div style={{ fontSize: 12, color: "var(--color-text-secondary)" }}>Loading...</div>
                    ) : comments.length === 0 ? (
                        <div style={{ fontSize: 12, color: "var(--color-text-secondary)" }}>No comments yet.</div>
                    ) : (
                        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                            {comments.map(c => (
                                <div key={c.id} style={{ background: "var(--color-bg)", padding: 12, borderRadius: 8 }}>
                                    <div style={{ fontSize: 11, color: "var(--color-text-secondary)", marginBottom: 4 }}>
                                        {c.created_by} • {new Date(c.created_at).toLocaleString()}
                                    </div>
                                    <div style={{ fontSize: 13 }}>{c.comment_text}</div>
                                </div>
                            ))}
                        </div>
                    )}
                    <div style={{ display: "flex", gap: 8, marginTop: 8 }}>
                        <input
                            type="text"
                            placeholder="Add a comment..."
                            value={newComment}
                            onChange={(e) => setNewComment(e.target.value)}
                            onKeyDown={(e) => e.key === "Enter" && handlePostComment()}
                            style={{ flex: 1, padding: "8px 12px", borderRadius: 8, border: "1px solid var(--color-border)", background: "var(--color-bg)", color: "var(--color-text)" }}
                        />
                        <button className="btn btn-primary" onClick={handlePostComment} disabled={!newComment.trim()}>Post</button>
                    </div>
                </div>
            </div>
        </div>
    );
}

export default function ActionsPage() {
    const [actions, setActions] = useState<Action[]>([]);
    const [filter, setFilter] = useState<string>("");
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [selectedAction, setSelectedAction] = useState<Action | null>(null);
    const [showCreateModal, setShowCreateModal] = useState(false);
    const [sortBy, setSortBy] = useState<SortField>("updated");
    const [viewMode, setViewMode] = useState<ViewMode>("grid");

    const loadActions = async () => {
        try {
            setLoading(true);
            const data = await actionsAPI.getActions(filter);
            setActions(data);
        } catch (err) {
            setError(err instanceof Error ? err.message : "Failed to load actions");
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => { loadActions(); }, [filter]);

    const handleApprove = async (id: string) => {
        await actionsAPI.approveAction(id);
        setActions((prev) => prev.map((a) => (a.id === id ? { ...a, status: "approved", updated_at: new Date().toISOString() } : a)));
    };

    const handleReject = async (id: string) => {
        await actionsAPI.rejectAction(id);
        setActions((prev) => prev.map((a) => (a.id === id ? { ...a, status: "rejected", updated_at: new Date().toISOString() } : a)));
    };

    const handleRevert = async (id: string) => {
        await actionsAPI.revertAction(id);
        setActions((prev) => prev.map((a) => (a.id === id ? { ...a, status: "pending", updated_at: new Date().toISOString() } : a)));
    };

    const handleUpdate = async (id: string, data: { title: string; description: string }) => {
        await actionsAPI.updateAction(id, data);
        const now = new Date().toISOString();
        setActions((prev) => prev.map((a) => (a.id === id ? { ...a, ...data, updated_at: now } : a)));
        setSelectedAction((prev) => (prev ? { ...prev, ...data, updated_at: now } : null));
    };

    const filters = [
        { value: "", label: "All" },
        { value: "pending", label: "Pending" },
        { value: "approved", label: "Approved" },
        { value: "rejected", label: "Rejected" },
    ];

    const counts = {
        pending: actions.filter((a) => a.status === "pending").length,
        approved: actions.filter((a) => a.status === "approved").length,
        rejected: actions.filter((a) => a.status === "rejected").length,
    };

    const statusOrder: Record<string, number> = { pending: 0, approved: 1, rejected: 2 };
    const sortedActions = [...actions].sort((a, b) => {
        if (sortBy === "newest") return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
        if (sortBy === "oldest") return new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
        if (sortBy === "updated") return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
        return (statusOrder[a.status] ?? 3) - (statusOrder[b.status] ?? 3);
    });

    if (error) {
        return (
            <div style={{ padding: 40, textAlign: "center" }}>
                <div style={{ fontSize: 48, marginBottom: 16 }}>⚠️</div>
                <div style={{ fontSize: 18, fontWeight: 600, marginBottom: 8 }}>Connection Error</div>
                <div style={{ color: "var(--color-text-secondary)", marginBottom: 16 }}>{error}</div>
                <button className="btn btn-primary" onClick={() => window.location.reload()}>Retry</button>
            </div>
        );
    }

    return (
        <div>
            {/* Header */}
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 24 }}>
                <div>
                    <h1 style={{ fontSize: 24, fontWeight: 700, margin: 0, letterSpacing: "-0.02em" }}>Action Center</h1>
                    <p style={{ color: "var(--color-text-secondary)", margin: "4px 0 0", fontSize: 14 }}>
                        AI-recommended actions for your categories
                    </p>
                </div>
                <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                    {/* View mode toggle */}
                    <div style={{ display: "flex", gap: 2, background: "var(--color-bg)", borderRadius: 8, padding: 2, border: "1px solid var(--color-border)" }}>
                        {([
                            { mode: "grid" as ViewMode, icon: "⊞", label: "Grid" },
                            { mode: "list" as ViewMode, icon: "≡", label: "List" },
                            { mode: "details" as ViewMode, icon: "☰", label: "Details" },
                        ] as const).map(({ mode, icon, label }) => (
                            <button
                                key={mode}
                                title={label}
                                onClick={() => setViewMode(mode)}
                                style={{
                                    padding: "4px 10px", borderRadius: 6, border: "none", cursor: "pointer",
                                    background: viewMode === mode ? "var(--color-primary)" : "transparent",
                                    color: viewMode === mode ? "white" : "var(--color-text-secondary)",
                                    fontSize: 14, fontWeight: 600, transition: "all 0.15s",
                                }}
                            >
                                {icon}
                            </button>
                        ))}
                    </div>
                    <button className="btn btn-primary" onClick={() => setShowCreateModal(true)}>
                        ✨ Create New Action
                    </button>
                </div>
            </div>

            {/* Stats */}
            <div style={{ display: "flex", gap: 12, marginBottom: 20 }}>
                {[
                    { icon: "⏳", count: counts.pending, label: "Pending", color: undefined },
                    { icon: "✅", count: counts.approved, label: "Approved", color: "var(--color-success)" },
                    { icon: "❌", count: counts.rejected, label: "Rejected", color: "var(--color-danger)" },
                ].map(({ icon, count, label, color }) => (
                    <div key={label} className="card" style={{ padding: "12px 16px", display: "flex", alignItems: "center", gap: 8 }}>
                        <span style={{ fontSize: 20 }}>{icon}</span>
                        <div>
                            <div style={{ fontSize: 20, fontWeight: 700, color }}>{count}</div>
                            <div style={{ fontSize: 11, color: "var(--color-text-secondary)" }}>{label}</div>
                        </div>
                    </div>
                ))}
            </div>

            {/* Filters & Sort */}
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 20, flexWrap: "wrap", gap: 8 }}>
                <div style={{ display: "flex", gap: 8 }}>
                    {filters.map((f) => (
                        <button key={f.value} className={`btn ${filter === f.value ? "btn-primary" : "btn-ghost"}`} onClick={() => setFilter(f.value)} style={{ fontSize: 13 }}>
                            {f.label}
                        </button>
                    ))}
                </div>
                <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                    <span style={{ fontSize: 12, color: "var(--color-text-secondary)", marginRight: 2 }}>Sort:</span>
                    {(["updated", "newest", "oldest", "status"] as const).map((s) => (
                        <button
                            key={s}
                            className={`btn ${sortBy === s ? "btn-primary" : "btn-ghost"}`}
                            onClick={() => setSortBy(s)}
                            style={{ fontSize: 12, padding: "4px 10px" }}
                        >
                            {s === "updated" ? "↻ Updated" : s === "newest" ? "↓ Newest" : s === "oldest" ? "↑ Oldest" : "⬡ Status"}
                        </button>
                    ))}
                </div>
            </div>

            {/* Content */}
            {loading ? (
                <div style={{ textAlign: "center", padding: 40 }}>
                    <div style={{ animation: "thinking 1.4s ease-in-out infinite", fontSize: 32 }}>⚡</div>
                    <div style={{ color: "var(--color-text-secondary)", marginTop: 8 }}>Loading actions...</div>
                </div>
            ) : actions.length === 0 ? (
                <div className="card" style={{ padding: 40, textAlign: "center" }}>
                    <div style={{ fontSize: 40, marginBottom: 12 }}>📋</div>
                    <div style={{ fontWeight: 600, marginBottom: 4 }}>No actions found</div>
                    <div style={{ color: "var(--color-text-secondary)", fontSize: 13 }}>
                        Click &quot;Create New Action&quot; to draft an AI-powered action
                    </div>
                </div>
            ) : viewMode === "list" ? (
                <div className="card" style={{ overflow: "auto" }}>
                    <table style={{ width: "100%", borderCollapse: "collapse" }}>
                        <thead>
                            <ActionListRow action={null} sortField={sortBy} onSortChange={setSortBy} showHeader />
                        </thead>
                        <tbody>
                            {sortedActions.map((a) => (
                                <ActionListRow key={a.id} action={a} onClick={() => setSelectedAction(a)} sortField={sortBy} />
                            ))}
                        </tbody>
                    </table>
                </div>
            ) : viewMode === "details" ? (
                <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
                    {sortedActions.map((a) => (
                        <div key={a.id} onClick={() => setSelectedAction(a)}>
                            <ActionCard action={a} onClick={() => setSelectedAction(a)} compact={false} />
                        </div>
                    ))}
                </div>
            ) : (
                <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(340px, 1fr))", gap: 16 }}>
                    {sortedActions.map((a) => (
                        <ActionCard key={a.id} action={a} onClick={() => setSelectedAction(a)} compact />
                    ))}
                </div>
            )}

            {selectedAction && (
                <ActionDetailsModal
                    action={selectedAction}
                    onClose={() => setSelectedAction(null)}
                    onApprove={handleApprove}
                    onReject={handleReject}
                    onRevert={handleRevert}
                    onUpdate={handleUpdate}
                />
            )}

            {showCreateModal && (
                <CreateActionModal
                    onClose={() => setShowCreateModal(false)}
                    onSuccess={(newAction) => setActions([newAction, ...actions])}
                />
            )}
        </div>
    );
}
