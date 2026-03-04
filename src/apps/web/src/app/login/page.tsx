"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

export default function LoginPage() {
    const [username, setUsername] = useState("");
    const [password, setPassword] = useState("");
    const [error, setError] = useState("");
    const [loading, setLoading] = useState(false);
    const router = useRouter();

    const handleLogin = async (e: React.FormEvent) => {
        e.preventDefault();
        setLoading(true);
        setError("");

        // Dummy login: admin/admin
        await new Promise((r) => setTimeout(r, 800));

        if (username === "admin" && password === "admin") {
            // Store login state
            if (typeof window !== "undefined") {
                localStorage.setItem("aicm_auth", JSON.stringify({ user: "admin", role: "Category Manager", loggedIn: true }));
            }
            router.push("/dashboard");
        } else {
            setError("Invalid credentials. Try admin/admin");
            setLoading(false);
        }
    };

    return (
        <div
            style={{
                minHeight: "100vh",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                background: "linear-gradient(135deg, #0f0f1a 0%, #1a1a2e 50%, #16213e 100%)",
                fontFamily: "'Inter', sans-serif",
            }}
        >
            <div
                style={{
                    width: 400,
                    padding: 40,
                    borderRadius: 16,
                    background: "rgba(17, 17, 27, 0.9)",
                    border: "1px solid rgba(100, 100, 150, 0.2)",
                    backdropFilter: "blur(20px)",
                    boxShadow: "0 24px 48px rgba(0, 0, 0, 0.4), 0 0 120px rgba(99, 102, 241, 0.1)",
                }}
            >
                {/* Logo */}
                <div style={{ textAlign: "center", marginBottom: 32 }}>
                    <div
                        style={{
                            width: 60,
                            height: 60,
                            borderRadius: 16,
                            background: "var(--color-gradient-gold)",
                            display: "inline-flex",
                            alignItems: "center",
                            justifyContent: "center",
                            fontSize: 24,
                            fontWeight: 700,
                            color: "white",
                            marginBottom: 16,
                            boxShadow: "0 8px 32px rgba(99, 102, 241, 0.3)",
                        }}
                    >
                        AI
                    </div>
                    <h1 style={{ fontSize: 24, fontWeight: 700, color: "white", marginBottom: 4 }}>
                        AI-CM Copilot
                    </h1>
                    <p style={{ fontSize: 14, color: "#94a3b8" }}>
                        Category Manager Decision Intelligence
                    </p>
                </div>

                {/* Form */}
                <form onSubmit={handleLogin}>
                    <div style={{ marginBottom: 16 }}>
                        <label style={{ fontSize: 13, fontWeight: 500, color: "#94a3b8", display: "block", marginBottom: 6 }}>
                            Username
                        </label>
                        <input
                            type="text"
                            value={username}
                            onChange={(e) => setUsername(e.target.value)}
                            placeholder="Enter username"
                            required
                            style={{
                                width: "100%",
                                padding: "12px 16px",
                                borderRadius: 10,
                                border: "1px solid rgba(100, 100, 150, 0.3)",
                                background: "rgba(30, 30, 50, 0.5)",
                                color: "white",
                                fontSize: 14,
                                outline: "none",
                                boxSizing: "border-box",
                                transition: "border-color 0.2s ease",
                            }}
                        />
                    </div>
                    <div style={{ marginBottom: 24 }}>
                        <label style={{ fontSize: 13, fontWeight: 500, color: "#94a3b8", display: "block", marginBottom: 6 }}>
                            Password
                        </label>
                        <input
                            type="password"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            placeholder="Enter password"
                            required
                            style={{
                                width: "100%",
                                padding: "12px 16px",
                                borderRadius: 10,
                                border: "1px solid rgba(100, 100, 150, 0.3)",
                                background: "rgba(30, 30, 50, 0.5)",
                                color: "white",
                                fontSize: 14,
                                outline: "none",
                                boxSizing: "border-box",
                                transition: "border-color 0.2s ease",
                            }}
                        />
                    </div>

                    {error && (
                        <div
                            style={{
                                padding: "10px 14px",
                                borderRadius: 8,
                                background: "rgba(239, 68, 68, 0.1)",
                                border: "1px solid rgba(239, 68, 68, 0.3)",
                                color: "#ef4444",
                                fontSize: 13,
                                marginBottom: 16,
                            }}
                        >
                            {error}
                        </div>
                    )}

                    <button
                        type="submit"
                        disabled={loading}
                        style={{
                            width: "100%",
                            padding: "12px",
                            borderRadius: 10,
                            border: "none",
                            background: loading
                                ? "rgba(99, 102, 241, 0.5)"
                                : "var(--color-gradient-gold)",
                            color: "white",
                            fontSize: 15,
                            fontWeight: 600,
                            cursor: loading ? "not-allowed" : "pointer",
                            transition: "all 0.2s ease",
                            boxShadow: "0 4px 16px rgba(99, 102, 241, 0.25)",
                        }}
                    >
                        {loading ? "Signing in..." : "Sign In"}
                    </button>
                </form>

                {/* Hint */}
                <div
                    style={{
                        marginTop: 20,
                        padding: "10px 14px",
                        borderRadius: 8,
                        background: "rgba(99, 102, 241, 0.1)",
                        border: "1px solid rgba(99, 102, 241, 0.2)",
                        fontSize: 12,
                        color: "#94a3b8",
                        textAlign: "center",
                    }}
                >
                    💡 Demo credentials: <strong style={{ color: "#a5b4fc" }}>admin / admin</strong>
                </div>
            </div>
        </div>
    );
}
