"use client";

import { useState, useEffect } from "react";
import { Link, useLocation } from "react-router-dom";

const navItems = [
    { href: "/dashboard", label: "Dashboard", icon: "📊" },
    { href: "/news", label: "News", icon: "📰" },
    { href: "/actions", label: "Approvals", icon: "⚡" },
    { href: "/reports", label: "Reports", icon: "📋" },
    { href: "/alerts", label: "Alerts", icon: "🔔" },
    { href: "/config", label: "Config", icon: "⚙️" },
];

export default function Sidebar() {
    const location = useLocation();
    const pathname = location.pathname;
    const [isDark, setIsDark] = useState(true);
    const [collapsed, setCollapsed] = useState(false);

    useEffect(() => {
        const saved = localStorage.getItem("aicm-theme");
        if (saved === "light") {
            setIsDark(false);
            document.documentElement.classList.remove("dark");
        }
        const savedCollapse = localStorage.getItem("aicm-sidebar-collapsed");
        if (savedCollapse === "true") setCollapsed(true);
    }, []);

    // Broadcast sidebar width via CSS custom property for layout resizing
    useEffect(() => {
        const width = collapsed ? 60 : 240;
        document.documentElement.style.setProperty("--sidebar-width", `${width}px`);
    }, [collapsed]);

    const toggleTheme = () => {
        const next = !isDark;
        setIsDark(next);
        if (next) {
            document.documentElement.classList.add("dark");
            localStorage.setItem("aicm-theme", "dark");
        } else {
            document.documentElement.classList.remove("dark");
            localStorage.setItem("aicm-theme", "light");
        }
    };

    const toggleCollapse = () => {
        const next = !collapsed;
        setCollapsed(next);
        localStorage.setItem("aicm-sidebar-collapsed", String(next));
    };

    return (
        <aside className={`sidebar ${collapsed ? "collapsed" : ""}`}>
            <div className="sidebar-logo">
                <div className="sidebar-logo-box">
                    <div className="sidebar-logo-icon">AI</div>
                    {!collapsed && (
                        <div>
                            <div className="sidebar-logo-title">AI-CM</div>
                            <div className="sidebar-logo-subtitle">Category Copilot</div>
                        </div>
                    )}
                </div>
            </div>

            {/* Collapse toggle */}
            <button className="sidebar-collapse-btn" onClick={toggleCollapse} title={collapsed ? "Expand" : "Collapse"}>
                {collapsed ? "»" : "«"}
            </button>

            <nav className="sidebar-nav">
                {navItems.map((item) => {
                    const isActive = pathname === item.href;
                    return (
                        <Link
                            key={item.href}
                            to={item.href}
                            className={`sidebar-link ${isActive ? "active" : ""}`}
                            title={collapsed ? item.label : undefined}
                        >
                            <span className="sidebar-link-icon">{item.icon}</span>
                            {!collapsed && item.label}
                        </Link>
                    );
                })}
            </nav>

            {/* Theme Toggle */}
            <button className="theme-toggle" onClick={toggleTheme}>
                <span>{isDark ? "🌙" : "☀️"}</span>
                {!collapsed && (
                    <>
                        <div className={`theme-toggle-track ${isDark ? "active" : ""}`}>
                            <div className={`theme-toggle-knob ${isDark ? "active" : ""}`} />
                        </div>
                        <span>{isDark ? "Dark" : "Light"}</span>
                    </>
                )}
            </button>

            {!collapsed && (
                <div className="sidebar-footer">
                    <div className="sidebar-user">
                        <div className="sidebar-avatar">CM</div>
                        <div>
                            <div className="sidebar-username">Demo User</div>
                            <div className="sidebar-role">Category Manager</div>
                        </div>
                    </div>
                </div>
            )}
        </aside>
    );
}
