"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import { streamChat, chatAPI, type ChatSSEEvent, type ChatSession } from "@/lib/api";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

type DockPosition = "right" | "left" | "bottom";

interface SuggestionItem {
    label: string;
    type: "question" | "download" | "email" | "action";
    value: string;
}

interface ChatMessage {
    id: string;
    role: "user" | "assistant";
    content: string;
    reasoning?: Array<{ type: string; content: string }>;
    isStreaming?: boolean;
    suggestions?: SuggestionItem[];
}

export default function ChatPanel() {
    const [isOpen, setIsOpen] = useState(false);
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [input, setInput] = useState("");
    const [isLoading, setIsLoading] = useState(false);
    const [sessionId, setSessionId] = useState<string | null>(null);
    const [status, setStatus] = useState("");
    const [dockPosition, setDockPosition] = useState<DockPosition>("right");
    const [showHistory, setShowHistory] = useState(false);
    const [sessions, setSessions] = useState<ChatSession[]>([]);
    const messagesEndRef = useRef<HTMLDivElement>(null);
    const abortRef = useRef<AbortController | null>(null);
    const pendingMessage = useRef<string | null>(null);
    const [explainTrigger, setExplainTrigger] = useState(0);

    const scrollToBottom = () => {
        messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
    };

    useEffect(scrollToBottom, [messages, status]);

    // Restore dock position
    useEffect(() => {
        const saved = localStorage.getItem("aicm-chat-dock");
        if (saved === "left" || saved === "right" || saved === "bottom") setDockPosition(saved);
    }, []);

    // Broadcast panel width via CSS variable for page resize
    useEffect(() => {
        if (!isOpen) {
            document.documentElement.style.setProperty("--chat-panel-width", "0px");
            return;
        }
        if (dockPosition === "bottom") {
            document.documentElement.style.setProperty("--chat-panel-width", "0px");
        } else {
            document.documentElement.style.setProperty("--chat-panel-width", "420px");
        }
        // Also set a data attribute for left dock margin adjustments
        if (dockPosition === "left" && isOpen) {
            document.documentElement.setAttribute("data-chat-dock", "left");
        } else {
            document.documentElement.removeAttribute("data-chat-dock");
        }
    }, [isOpen, dockPosition]);

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

    const handleSend = useCallback(async (overrideMsg?: string) => {
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

        abortRef.current = streamChat(
            msgText,
            sessionId,
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
            () => {
                setIsLoading(false);
                setStatus("");
            },
            (err) => {
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

    const changeDock = (pos: DockPosition) => {
        setDockPosition(pos);
        localStorage.setItem("aicm-chat-dock", pos);
    };

    const handleNewSession = () => {
        setMessages([]);
        setSessionId(null);
        setShowHistory(false);
    };

    const loadSessions = async () => {
        try {
            const data = await chatAPI.getSessions();
            setSessions(data.slice(0, 10));
        } catch {
            setSessions([]);
        }
    };

    const loadSession = async (sid: string) => {
        try {
            const msgs = await chatAPI.getMessages(sid);
            setMessages(msgs.map(m => ({
                id: m.id,
                role: m.role as "user" | "assistant",
                content: m.content,
            })));
            setSessionId(sid);
            setShowHistory(false);
        } catch {
            // ignore
        }
    };

    const toggleHistory = () => {
        if (!showHistory) loadSessions();
        setShowHistory(!showHistory);
    };

    const detachToTab = () => {
        // Always save current messages (including mid-stream) to localStorage
        localStorage.setItem("aicm-detach-messages", JSON.stringify(messages));
        if (sessionId) {
            localStorage.setItem("aicm-detach-session", sessionId);
        }
        const url = sessionId ? `/chat?sessionId=${sessionId}` : `/chat`;
        window.open(url, "_blank");
    };

    const panelWidth = dockPosition === "bottom" ? "auto" : isOpen ? 420 : 0;
    const panelHeight = dockPosition === "bottom" ? (isOpen ? 360 : 0) : "auto";

    return (
        <>
            {/* FAB - only when closed */}
            {!isOpen && (
                <button className="chat-fab animate-pulse-glow" onClick={() => setIsOpen(true)}>
                    💬
                </button>
            )}

            {/* Chat Panel */}
            {isOpen && (
                <div
                    className={`chat-panel dock-${dockPosition}`}
                    style={{
                        width: dockPosition !== "bottom" ? panelWidth : undefined,
                        height: dockPosition === "bottom" ? panelHeight : undefined,
                    }}
                >
                    {/* Header */}
                    <div className="chat-header">
                        <div style={{ display: "flex", alignItems: "center", gap: 8, flex: 1, minWidth: 0 }}>
                            <span className="chat-header-icon">🤖</span>
                            <span className="chat-header-title">AI Copilot</span>
                        </div>
                        <div className="chat-header-actions">
                            {/* New Session */}
                            <button
                                title={isLoading ? "Please wait for response to finish" : "Clear & start new session"}
                                className="chat-header-btn"
                                onClick={handleNewSession}
                                disabled={isLoading}
                                style={{ opacity: isLoading ? 0.5 : 1, cursor: isLoading ? "not-allowed" : "pointer" }}
                            >
                                ✚
                            </button>
                            {/* History */}
                            <button className="chat-header-btn" onClick={toggleHistory} title="Chat History">🕐</button>
                            {/* Dock controls */}
                            <button className="chat-header-btn" onClick={() => changeDock(dockPosition === "left" ? "right" : "left")} title="Toggle dock side">
                                {dockPosition === "left" ? "▶" : "◀"}
                            </button>
                            <button className="chat-header-btn" onClick={() => changeDock("bottom")} title="Dock bottom">▽</button>
                            <button
                                title={isLoading ? "Cannot detach while generating" : "Open in new window"}
                                className="chat-header-btn"
                                onClick={detachToTab}
                                disabled={isLoading}
                                style={{ opacity: isLoading ? 0.5 : 1, cursor: isLoading ? "not-allowed" : "pointer" }}
                            >
                                ⧉
                            </button>
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
                                    <button
                                        key={s.id}
                                        className={`chat-history-item ${s.id === sessionId ? "active" : ""}`}
                                        onClick={() => loadSession(s.id)}
                                    >
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

                                {/* Follow-up suggestions from backend */}
                                {msg.role === "assistant" && !msg.isStreaming && msg.suggestions && msg.suggestions.length > 0 && (
                                    <div className="chat-followups">
                                        {msg.suggestions.map((s, i) => (
                                            <button
                                                key={i}
                                                className={`chat-followup-pill action-${s.type}`}
                                                onClick={() => handleSend(s.value)}
                                            >
                                                {s.label}
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
