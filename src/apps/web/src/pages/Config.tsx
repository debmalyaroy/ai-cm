import { useState } from "react";

interface ConfigSection {
    title: string;
    description: string;
    fields: ConfigField[];
}

interface ConfigField {
    key: string;
    label: string;
    type: "text" | "select" | "toggle" | "number";
    value: string | number | boolean;
    options?: string[];
    description?: string;
}

const initialConfig: ConfigSection[] = [
    {
        title: "AI Agent Settings",
        description: "Configure AI agent behavior and thresholds",
        fields: [
            { key: "llm_provider", label: "LLM Provider", type: "select", value: "gemini", options: ["gemini", "openai"], description: "AI model provider for agent responses" },
            { key: "confidence_threshold", label: "Confidence Threshold", type: "number", value: 0.7, description: "Minimum confidence for auto-actions (0.0-1.0)" },
            { key: "auto_approve", label: "Auto-approve High Confidence Actions", type: "toggle", value: false, description: "Automatically approve actions above confidence threshold" },
            { key: "max_retries", label: "Max SQL Retries", type: "number", value: 3, description: "Number of retry attempts for failed SQL generation" },
        ],
    },
    {
        title: "Watchdog Configuration",
        description: "Alert thresholds for anomaly detection",
        fields: [
            { key: "price_drop_critical", label: "Price Drop Critical (%)", type: "number", value: 20, description: "Price drop percentage to trigger critical alert" },
            { key: "price_drop_warning", label: "Price Drop Warning (%)", type: "number", value: 15, description: "Price drop percentage to trigger warning alert" },
            { key: "stockout_days", label: "Stockout Risk Days", type: "number", value: 7, description: "Days of supply below which to flag stockout risk" },
            { key: "excess_inventory_days", label: "Excess Inventory Days", type: "number", value: 60, description: "Days of supply above which to flag excess inventory" },
        ],
    },
    {
        title: "Notification Preferences",
        description: "Control how and when you receive notifications",
        fields: [
            { key: "email_alerts", label: "Email Alerts", type: "toggle", value: true, description: "Send critical alerts via email" },
            { key: "dashboard_refresh", label: "Dashboard Auto-Refresh (sec)", type: "number", value: 60, description: "Auto-refresh interval for dashboard data" },
            { key: "alert_sound", label: "Alert Sound", type: "toggle", value: false, description: "Play sound for new alerts" },
        ],
    },
    {
        title: "Data & Integration",
        description: "Data source and integration settings",
        fields: [
            { key: "db_host", label: "Database Host", type: "text", value: "localhost:5432", description: "PostgreSQL connection host" },
            { key: "db_name", label: "Database Name", type: "text", value: "aicm", description: "Database name" },
            { key: "api_url", label: "Backend API URL", type: "text", value: "http://localhost:8080", description: "Backend API endpoint" },
        ],
    },
];

