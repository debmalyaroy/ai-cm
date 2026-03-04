"use client";

import { useState, useEffect } from "react";
import { actionsAPI, type Action } from "@/lib/api";

const actionTypeIcons: Record<string, string> = {
    price_match: "💲",
    restock: "📦",
    promotion: "🎯",
    delist: "🚫",
};

const actionTypeColors: Record<string, string> = {
    price_match: "var(--color-warning)",
    restock: "var(--color-danger)",
    promotion: "var(--color-primary)",
    delist: "var(--color-text-secondary)",
};

const statusBadge: Record<string, { className: string; label: string }> = {
    pending: { className: "badge badge-warning", label: "Pending" },
    approved: { className: "badge badge-success", label: "Approved" },
    rejected: { className: "badge badge-danger", label: "Rejected" },
};

function ConfidenceMeter({ score }: { score: number }) {
    const pct = score * 100;
    const color = pct >= 80 ? "var(--color-success)" : pct >= 60 ? "var(--color-warning)" : "var(--color-danger)";
    return (
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <div style={{ flex: 1, height: 4, borderRadius: 2, background: "var(--color-bg)", overflow: "hidden" }}>
                <div
                    style={{
                        height: "100%",
                        width: `${pct}%`,
                        borderRadius: 2,
                        background: color,
                        transition: "width 0.5s ease",
                    }}
                />
            </div>
            <span style={{ fontSize: 12, fontWeight: 600, color, minWidth: 36, textAlign: "right" }}>
                {pct.toFixed(0)}%
            </span>
        </div>
    );
}

function ActionCard({
    action,
    onApprove,
    onReject,
}: {
    action: Action;
    onApprove: () => void;
    onReject: () => void;
}) {
    const [isProcessing, setIsProcessing] = useState(false);
    const badge = statusBadge[action.status] || statusBadge.pending;

    const handleAction = async (handler: () => void) => {
        setIsProcessing(true);
        try {
            handler();
        } finally {
            setIsProcessing(false);
        }
    };

    return (
        <div className="card animate-fade-in" style={{ padding: 20 }}>
            {/* Header */}
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 12 }}>
                <div style={{ display: "flex", gap: 12, alignItems: "flex-start" }}>
                    <div
                        style={{
                            width: 40,
                            height: 40,
                            borderRadius: 10,
                            background: `${actionTypeColors[action.action_type]}15`,
                            display: "flex",
                            alignItems: "center",
                            justifyContent: "center",
                            fontSize: 20,
                            flexShrink: 0,
                        }}
                    >
                        {actionTypeIcons[action.action_type] || "⚡"}
                    </div>
                    <div>
                        <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 2 }}>{action.title}</div>
                        <div style={{ fontSize: 11, color: "var(--color-text-secondary)" }}>
                            {action.action_type.replace("_", " ").toUpperCase()} • {new Date(action.created_at).toLocaleDateString()}
                        </div>
                    </div>
                </div>
                <span className={badge.className}>{badge.label}</span>
            </div>

            {/* Description */}
            <p style={{ fontSize: 13, color: "var(--color-text-secondary)", lineHeight: 1.6, margin: "0 0 12px" }}>
                {action.description}
            </p>

            {/* Confidence */}
            <div style={{ marginBottom: 16 }}>
                <div style={{ fontSize: 11, color: "var(--color-text-secondary)", marginBottom: 4 }}>Confidence Score</div>
                <ConfidenceMeter score={action.confidence_score} />
            </div>

            {/* Actions */}
            {action.status === "pending" && (
                <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
                    <button
                        className="btn btn-danger"
                        onClick={() => handleAction(onReject)}
                        disabled={isProcessing}
                        style={{ fontSize: 13 }}
                    >
                        ✕ Reject
                    </button>
                    <button
                        className="btn btn-success"
                        onClick={() => handleAction(onApprove)}
                        disabled={isProcessing}
                        style={{ fontSize: 13 }}
                    >
                        ✓ Approve
                    </button>
                </div>
            )}
        </div>
    );
}

