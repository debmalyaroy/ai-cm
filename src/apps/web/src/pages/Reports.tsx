import { useState } from "react";
import { reportsAPI } from "@/lib/api";

interface Report {
    id: string;
    title: string;
    type: string;
    period: string;
    generatedAt: string;
    status: "ready" | "generating" | "scheduled";
    description: string;
}

const mockReports: Report[] = [
    {
        id: "1",
        title: "Monthly Category Performance",
        type: "Performance",
        period: "January 2026",
        generatedAt: "2026-02-01 09:00",
        status: "ready",
        description: "Comprehensive analysis of revenue, margin, and volume trends across all baby care categories.",
    },
    {
        id: "2",
        title: "Competitor Price Intelligence",
        type: "Competitive",
        period: "Week 5, 2026",
        generatedAt: "2026-02-03 06:00",
        status: "ready",
        description: "Weekly competitor pricing comparison with recommendations for pricing adjustments.",
    },
    {
        id: "3",
        title: "Inventory Health Report",
        type: "Operations",
        period: "Q1 2026",
        generatedAt: "2026-02-05 10:00",
        status: "ready",
        description: "Stockout risk analysis and reorder point optimization for all active SKUs.",
    },
    {
        id: "4",
        title: "Regional Sales Deep Dive",
        type: "Analytics",
        period: "January 2026",
        generatedAt: "2026-02-02 12:00",
        status: "ready",
        description: "Region-by-region sales performance with market penetration insights.",
    },
    {
        id: "5",
        title: "February Forecast Report",
        type: "Forecast",
        period: "February 2026",
        generatedAt: "",
        status: "generating",
        description: "AI-generated demand forecast with confidence intervals and seasonal adjustments.",
    },
    {
        id: "6",
        title: "Quarterly Business Review",
        type: "Executive",
        period: "Q1 2026",
        generatedAt: "",
        status: "scheduled",
        description: "Executive summary for quarterly business review with key metrics and strategic recommendations.",
    },
];

const typeColors: Record<string, string> = {
    Performance: "#3b82f6",
    Competitive: "#ef4444",
    Operations: "#f59e0b",
    Analytics: "var(--color-primary)",
    Forecast: "#22c55e",
    Executive: "#ec4899",
};

const statusLabel: Record<string, { text: string; color: string; bg: string }> = {
    ready: { text: "Ready", color: "#22c55e", bg: "#22c55e20" },
    generating: { text: "Generating...", color: "#f59e0b", bg: "#f59e0b20" },
    scheduled: { text: "Scheduled", color: "#3b82f6", bg: "#3b82f620" },
};

export default function ReportsPage() {
    const [typeFilter, setTypeFilter] = useState<string>("all");
    const [isDownloading, setIsDownloading] = useState(false);
    const types = ["all", ...Array.from(new Set(mockReports.map((r) => r.type)))];
    const filtered = typeFilter === "all" ? mockReports : mockReports.filter((r) => r.type === typeFilter);

    const handleDownload = async () => {
        setIsDownloading(true);
        try {
            await reportsAPI.downloadReport();
        } catch (err) {
            console.error('Download failed:', err);
        } finally {
            setIsDownloading(false);
        }
    };

    return (
        <div style={{ maxWidth: 960, margin: "0 auto" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 24 }}>
                <div>
                    <h1 style={{ fontSize: 24, fontWeight: 700, marginBottom: 4 }}>📋 Reports</h1>
                    <p style={{ fontSize: 14, color: "var(--color-text-secondary)" }}>
                        AI-generated reports and analytics
                    </p>
                </div>
                <div style={{ display: "flex", gap: 12 }}>
                    <button
                        onClick={handleDownload}
                        disabled={isDownloading}
                        style={{
                            padding: "8px 20px",
                            borderRadius: 8,
                            background: "var(--color-surface)",
                            color: "var(--color-primary)",
                            border: "1px solid var(--color-border)",
                            fontSize: 14,
                            fontWeight: 600,
                            cursor: "pointer",
                        }}
                    >
                        {isDownloading ? "Downloading..." : "📥 Download CSV"}
                    </button>
                    <button
                        style={{
                            padding: "8px 20px",
                            borderRadius: 8,
                            background: "var(--color-gradient-gold)",
                            color: "white",
                            border: "none",
                            fontSize: 14,
                            fontWeight: 600,
                            cursor: "pointer",
                        }}
                    >
                        + Generate Report
                    </button>
                </div>
            </div>

            <div style={{ display: "flex", gap: 8, marginBottom: 24, flexWrap: "wrap" }}>
                {types.map((t) => (
                    <button
                        key={t}
                        onClick={() => setTypeFilter(t)}
                        style={{
                            padding: "6px 16px",
                            borderRadius: 20,
                            border: typeFilter === t ? "2px solid var(--color-primary)" : "1px solid var(--color-border)",
                            background: typeFilter === t ? "var(--color-primary-glow)" : "var(--color-surface)",
                            color: typeFilter === t ? "var(--color-primary)" : "var(--color-text-secondary)",
                            fontSize: 13,
                            fontWeight: typeFilter === t ? 600 : 400,
                            cursor: "pointer",
                        }}
                    >
                        {t}
                    </button>
                ))}
            </div>

            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(420px, 1fr))", gap: 16 }}>
                {filtered.map((report) => {
                    const st = statusLabel[report.status];
                    return (
                        <div
                            key={report.id}
                            style={{
                                background: "var(--color-surface)",
                                border: "1px solid var(--color-border)",
                                borderRadius: 12,
                                padding: 20,
                                display: "flex",
                                flexDirection: "column",
                                gap: 12,
                            }}
                        >
                            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                                <span
                                    style={{
                                        fontSize: 11,
                                        padding: "2px 8px",
                                        borderRadius: 4,
                                        background: (typeColors[report.type] || "#666") + "20",
                                        color: typeColors[report.type] || "#666",
                                        fontWeight: 600,
                                    }}
                                >
                                    {report.type}
                                </span>
                                <span style={{ fontSize: 11, padding: "2px 8px", borderRadius: 4, background: st.bg, color: st.color, fontWeight: 600 }}>
                                    {st.text}
                                </span>
                            </div>
                            <h3 style={{ fontSize: 16, fontWeight: 600 }}>{report.title}</h3>
                            <p style={{ fontSize: 13, color: "var(--color-text-secondary)", lineHeight: 1.5 }}>
                                {report.description}
                            </p>
                            <div style={{ display: "flex", justifyContent: "space-between", fontSize: 12, color: "var(--color-text-secondary)" }}>
                                <span>📅 {report.period}</span>
                                {report.generatedAt && <span>{report.generatedAt}</span>}
                            </div>
                            {report.status === "ready" && (
                                <button
                                    onClick={handleDownload}
                                    disabled={isDownloading}
                                    style={{
                                        padding: "8px 16px",
                                        borderRadius: 6,
                                        border: "1px solid var(--color-border)",
                                        background: "var(--color-surface)",
                                        color: "var(--color-primary)",
                                        fontSize: 13,
                                        fontWeight: 500,
                                        cursor: "pointer",
                                    }}
                                >
                                    {isDownloading ? "Downloading..." : "📥 Download Report"}
                                </button>
                            )}
                        </div>
                    );
                })}
            </div>
        </div>
    );
}
