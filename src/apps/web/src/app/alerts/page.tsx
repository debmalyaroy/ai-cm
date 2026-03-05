"use client";

import { useState, useEffect } from "react";
import { alertsAPI, type Alert } from "@/lib/api";

const severityConfig: Record<string, { color: string; bg: string; icon: string }> = {
    critical: { color: "#ef4444", bg: "#ef444420", icon: "🔴" },
    high: { color: "#ef4444", bg: "#ef444420", icon: "🔴" },
    warning: { color: "#f59e0b", bg: "#f59e0b20", icon: "🟡" },
    medium: { color: "#f59e0b", bg: "#f59e0b20", icon: "🟡" },
    info: { color: "#3b82f6", bg: "#3b82f620", icon: "🔵" },
    low: { color: "#3b82f6", bg: "#3b82f620", icon: "🔵" },
};

export default function AlertsPage() {
    const [alerts, setAlerts] = useState<Alert[]>([]);
    const [filter, setFilter] = useState<string>("all");
    const [loading, setLoading] = useState(true);

    const loadAlerts = async () => {
        try {
            setLoading(true);
            const data = await alertsAPI.getAlerts();
            setAlerts(data);
        } catch (err) {
            // Silent fail to avoid leaking internal error details to client console
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        loadAlerts();
    }, []);

    const filtered = filter === "all"
        ? alerts
        : filter === "unacknowledged"
            ? alerts.filter((a) => !a.acknowledged)
            : alerts.filter((a) => a.severity === filter);

    const acknowledge = async (id: string) => {
        try {
            await alertsAPI.acknowledgeAlert(id);
            setAlerts((prev) => prev.map((a) => (a.id === id ? { ...a, acknowledged: true } : a)));
        } catch (err) {
            // Silent fail to avoid leaking internal error details to client console
        }
    };

    const unackCount = alerts.filter((a) => !a.acknowledged).length;

    return (
        <div style={{ maxWidth: 960, margin: "0 auto" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 24 }}>
                <div>
                    <h1 style={{ fontSize: 24, fontWeight: 700, marginBottom: 4 }}>
                        🔔 Alerts & Notifications
                        {unackCount > 0 && (
                            <span
                                style={{
                                    marginLeft: 10,
                                    fontSize: 14,
                                    padding: "2px 10px",
                                    borderRadius: 12,
                                    background: "#ef444420",
                                    color: "#ef4444",
                                    fontWeight: 600,
                                }}
                            >
                                {unackCount} new
                            </span>
                        )}
                    </h1>
                    <p style={{ fontSize: 14, color: "var(--color-text-secondary)" }}>
                        Real-time alerts from Watchdog Agent monitoring
                    </p>
                </div>
            </div>

            <div style={{ display: "flex", gap: 8, marginBottom: 24, flexWrap: "wrap" }}>
                {["all", "unacknowledged", "critical", "warning", "info"].map((f) => (
                    <button
                        key={f}
                        onClick={() => setFilter(f)}
                        style={{
                            padding: "6px 16px",
                            borderRadius: 20,
                            border: filter === f ? "2px solid var(--color-primary)" : "1px solid var(--color-border)",
                            background: filter === f ? "var(--color-primary-glow)" : "var(--color-surface)",
                            color: filter === f ? "var(--color-primary)" : "var(--color-text-secondary)",
                            fontSize: 13,
                            fontWeight: filter === f ? 600 : 400,
                            cursor: "pointer",
                            textTransform: "capitalize",
                        }}
                    >
                        {f}
                    </button>
                ))}
            </div>

            <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
                {loading ? (
                    <div style={{ textAlign: "center", padding: 40 }}>
                        <div style={{ animation: "thinking 1.4s ease-in-out infinite", fontSize: 32 }}>🔔</div>
                        <div style={{ color: "var(--color-text-secondary)", marginTop: 8 }}>Loading alerts...</div>
                    </div>
                ) : filtered.length === 0 ? (
                    <div className="card" style={{ padding: 40, textAlign: "center" }}>
                        <div style={{ fontSize: 40, marginBottom: 12 }}>🔔</div>
                        <div style={{ fontWeight: 600, marginBottom: 4 }}>No alerts found</div>
                        <div style={{ color: "var(--color-text-secondary)", fontSize: 13 }}>
                            Watchdog hasn&apos;t detected any anomalies recently.
                        </div>
                    </div>
                ) : filtered.map((alert) => {
                    const sev = severityConfig[alert.severity] ?? severityConfig['info'];
                    return (
                        <div
                            key={alert.id}
                            style={{
                                background: "var(--color-surface)",
                                borderTop: `1px solid ${alert.acknowledged ? "var(--color-border)" : sev.color + "40"}`,
                                borderRight: `1px solid ${alert.acknowledged ? "var(--color-border)" : sev.color + "40"}`,
                                borderBottom: `1px solid ${alert.acknowledged ? "var(--color-border)" : sev.color + "40"}`,
                                borderLeft: `4px solid ${sev.color}`,
                                borderRadius: 12,
                                padding: 20,
                                opacity: alert.acknowledged ? 0.7 : 1,
                                transition: "all 0.2s ease",
                            }}
                        >
                            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 8 }}>
                                <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                                    <span>{sev.icon}</span>
                                    <span style={{ fontSize: 11, padding: "2px 8px", borderRadius: 4, background: sev.bg, color: sev.color, fontWeight: 600, textTransform: "uppercase" }}>
                                        {alert.severity}
                                    </span>
                                    <span style={{ fontSize: 11, padding: "2px 8px", borderRadius: 4, background: "var(--color-bg)", color: "var(--color-text-secondary)", fontWeight: 500 }}>
                                        {alert.category}
                                    </span>
                                </div>
                                <span style={{ fontSize: 12, color: "var(--color-text-secondary)" }}>{new Date(alert.created_at).toLocaleString()}</span>
                            </div>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6 }}>{alert.title}</h3>
                            <p style={{ fontSize: 13, color: "var(--color-text-secondary)", lineHeight: 1.5, marginBottom: 12 }}>
                                {alert.message}
                            </p>
                            {!alert.acknowledged && (
                                <button
                                    onClick={() => acknowledge(alert.id)}
                                    style={{
                                        padding: "6px 14px",
                                        borderRadius: 6,
                                        border: "1px solid var(--color-border)",
                                        background: "var(--color-surface)",
                                        color: "var(--color-text)",
                                        fontSize: 12,
                                        fontWeight: 500,
                                        cursor: "pointer",
                                    }}
                                >
                                    ✓ Acknowledge
                                </button>
                            )}
                        </div>
                    );
                })}
            </div>
        </div>
    );
}
