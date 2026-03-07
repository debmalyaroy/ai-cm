import { useState, useRef, useEffect, useCallback } from "react";
import { streamChat, chatAPI, alertsAPI, type ChatSSEEvent, type ChatSession } from "@/lib/api";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

type DockPosition = "right" | "left" | "bottom";

interface SuggestionItem {
    label: string;
    type: "question" | "download" | "email" | "action";
    value: string;
    link?: string;
}

interface ChatMessage {
    id: string;
    role: "user" | "assistant";
    content: string;
    reasoning?: Array<{ type: string; content: string }>;
    isStreaming?: boolean;
    suggestions?: SuggestionItem[];
}

interface CreateAlertForm {
    title: string;
    message: string;
    severity: "critical" | "warning" | "info";
    category: string;
}

export default function ChatPanel() {
    const [isOpen, setIsOpen] = useState(false);
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [input, setInput] = useState("");
    const [isLoading, setIsLoading] = useState(false);
    const [sessionId, setSessionId] = useState<string | null>(null);
    const [status, setStatus] = useState("");
    const [dockPosition, setDockPosition] = useState<DockPosition>("right");
    const [panelWidth, setPanelWidth] = useState(420);
    const [isResizing, setIsResizing] = useState(false);
    const [showHistory, setShowHistory] = useState(false);
    const [sessions, setSessions] = useState<ChatSession[]>([]);
    const [createAlertForm, setCreateAlertForm] = useState<CreateAlertForm | null>(null);
    const [alertSaving, setAlertSaving] = useState(false);
    const [actedSuggestions, setActedSuggestions] = useState<Set<string>>(new Set());
    const [restoreOffer, setRestoreOffer] = useState<ChatSession | null>(null);
    const messagesEndRef = useRef<HTMLDivElement>(null);
    const abortRef = useRef<AbortController | null>(null);
    const pendingMessage = useRef<string | null>(null);
    const [explainTrigger, setExplainTrigger] = useState(0);
    const proactiveCountRef = useRef(0); // tracks proactive insights shown this session (max 3)

    const scrollToBottom = () => {
        messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
    };

    useEffect(scrollToBottom, [messages, status]);

    // Restore dock position
    useEffect(() => {
        const saved = localStorage.getItem("aicm-chat-dock");
        if (saved === "left" || saved === "right" || saved === "bottom") setDockPosition(saved);
    }, []);

    // When panel opens with no active session: offer to restore the most recent session
    useEffect(() => {
        if (!isOpen || messages.length > 0 || sessionId) return;
        chatAPI.getSessions().then(sessions => {
            if (sessions.length > 0) setRestoreOffer(sessions[0]);
        }).catch(() => {/* non-fatal */});
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [isOpen]);

    // Broadcast panel width via CSS variable for page resize
    useEffect(() => {
        if (!isOpen) {
            document.documentElement.style.setProperty("--chat-panel-width", "0px");
            return;
        }
        if (dockPosition === "bottom") {
            document.documentElement.style.setProperty("--chat-panel-width", "0px");
        } else {
            document.documentElement.style.setProperty("--chat-panel-width", `${panelWidth}px`);
        }
        if (dockPosition === "left" && isOpen) {
            document.documentElement.setAttribute("data-chat-dock", "left");
        } else {
            document.documentElement.removeAttribute("data-chat-dock");
        }
    }, [isOpen, dockPosition, panelWidth]);

    // Handle Resize — disable transitions during drag for smooth experience
    useEffect(() => {
        if (!isResizing) {
            document.documentElement.removeAttribute("data-resizing");
            return;
        }
        document.documentElement.setAttribute("data-resizing", "true");

        const handleMouseMove = (e: MouseEvent) => {
            if (dockPosition === "right") {
                const newWidth = window.innerWidth - e.clientX;
                setPanelWidth(Math.max(300, Math.min(newWidth, Math.floor(window.innerWidth * 0.8))));
            } else if (dockPosition === "left") {
                const newWidth = e.clientX;
                setPanelWidth(Math.max(300, Math.min(newWidth, Math.floor(window.innerWidth * 0.8))));
            }
        };
        const handleMouseUp = () => setIsResizing(false);

        document.addEventListener("mousemove", handleMouseMove);
        document.addEventListener("mouseup", handleMouseUp);
        document.body.style.userSelect = "none";
        return () => {
            document.removeEventListener("mousemove", handleMouseMove);
            document.removeEventListener("mouseup", handleMouseUp);
            document.body.style.userSelect = "";
        };
    }, [isResizing, dockPosition]);

    // Listen for explain events from dashboard
    useEffect(() => {
        const handler = (e: Event) => {
            const detail = (e as CustomEvent).detail;
            if (detail?.message) {
                setIsOpen(true);
                pendingMessage.current = detail.message;
                setExplainTrigger(prev => prev + 1);
            }
        };
        window.addEventListener("aicm-chat", handler);
        return () => window.removeEventListener("aicm-chat", handler);
    }, []);

    const handleSend = useCallback(async (overrideMsg?: string, overrideContext?: string) => {
        const msgText = overrideMsg || input.trim();
        if (!msgText || isLoading) return;

        const userMessage: ChatMessage = { id: Date.now().toString(), role: "user", content: msgText };
        setMessages((prev) => [...prev, userMessage]);
        setInput("");
        setIsLoading(true);
        setStatus("Thinking...");

        const assistantId = (Date.now() + 1).toString();
        let reasoning: Array<{ type: string; content: string }> = [];

        setMessages((prev) => [
            ...prev,
            { id: assistantId, role: "assistant", content: "", reasoning: [], isStreaming: true },
        ]);

        // For the very first message of a new session, include page context so the
        // agent knows where the user is (C4: update context with current page)
        const effectiveContext = overrideContext ||
            (!sessionId ? `User is on page: ${window.location.pathname}` : undefined);

        abortRef.current = streamChat(
            msgText,
            sessionId,
            effectiveContext,
            (event: ChatSSEEvent) => {
                switch (event.event) {
                    case "session":
                        setSessionId(event.data as unknown as string);
                        break;
                    case "status":
                        setStatus((event.data as { message: string }).message);
                        break;
                    case "reasoning":
                        reasoning = [...reasoning, event.data as { type: string; content: string }];
                        setMessages((prev) =>
                            prev.map((m) => (m.id === assistantId ? { ...m, reasoning } : m))
                        );
                        break;
                    case "response":
                        setMessages((prev) =>
                            prev.map((m) =>
                                m.id === assistantId
                                    ? { ...m, content: (event.data as { content: string }).content, isStreaming: false }
                                    : m
                            )
                        );
                        setStatus("");
                        break;
                    case "suggestions": {
                        const sugData = event.data as { questions: SuggestionItem[] | string[] };
                        const items: SuggestionItem[] = sugData.questions.map((q: SuggestionItem | string) => {
                            if (typeof q === "string") return { label: q, type: "question" as const, value: q };
                            return q as SuggestionItem;
                        });
                        setMessages((prev) =>
                            prev.map((m) =>
                                m.id === assistantId ? { ...m, suggestions: items } : m
                            )
                        );
                        break;
                    }
                    case "error":
                        setMessages((prev) =>
                            prev.map((m) =>
                                m.id === assistantId
                                    ? { ...m, content: `Error: ${(event.data as { error: string }).error}`, isStreaming: false }
                                    : m
                            )
                        );
                        setStatus("");
                        break;
                }
            },
            () => { setIsLoading(false); setStatus(""); },
            (err: Error) => {
                setMessages((prev) =>
                    prev.map((m) =>
                        m.id === assistantId
                            ? { ...m, content: `Connection error: ${err.message}`, isStreaming: false }
                            : m
                    )
                );
                setIsLoading(false);
                setStatus("");
            }
        );
    }, [input, isLoading, sessionId]);

    // Auto-send pending messages (from explain events)
    useEffect(() => {
        if (pendingMessage.current && !isLoading) {
            const msg = pendingMessage.current;
            pendingMessage.current = null;
            handleSend(msg);
        }
    }, [isOpen, isLoading, handleSend, explainTrigger]);

    // Returns page-aware quick-start suggestions shown in the welcome screen
    const getPageQuickActions = (): SuggestionItem[] => {
        const path = window.location.pathname;
        if (path.includes("/actions")) return [
            { label: "📋 Pending approvals", type: "question", value: "Show all pending actions awaiting my approval" },
            { label: "⚡ High-impact actions", type: "question", value: "Which pending actions have the highest expected impact?" },
            { label: "📊 Actions by category", type: "question", value: "Break down pending actions by category" },
            { label: "📅 Recommend order", type: "question", value: "In what order should I approve these actions for maximum impact?" },
            { label: "⚙️ Propose new actions", type: "action", value: "Analyse current sales data and propose 3 new actions I should take this week" },
        ];
        if (path.includes("/alerts")) return [
            { label: "🔴 Critical issues now", type: "question", value: "What are the most critical business issues I should address today?" },
            { label: "📦 Out-of-stock risk", type: "question", value: "Which products are at risk of going out of stock in the next 7 days?" },
            { label: "💰 Margin at risk", type: "question", value: "Which products have margin declining more than 10% this month?" },
            { label: "📈 Revenue anomalies", type: "question", value: "Are there any unusual revenue spikes or drops in the last 14 days?" },
            { label: "🔔 Create alert", type: "action", value: "Create an alert for products where stock drops below reorder level" },
        ];
        if (path.includes("/reports")) return [
            { label: "📊 Monthly summary", type: "download", value: "Generate a monthly performance summary report for this month" },
            { label: "📈 Sales by category", type: "question", value: "Show me revenue and margin breakdown by category for the last 3 months" },
            { label: "🏆 Top 10 products", type: "question", value: "Who are the top 10 products by revenue in the last 90 days?" },
            { label: "📉 Underperformers", type: "question", value: "Which products are significantly underperforming vs last period?" },
            { label: "📧 Brief the team", type: "email", value: "Draft a monthly performance summary email for my category team" },
        ];
        // Default / dashboard
        return [
            { label: "🏆 Top performers", type: "question", value: "Show me the top 10 products by revenue this month" },
            { label: "📉 Underperformers", type: "question", value: "Which products are underperforming vs last month?" },
            { label: "📅 Compare last period", type: "question", value: "Compare this month's sales vs last month by category" },
            { label: "💡 Show recommendations", type: "action", value: "Based on current data, what are your top 3 recommendations for me this week?" },
            { label: "⚠️ Inventory alerts", type: "question", value: "Which products are below reorder level right now?" },
        ];
    };

    const changeDock = (pos: DockPosition) => {
        setDockPosition(pos);
        localStorage.setItem("aicm-chat-dock", pos);
    };

    const handleNewSession = () => {
        if (abortRef.current) { abortRef.current.abort(); abortRef.current = null; }
        setMessages([]);
        setSessionId(null);
        setShowHistory(false);
        setStatus("");
        setIsLoading(false);
        setCreateAlertForm(null);
        setActedSuggestions(new Set());
        setRestoreOffer(null);
    };

    const loadSessions = async () => {
        try {
            const data = await chatAPI.getSessions();
            setSessions(data.slice(0, 10));
        } catch { setSessions([]); }
    };

    const loadSession = async (sid: string) => {
        try {
            const msgs = await chatAPI.getMessages(sid);
            setMessages(msgs.map(m => {
                let suggestions: SuggestionItem[] | undefined = undefined;
                if (m.role === "assistant" && m.metadata) {
                    try {
                        const meta = typeof m.metadata === "string" ? JSON.parse(m.metadata) : m.metadata;
                        if (meta?.suggestions) suggestions = meta.suggestions;
                    } catch { /* ignore */ }
                }
                return { id: m.id, role: m.role as "user" | "assistant", content: m.content, suggestions };
            }));
            setSessionId(sid);
            setShowHistory(false);
        } catch { /* ignore */ }
    };

    const toggleHistory = () => {
        if (!showHistory) loadSessions();
        setShowHistory(!showHistory);
    };

    const detachToTab = () => {
        localStorage.setItem("aicm-detach-messages", JSON.stringify(messages));
        if (sessionId) localStorage.setItem("aicm-detach-session", sessionId);
        window.open(sessionId ? `/chat?sessionId=${sessionId}` : `/chat`, "_blank");
    };

    // Handle suggestion click — intercept "create alert" action, track acted state, handle nav links
    const handleSuggestionClick = (s: SuggestionItem, parentMsgContent?: string) => {
        setActedSuggestions(prev => { const next = new Set(prev); next.add(s.value); return next; });

        // Intercept "create alert" action — show inline form instead of sending
        const isCreateAlert = s.type === "action" && /alert/i.test(s.value);
        if (isCreateAlert) {
            setCreateAlertForm({ title: "", message: "", severity: "warning", category: "General" });
            return;
        }

        // For suggestions with nav links, open them after sending (if action/download type)
        if (s.link && (s.type === "action" || s.type === "download")) {
            handleSend(s.value, parentMsgContent);
            setTimeout(() => window.open(s.link, "_self"), 1500);
            return;
        }

        handleSend(s.value, parentMsgContent);
    };

    const handleSaveAlert = async () => {
        if (!createAlertForm || !createAlertForm.title.trim()) return;
        setAlertSaving(true);
        try {
            await alertsAPI.addAlert(createAlertForm);
            // Add confirmation message to chat
            const confirmMsg: ChatMessage = {
                id: Date.now().toString(),
                role: "assistant",
                content: `✅ **Alert created successfully!**\n\n**"${createAlertForm.title}"** has been saved as a **${createAlertForm.severity}** alert. You can view and manage it on the [Alerts page](/alerts).`,
            };
            setMessages((prev) => [...prev, confirmMsg]);
            setCreateAlertForm(null);
        } catch (e) {
            console.error(e);
        } finally {
            setAlertSaving(false);
        }
    };

    return (
        <>
            {/* FAB - only when closed */}
            {!isOpen && (
                <button className="chat-fab animate-pulse-glow" onClick={() => setIsOpen(true)}>💬</button>
            )}

            {/* Chat Panel */}
            {isOpen && (
                <div
                    className={`chat-panel dock-${dockPosition}`}
                    style={{
                        width: dockPosition !== "bottom" ? (isOpen ? panelWidth : 0) : undefined,
                        height: dockPosition === "bottom" ? (isOpen ? 360 : 0) : undefined,
                    }}
                >
                    {/* Resizer Handle */}
                    {dockPosition !== "bottom" && (
                        <div
                            className={`chat-resizer ${dockPosition === 'right' ? 'resizer-left' : 'resizer-right'}`}
                            onMouseDown={() => setIsResizing(true)}
                            style={{
                                position: "absolute", top: 0, bottom: 0,
                                [dockPosition === "right" ? "left" : "right"]: 0,
                                width: "6px", cursor: "col-resize", zIndex: 10,
                                backgroundColor: isResizing ? "var(--color-primary)" : "transparent",
                                transition: "background-color 0.2s"
                            }}
                        />
                    )}

                    {/* Header */}
                    <div className="chat-header">
                        <div style={{ display: "flex", alignItems: "center", gap: 8, flex: 1, minWidth: 0 }}>
                            <span className="chat-header-icon">🤖</span>
                            <span className="chat-header-title">AI Copilot</span>
                        </div>
                        <div className="chat-header-actions">
                            <button title={isLoading ? "Please wait for response to finish" : "New session"} className="chat-header-btn" onClick={handleNewSession} disabled={isLoading} style={{ opacity: isLoading ? 0.5 : 1 }}>✚</button>
                            <button className="chat-header-btn" onClick={toggleHistory} title="Chat History">🕐</button>
                            <button className="chat-header-btn" onClick={() => changeDock(dockPosition === "left" ? "right" : "left")} title="Toggle dock side">
                                {dockPosition === "left" ? "▶" : "◀"}
                            </button>
                            <button className="chat-header-btn" onClick={() => changeDock("bottom")} title="Dock bottom">▽</button>
                            <button title={isLoading ? "Cannot detach while generating" : "Open in new window"} className="chat-header-btn" onClick={detachToTab} disabled={isLoading} style={{ opacity: isLoading ? 0.5 : 1 }}>⧉</button>
                            <button className="chat-header-btn chat-close-btn" onClick={() => setIsOpen(false)} title="Close">✕</button>
                        </div>
                    </div>

                    {/* Session History Drawer */}
                    {showHistory && (
                        <div className="chat-history-drawer">
                            <div className="chat-history-title">Recent Sessions (max 10)</div>
                            {sessions.length === 0 ? (
                                <div className="chat-history-empty">No sessions yet</div>
                            ) : (
                                sessions.map(s => (
                                    <button key={s.id} className={`chat-history-item ${s.id === sessionId ? "active" : ""}`} onClick={() => loadSession(s.id)}>
                                        <span className="chat-history-msg">{s.first_message.slice(0, 50)}</span>
                                        <span className="chat-history-date">{new Date(s.created_at).toLocaleDateString()}</span>
                                    </button>
                                ))
                            )}
                        </div>
                    )}

                    {/* Messages */}
                    <div className="chat-messages">
                        {messages.length === 0 && (
                            <div className="chat-welcome">
                                <div className="chat-welcome-icon">🤖</div>
                                <div className="chat-welcome-title">AI Category Copilot</div>
                                <div className="chat-welcome-text">Ask me about your sales, products, trends, or any business question.</div>

                                {/* Restore last session offer */}
                                {restoreOffer && (
                                    <div style={{ margin: "12px 0 4px", padding: "10px 14px", background: "var(--color-surface)", borderRadius: 10, border: "1px solid var(--color-border)", fontSize: 13 }}>
                                        <div style={{ color: "var(--color-text-secondary)", marginBottom: 6 }}>Continue last conversation?</div>
                                        <div style={{ fontWeight: 500, marginBottom: 8, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                                            "{restoreOffer.first_message.slice(0, 60)}"
                                        </div>
                                        <div style={{ display: "flex", gap: 8 }}>
                                            <button
                                                className="btn btn-primary"
                                                style={{ fontSize: 12, padding: "4px 12px" }}
                                                onClick={() => { loadSession(restoreOffer.id); setRestoreOffer(null); }}
                                            >Restore</button>
                                            <button
                                                style={{ fontSize: 12, padding: "4px 10px", background: "none", border: "none", color: "var(--color-text-secondary)", cursor: "pointer" }}
                                                onClick={() => setRestoreOffer(null)}
                                            >Dismiss</button>
                                        </div>
                                    </div>
                                )}

                                {/* Page-aware quick action buttons */}
                                <div style={{ marginTop: 14 }}>
                                    <div style={{ fontSize: 11, color: "var(--color-text-secondary)", marginBottom: 8, textTransform: "uppercase", letterSpacing: "0.05em" }}>Quick starts</div>
                                    <div className="chat-followups">
                                        {getPageQuickActions().map((s, i) => (
                                            <button
                                                key={i}
                                                className={`chat-followup-pill action-${s.type}`}
                                                onClick={() => handleSuggestionClick(s)}
                                            >
                                                {s.label}
                                            </button>
                                        ))}
                                    </div>
                                </div>
                            </div>
                        )}

                        {messages.map((msg) => (
                            <div key={msg.id} className={`chat-message ${msg.role}`}>
                                <div className={`chat-bubble ${msg.role}`}>
                                    {msg.role === "user" ? (
                                        <span>{msg.content}</span>
                                    ) : (
                                        <>
                                            {msg.reasoning && msg.reasoning.length > 0 && (
                                                <div className="chat-reasoning">
                                                    {msg.reasoning.map((r, i) => (
                                                        <div key={i} className="chat-reasoning-step">
                                                            <span className="chat-reasoning-icon">💭</span>
                                                            <span>{r.content}</span>
                                                        </div>
                                                    ))}
                                                </div>
                                            )}
                                            {msg.isStreaming && !msg.content ? (
                                                <div className="chat-thinking-dots">
                                                    <span className="thinking-dot chat-thinking-dot" />
                                                    <span className="thinking-dot chat-thinking-dot" />
                                                    <span className="thinking-dot chat-thinking-dot" />
                                                </div>
                                            ) : (
                                                <div className="chat-response">
                                                    <ReactMarkdown
                                                        remarkPlugins={[remarkGfm]}
                                                        components={{
                                                            table: ({ children, ...props }) => (
                                                                <div style={{ overflowX: "auto", maxWidth: "100%" }}>
                                                                    <table {...props}>{children}</table>
                                                                </div>
                                                            ),
                                                        }}
                                                    >{msg.content}</ReactMarkdown>
                                                </div>
                                            )}
                                        </>
                                    )}
                                </div>

                                {msg.role === "assistant" && !msg.isStreaming && msg.suggestions && msg.suggestions.length > 0 && (
                                    <div className="chat-followups">
                                        {msg.suggestions.map((s, i) => (
                                            <button
                                                key={i}
                                                className={`chat-followup-pill action-${s.type}${actedSuggestions.has(s.value) ? " acted" : ""}`}
                                                onClick={() => handleSuggestionClick(s, msg.content)}
                                                title={s.link ? `Opens ${s.link}` : undefined}
                                            >
                                                {actedSuggestions.has(s.value) ? "✓ " : ""}{s.label}
                                            </button>
                                        ))}
                                    </div>
                                )}
                            </div>
                        ))}

                        {status && (
                            <div className="chat-status">
                                <span className="chat-status-dot" />
                                {status}
                            </div>
                        )}

                        {/* Inline Create Alert form */}
                        {createAlertForm && (
                            <div style={{ margin: "8px 0", padding: 16, background: "var(--color-bg)", borderRadius: 12, border: "1px solid var(--color-border)" }}>
                                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
                                    <div style={{ fontWeight: 600, fontSize: 14 }}>🔔 Create Alert</div>
                                    <button
                                        onClick={() => setCreateAlertForm(null)}
                                        style={{ background: "none", border: "none", cursor: "pointer", color: "var(--color-text-secondary)", fontSize: 14 }}
                                    >✕</button>
                                </div>
                                <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                                    <input
                                        placeholder="Alert title *"
                                        value={createAlertForm.title}
                                        onChange={(e) => setCreateAlertForm({ ...createAlertForm, title: e.target.value })}
                                        style={{ padding: "7px 10px", borderRadius: 8, border: "1px solid var(--color-border)", background: "var(--color-surface)", color: "var(--color-text)", fontSize: 13 }}
                                    />
                                    <textarea
                                        placeholder="Alert message / details *"
                                        value={createAlertForm.message}
                                        onChange={(e) => setCreateAlertForm({ ...createAlertForm, message: e.target.value })}
                                        rows={2}
                                        style={{ padding: "7px 10px", borderRadius: 8, border: "1px solid var(--color-border)", background: "var(--color-surface)", color: "var(--color-text)", fontSize: 13, resize: "vertical" }}
                                    />
                                    <div style={{ display: "flex", gap: 8 }}>
                                        <div style={{ flex: 1 }}>
                                            <select
                                                value={createAlertForm.severity}
                                                onChange={(e) => setCreateAlertForm({ ...createAlertForm, severity: e.target.value as "critical" | "warning" | "info" })}
                                                style={{ width: "100%", padding: "7px 10px", borderRadius: 8, border: "1px solid var(--color-border)", background: "var(--color-surface)", color: "var(--color-text)", fontSize: 13 }}
                                            >
                                                <option value="critical">🔴 Critical</option>
                                                <option value="warning">🟡 Warning</option>
                                                <option value="info">🔵 Info</option>
                                            </select>
                                        </div>
                                        <input
                                            placeholder="Category"
                                            value={createAlertForm.category}
                                            onChange={(e) => setCreateAlertForm({ ...createAlertForm, category: e.target.value })}
                                            style={{ flex: 1, padding: "7px 10px", borderRadius: 8, border: "1px solid var(--color-border)", background: "var(--color-surface)", color: "var(--color-text)", fontSize: 13 }}
                                        />
                                    </div>
                                    <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
                                        <button onClick={() => setCreateAlertForm(null)} style={{ padding: "6px 14px", borderRadius: 8, border: "1px solid var(--color-border)", background: "transparent", color: "var(--color-text-secondary)", cursor: "pointer", fontSize: 13 }}>
                                            Cancel
                                        </button>
                                        <button
                                            onClick={handleSaveAlert}
                                            disabled={alertSaving || !createAlertForm.title.trim() || !createAlertForm.message.trim()}
                                            className="btn btn-primary"
                                            style={{ fontSize: 13, padding: "6px 16px" }}
                                        >
                                            {alertSaving ? "Saving..." : "🔔 Create Alert"}
                                        </button>
                                    </div>
                                </div>
                            </div>
                        )}

                        <div ref={messagesEndRef} />
                    </div>

                    {/* Input */}
                    <div className="chat-input-area">
                        <input
                            id="chat-input"
                            className="chat-input"
                            value={input}
                            onChange={(e) => setInput(e.target.value)}
                            onKeyDown={(e) => e.key === "Enter" && handleSend()}
                            placeholder="Ask anything..."
                            disabled={isLoading}
                        />
                        <button
                            className={`chat-send-btn ${input.trim() ? "active" : "inactive"}`}
                            onClick={() => handleSend()}
                            disabled={!input.trim() || isLoading}
                        >
                            ➤
                        </button>
                    </div>
                </div>
            )}
        </>
    );
}
