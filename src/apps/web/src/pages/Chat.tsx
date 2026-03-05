import { useEffect, useState, useCallback, useRef } from "react";
import { streamChat, chatAPI, type ChatSSEEvent } from "@/lib/api";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

interface SuggestionItem {
    label: string;
    type: string;
    value: string;
}

interface ChatMessage {
    id: string;
    role: "user" | "assistant";
    content: string;
    suggestions?: SuggestionItem[];
    isStreaming?: boolean;
}

export default function ChatPage() {
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [input, setInput] = useState("");
    const [isLoading, setIsLoading] = useState(false);
    const [sessionId, setSessionId] = useState<string | null>(null);
    const [status, setStatus] = useState("");
    const [isDark, setIsDark] = useState(false);
    const messagesEndRef = useRef<HTMLDivElement>(null);
    const abortRef = useRef<AbortController | null>(null);

    const scrollToBottom = () => {
        messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
    };
    useEffect(scrollToBottom, [messages, status]);

    // Inherit theme from main window
    useEffect(() => {
        const checkTheme = () => {
            const savedTheme = localStorage.getItem("aicm-theme");
            setIsDark(savedTheme !== "light");
            if (savedTheme === "light") {
                document.documentElement.classList.remove("dark");
            } else {
                document.documentElement.classList.add("dark");
            }
        };

        checkTheme();

        // Listen for theme changes via storage events (cross-tab sync)
        const storageHandler = (e: StorageEvent) => {
            if (e.key === "aicm-theme") {
                checkTheme();
            }
        };
        window.addEventListener("storage", storageHandler);
        return () => window.removeEventListener("storage", storageHandler);
    }, []);

    const toggleTheme = () => {
        const newTheme = isDark ? "light" : "dark";
        localStorage.setItem("aicm-theme", newTheme);
        setIsDark(!isDark);
        if (newTheme === "light") {
            document.documentElement.classList.remove("dark");
        } else {
            document.documentElement.classList.add("dark");
        }
    };

    // Load session: first from localStorage (for mid-stream detach), then from DB
    useEffect(() => {
        const params = new URLSearchParams(window.location.search);
        const sid = params.get("sessionId");

        // Always try localStorage first — has the most current state (including mid-stream)
        try {
            const saved = localStorage.getItem("aicm-detach-messages");
            if (saved) {
                const parsed = JSON.parse(saved);
                if (parsed && parsed.length > 0) {
                    setMessages(parsed);
                }
                localStorage.removeItem("aicm-detach-messages");
            }
        } catch { /* ignore */ }

        // Restore session ID
        if (sid) {
            setSessionId(sid);

            // If no messages loaded from localStorage, load from DB
            if (messages.length === 0) {
                chatAPI.getMessages(sid).then((msgs) => {
                    setMessages(prev => {
                        // Only load from DB if we don't already have messages (from localStorage)
                        if (prev.length > 0) return prev;
                        return msgs.map(m => ({
                            id: m.id,
                            role: m.role as "user" | "assistant",
                            content: m.content,
                        }));
                    });
                }).catch(() => { /* ignore */ });
            }
        } else {
            // Check if session was stored separately
            const savedSession = localStorage.getItem("aicm-detach-session");
            if (savedSession) {
                setSessionId(savedSession);
                localStorage.removeItem("aicm-detach-session");
            }
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const handleSend = useCallback(async (overrideMsg?: string, overrideContext?: string) => {
        const msgText = overrideMsg || input.trim();
        if (!msgText || isLoading) return;

        const userMessage: ChatMessage = { id: Date.now().toString(), role: "user", content: msgText };
        setMessages(prev => [...prev, userMessage]);
        setInput("");
        setIsLoading(true);
        setStatus("Thinking...");

        const assistantId = (Date.now() + 1).toString();
        setMessages(prev => [...prev, { id: assistantId, role: "assistant", content: "", isStreaming: true }]);

        abortRef.current = streamChat(
            msgText,
            sessionId,
            overrideContext,
            (event: ChatSSEEvent) => {
                switch (event.event) {
                    case "session":
                        setSessionId(event.data as unknown as string);
                        break;
                    case "status":
                        setStatus((event.data as { message: string }).message);
                        break;
                    case "response":
                        setMessages(prev =>
                            prev.map(m => m.id === assistantId
                                ? { ...m, content: (event.data as { content: string }).content, isStreaming: false }
                                : m
                            )
                        );
                        setStatus("");
                        break;
                    case "suggestions": {
                        const sugData = event.data as { questions: SuggestionItem[] | string[] };
                        const items: SuggestionItem[] = sugData.questions.map((q: SuggestionItem | string) => {
                            if (typeof q === "string") return { label: q, type: "question", value: q };
                            return q as SuggestionItem;
                        });
                        setMessages(prev =>
                            prev.map(m => m.id === assistantId ? { ...m, suggestions: items } : m)
                        );
                        break;
                    }
                    case "error":
                        setMessages(prev =>
                            prev.map(m => m.id === assistantId
                                ? { ...m, content: `Error: ${(event.data as { error: string }).error}`, isStreaming: false }
                                : m
                            )
                        );
                        break;
                }
            },
            () => { setIsLoading(false); setStatus(""); },
            (err: Error) => {
                setMessages(prev =>
                    prev.map(m => m.id === assistantId
                        ? { ...m, content: `Connection error: ${err.message}`, isStreaming: false }
                        : m
                    )
                );
                setIsLoading(false); setStatus("");
            }
        );
    }, [input, isLoading, sessionId]);

    const goBack = () => {
        window.close();
        setTimeout(() => window.history.back(), 200);
    };

    return (
        <div style={{ maxWidth: 800, margin: "0 auto", padding: 24, height: "100vh", display: "flex", flexDirection: "column" }}>
            <style>{`
                .sidebar { display: none !important; }
                .chat-panel { display: none !important; }
                .chat-fab { display: none !important; }
                .main-content { margin: 0 !important; max-width: 100% !important; width: 100vw !important; padding: 0 !important; }
            `}</style>

            {/* Header */}
            <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 20 }}>
                <button className="back-btn" onClick={goBack}>← Back to main</button>
                <h1 className="dashboard-title" style={{ margin: 0 }}>💬 Chat</h1>
                <span style={{ color: "var(--color-text-secondary)", fontSize: 13, flex: 1 }}>
                    {sessionId ? `Session: ${sessionId.slice(0, 8)}...` : "New session"}
                </span>
                <button
                    className="chat-header-btn"
                    onClick={toggleTheme}
                    title="Toggle Theme"
                    style={{ fontSize: "1.2rem", padding: "4px 8px" }}
                >
                    {isDark ? "☀️" : "🌙"}
                </button>
            </div>

            {/* Messages */}
            <div style={{ flex: 1, overflow: "auto", display: "flex", flexDirection: "column", gap: 12 }}>
                {messages.length === 0 && (
                    <div className="chat-welcome">
                        <div className="chat-welcome-icon">🤖</div>
                        <div className="chat-welcome-title">AI Category Copilot</div>
                        <div className="chat-welcome-text">Ask me about your sales, products, trends, or any business question.</div>
                    </div>
                )}

                {messages.map(msg => (
                    <div key={msg.id} className={`chat-message ${msg.role}`}>
                        <div className={`chat-bubble ${msg.role}`}>
                            {msg.role === "user" ? (
                                <span>{msg.content}</span>
                            ) : msg.isStreaming && !msg.content ? (
                                <div className="chat-thinking-dots">
                                    <span className="thinking-dot" />
                                    <span className="thinking-dot" />
                                    <span className="thinking-dot" />
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
                        </div>

                        {msg.role === "assistant" && !msg.isStreaming && msg.suggestions && msg.suggestions.length > 0 && (
                            <div className="chat-followups">
                                {msg.suggestions.map((s, i) => (
                                    <button key={i} className={`chat-followup-pill action-${s.type}`} onClick={() => handleSend(s.value, msg.content)}>
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
            <div className="chat-input-area" style={{ marginTop: 12 }}>
                <input
                    className="chat-input"
                    value={input}
                    onChange={e => setInput(e.target.value)}
                    onKeyDown={e => e.key === "Enter" && handleSend()}
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
    );
}