export default function ConfigPage() {
    const [config, setConfig] = useState(initialConfig);
    const [saved, setSaved] = useState(false);

    const handleChange = (sectionIdx: number, fieldIdx: number, newValue: string | number | boolean) => {
        const updated = [...config];
        updated[sectionIdx] = {
            ...updated[sectionIdx],
            fields: updated[sectionIdx].fields.map((f, i) => (i === fieldIdx ? { ...f, value: newValue } : f)),
        };
        setConfig(updated);
        setSaved(false);
    };

    const handleSave = () => {
        setSaved(true);
        setTimeout(() => setSaved(false), 3000);
    };

    return (
        <div style={{ maxWidth: 720, margin: "0 auto" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 24 }}>
                <div>
                    <h1 style={{ fontSize: 24, fontWeight: 700, marginBottom: 4 }}>⚙️ Configuration</h1>
                    <p style={{ fontSize: 14, color: "var(--color-text-secondary)" }}>
                        System configuration and preferences
                    </p>
                </div>
                <button
                    onClick={handleSave}
                    style={{
                        padding: "8px 24px",
                        borderRadius: 8,
                        background: saved
                            ? "linear-gradient(135deg, #22c55e 0%, #16a34a 100%)"
                            : "var(--color-gradient-gold)",
                        color: "white",
                        border: "none",
                        fontSize: 14,
                        fontWeight: 600,
                        cursor: "pointer",
                        transition: "all 0.2s ease",
                    }}
                >
                    {saved ? "✓ Saved" : "Save Changes"}
                </button>
            </div>

            <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
                {config.map((section, sIdx) => (
                    <div
                        key={section.title}
                        style={{
                            background: "var(--color-surface)",
                            border: "1px solid var(--color-border)",
                            borderRadius: 12,
                            padding: 24,
                        }}
                    >
                        <h2 style={{ fontSize: 18, fontWeight: 600, marginBottom: 4 }}>{section.title}</h2>
                        <p style={{ fontSize: 13, color: "var(--color-text-secondary)", marginBottom: 20 }}>
                            {section.description}
                        </p>
                        <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
                            {section.fields.map((field, fIdx) => (
                                <div
                                    key={field.key}
                                    style={{
                                        display: "flex",
                                        justifyContent: "space-between",
                                        alignItems: "center",
                                        padding: "8px 0",
                                        borderBottom: fIdx < section.fields.length - 1 ? "1px solid var(--color-border)" : "none",
                                    }}
                                >
                                    <div style={{ flex: 1 }}>
                                        <div style={{ fontSize: 14, fontWeight: 500, marginBottom: 2 }}>{field.label}</div>
                                        {field.description && (
                                            <div style={{ fontSize: 12, color: "var(--color-text-secondary)" }}>{field.description}</div>
                                        )}
                                    </div>
                                    <div style={{ minWidth: 180, textAlign: "right" }}>
                                        {field.type === "toggle" ? (
                                            <button
                                                onClick={() => handleChange(sIdx, fIdx, !field.value)}
                                                style={{
                                                    width: 48,
                                                    height: 26,
                                                    borderRadius: 13,
                                                    border: "none",
                                                    background: field.value ? "var(--color-primary)" : "var(--color-border)",
                                                    position: "relative",
                                                    cursor: "pointer",
                                                    transition: "background 0.2s ease",
                                                }}
                                            >
                                                <div
                                                    style={{
                                                        width: 20,
                                                        height: 20,
                                                        borderRadius: "50%",
                                                        background: "white",
                                                        position: "absolute",
                                                        top: 3,
                                                        left: field.value ? 25 : 3,
                                                        transition: "left 0.2s ease",
                                                    }}
                                                />
                                            </button>
                                        ) : field.type === "select" ? (
                                            <select
                                                value={String(field.value)}
                                                onChange={(e) => handleChange(sIdx, fIdx, e.target.value)}
                                                style={{
                                                    padding: "6px 12px",
                                                    borderRadius: 6,
                                                    border: "1px solid var(--color-border)",
                                                    background: "var(--color-bg)",
                                                    color: "var(--color-text)",
                                                    fontSize: 13,
                                                }}
                                            >
                                                {field.options?.map((opt) => (
                                                    <option key={opt} value={opt}>{opt}</option>
                                                ))}
                                            </select>
                                        ) : (
                                            <input
                                                type={field.type}
                                                value={String(field.value)}
                                                onChange={(e) => handleChange(sIdx, fIdx, field.type === "number" ? Number(e.target.value) : e.target.value)}
                                                style={{
                                                    padding: "6px 12px",
                                                    borderRadius: 6,
                                                    border: "1px solid var(--color-border)",
                                                    background: "var(--color-bg)",
                                                    color: "var(--color-text)",
                                                    fontSize: 13,
                                                    width: field.type === "number" ? 80 : 180,
                                                    textAlign: field.type === "number" ? "right" : "left",
                                                }}
                                            />
                                        )}
                                    </div>
                                </div>
                            ))}
                        </div>
                    </div>
                ))}
            </div>
        </div>
    );
}