export default function ActionsPage() {
    const [actions, setActions] = useState<Action[]>([]);
    const [filter, setFilter] = useState<string>("");
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

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

    useEffect(() => {
        loadActions();
    }, [filter]);

    const handleApprove = async (id: string) => {
        try {
            await actionsAPI.approveAction(id);
            setActions((prev) => prev.map((a) => (a.id === id ? { ...a, status: "approved" } : a)));
        } catch (err) {
            // Silent fail to avoid leaking internal error details to client console
        }
    };

    const handleReject = async (id: string) => {
        try {
            await actionsAPI.rejectAction(id);
            setActions((prev) => prev.map((a) => (a.id === id ? { ...a, status: "rejected" } : a)));
        } catch (err) {
            // Silent fail to avoid leaking internal error details to client console
        }
    };

    const handleGenerate = async () => {
        try {
            await actionsAPI.generateActions();
            loadActions();
        } catch (err) {
            // Silent fail to avoid leaking internal error details to client console
        }
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
                    <h1 style={{ fontSize: 24, fontWeight: 700, margin: 0, letterSpacing: "-0.02em" }}>
                        Action Center
                    </h1>
                    <p style={{ color: "var(--color-text-secondary)", margin: "4px 0 0", fontSize: 14 }}>
                        AI-recommended actions for your categories
                    </p>
                </div>
                <button className="btn btn-primary" onClick={handleGenerate}>
                    🔄 Generate New
                </button>
            </div>

            {/* Stats */}
            <div style={{ display: "flex", gap: 12, marginBottom: 20 }}>
                <div className="card" style={{ padding: "12px 16px", display: "flex", alignItems: "center", gap: 8 }}>
                    <span style={{ fontSize: 20 }}>⏳</span>
                    <div>
                        <div style={{ fontSize: 20, fontWeight: 700 }}>{counts.pending}</div>
                        <div style={{ fontSize: 11, color: "var(--color-text-secondary)" }}>Pending</div>
                    </div>
                </div>
                <div className="card" style={{ padding: "12px 16px", display: "flex", alignItems: "center", gap: 8 }}>
                    <span style={{ fontSize: 20 }}>✅</span>
                    <div>
                        <div style={{ fontSize: 20, fontWeight: 700, color: "var(--color-success)" }}>{counts.approved}</div>
                        <div style={{ fontSize: 11, color: "var(--color-text-secondary)" }}>Approved</div>
                    </div>
                </div>
                <div className="card" style={{ padding: "12px 16px", display: "flex", alignItems: "center", gap: 8 }}>
                    <span style={{ fontSize: 20 }}>❌</span>
                    <div>
                        <div style={{ fontSize: 20, fontWeight: 700, color: "var(--color-danger)" }}>{counts.rejected}</div>
                        <div style={{ fontSize: 11, color: "var(--color-text-secondary)" }}>Rejected</div>
                    </div>
                </div>
            </div>

            {/* Filters */}
            <div style={{ display: "flex", gap: 8, marginBottom: 20 }}>
                {filters.map((f) => (
                    <button
                        key={f.value}
                        className={`btn ${filter === f.value ? "btn-primary" : "btn-ghost"}`}
                        onClick={() => setFilter(f.value)}
                        style={{ fontSize: 13 }}
                    >
                        {f.label}
                    </button>
                ))}
            </div>

            {/* Action Cards */}
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
                        Click &quot;Generate New&quot; to create AI-powered recommendations
                    </div>
                </div>
            ) : (
                <div style={{ display: "grid", gridTemplateColumns: "repeat(2, 1fr)", gap: 16 }}>
                    {actions.map((a) => (
                        <ActionCard
                            key={a.id}
                            action={a}
                            onApprove={() => handleApprove(a.id)}
                            onReject={() => handleReject(a.id)}
                        />
                    ))}
                </div>
            )}
        </div>
    );
}
