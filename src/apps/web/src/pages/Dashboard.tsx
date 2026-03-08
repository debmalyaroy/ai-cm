import { useState, useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import {
    dashboardAPI,
    actionsAPI,
    type KPIs,
    type SalesTrendPoint,
    type CategoryData,
    type RegionData,
    type TopProduct,
} from "@/lib/api";

const FALLBACK_ACTIONS = [
    "Review pricing strategy for underperforming SKUs",
    "Analyze inventory levels and reorder points",
    "Evaluate competitor positioning and market trends",
];

function formatCurrency(val: number): string {
    if (val >= 10000000) return `₹${(val / 10000000).toFixed(1)}Cr`;
    if (val >= 100000) return `₹${(val / 100000).toFixed(1)}L`;
    if (val >= 1000) return `₹${(val / 1000).toFixed(1)}K`;
    return `₹${val.toFixed(0)}`;
}

function formatNumber(val: number): string {
    if (val >= 100000) return `${(val / 1000).toFixed(0)}K`;
    if (val >= 1000) return `${(val / 1000).toFixed(1)}K`;
    return val.toFixed(0);
}

// --- Mock drill-down data generators ---
function generateDailyData(month: string) {
    const days = 28 + Math.floor(Math.random() * 3);
    return Array.from({ length: days }, (_, i) => ({
        day: `${month}-${String(i + 1).padStart(2, "0")}`,
        revenue: 50000 + Math.random() * 200000,
        units: 20 + Math.floor(Math.random() * 100),
    }));
}

function generateProductDetail(product: TopProduct) {
    return {
        months: ["Jan", "Feb", "Mar"].map((m) => ({
            month: m,
            revenue: product.revenue / 3 + (Math.random() - 0.5) * product.revenue * 0.2,
            units: Math.floor(product.units / 3 + (Math.random() - 0.5) * product.units * 0.2),
        })),
        topRegions: ["North", "South", "West", "East"].map((r) => ({
            name: r,
            share: 15 + Math.random() * 35,
        })),
    };
}

// --- Bar with CSS-hover tooltip (tooltip lives inside bar-col as absolute child) ---
function BarWithTooltip({
    height,
    tooltipLines,
    onClick,
    label,
    showValueLabel,
    valueLabel,
    animationDelay,
}: {
    height: number;
    tooltipLines: string[];
    onClick?: () => void;
    label: string;
    showValueLabel: boolean;
    valueLabel: string;
    animationDelay: string;
}) {
    return (
        <div className="bar-col animate-fade-in" style={{ animationDelay }}>
            <div className="bar-tooltip-popup">
                {tooltipLines.map((line, i) => (
                    <div key={i} className={i === 0 ? "bar-tooltip-primary" : "bar-tooltip-secondary"}>
                        {line}
                    </div>
                ))}
            </div>
            {showValueLabel && (
                <span className="bar-value-label">{valueLabel}</span>
            )}
            <div
                className="bar-fill"
                style={{ height, cursor: onClick ? "pointer" : "default" }}
                onClick={onClick}
            />
            <span className="bar-label">{label}</span>
        </div>
    );
}

function CardOptionsButton({ cardType, cardData, variant = "menu" }: { cardType: string; cardData?: unknown; variant?: "menu" | "inline" }) {
    const [showMenu, setShowMenu] = useState(false);
    const [showActions, setShowActions] = useState(false);
    const [actions, setActions] = useState<string[]>([]);
    const [loadingActions, setLoadingActions] = useState(false);
    const menuRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        const handler = (e: MouseEvent) => {
            if (menuRef.current && !menuRef.current.contains(e.target as Node)) setShowMenu(false);
        };
        document.addEventListener("mousedown", handler);
        return () => document.removeEventListener("mousedown", handler);
    }, []);

    // formatSummary removed since we now rely on DB queries

    const handleExplain = () => {
        setShowMenu(false);
        const query = `Fetch recent analytical data for my "${cardType}" metrics from the database using SQL. What do these numbers mean for my business? Are there any concerns or opportunities?`;
        window.dispatchEvent(new CustomEvent("aicm-chat", { detail: { message: query } }));
    };

    const handleRecommendActions = async () => {
        setShowMenu(false);
        setShowActions(true);
        setLoadingActions(true);
        try {
            const { actions: suggested } = await dashboardAPI.getCardActions(cardType, cardData);
            setActions(suggested.length > 0 ? suggested.slice(0, 3) : FALLBACK_ACTIONS);
        } catch {
            setActions(FALLBACK_ACTIONS);
        } finally {
            setLoadingActions(false);
        }
    };

    const executeAction = async (action: string) => {
        setShowActions(false);
        try {
            await actionsAPI.addAction({
                title: action,
                description: `Automatically created via Dashboard action on ${cardType}`,
                action_type: "manual_execution",
                category: cardType,
                confidence_score: 1.0
            });
        } catch (err) {
            // Silent fail to avoid leaking internal error details to client console
        }
        window.dispatchEvent(new CustomEvent("aicm-chat", { detail: { message: `Execute: ${action} for ${cardType}` } }));
    };

    const modal = showActions ? (
        <div className="modal-backdrop" onClick={() => setShowActions(false)}>
            <div className="modal-content modal-actions" onClick={(e) => e.stopPropagation()}>
                <div className="modal-actions-header">
                    <h3 className="modal-actions-title">⚡ Recommended Actions — {cardType}</h3>
                    <button className="modal-close" onClick={() => setShowActions(false)}>✕</button>
                </div>
                {loadingActions ? (
                    <div className="modal-actions-loading">
                        <div className="chat-status-dot" />
                        Analyzing...
                    </div>
                ) : (
                    <div className="modal-actions-list">
                        {actions.map((a, i) => (
                            <button
                                key={i}
                                className="action-modal-btn"
                                onClick={() => executeAction(a)}
                            >
                                <span className="action-modal-icon">▶</span>
                                <span className="action-modal-text">{a}</span>
                            </button>
                        ))}
                    </div>
                )}
            </div>
        </div>
    ) : null;

    return (
        <>
            {variant === "inline" ? (
                <div style={{ display: "flex", gap: "8px" }}>
                    <button className="chat-action-btn" style={{ width: "auto", padding: "0 12px", whiteSpace: "nowrap" }} onClick={handleExplain}>
                        <span style={{ marginRight: "4px" }}>🔍</span> Explain
                    </button>
                    <button className="chat-action-btn" style={{ width: "auto", padding: "0 12px", borderColor: "var(--color-primary)", color: "var(--color-primary)", whiteSpace: "nowrap" }} onClick={handleRecommendActions}>
                        <span style={{ marginRight: "4px" }}>⚡</span> Actions
                    </button>
                </div>
            ) : (
                <div ref={menuRef} style={{ position: "relative", display: "inline-block" }}>
                    <button className="explain-btn" onClick={() => setShowMenu((p) => !p)} title="Options">&#8942;</button>
                    {showMenu && (
                        <div className="explain-dropdown">
                            <button className="explain-dropdown-item" onClick={handleExplain}>
                                <span>🔍</span> Explain
                            </button>
                            <button className="explain-dropdown-item" onClick={handleRecommendActions}>
                                <span>⚡</span> Actions
                            </button>
                        </div>
                    )}
                </div>
            )}
            {modal && createPortal(modal, document.body)}
        </>
    );
}

// --- Daily drill-down chart ---
function DailyDrillDown({ month, onBack }: { month: string; onBack: () => void }) {
    const data = generateDailyData(month);
    const maxRev = Math.max(...data.map((d) => d.revenue));

    return (
        <div className="chart-card card">
            <div className="drill-down-header">
                <button className="back-btn" onClick={onBack}>← Back to monthly</button>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <h3 className="chart-title" style={{ margin: 0 }}>Daily Revenue — {month}</h3>
                    <CardOptionsButton cardType={`Daily Revenue ${month}`} cardData={data} variant="inline" />
                </div>
            </div>
            <div className="bar-chart">
                {data.map((d, i) => (
                    <BarWithTooltip
                        key={d.day}
                        height={maxRev > 0 ? (d.revenue / maxRev) * 160 : 0}
                        tooltipLines={[
                            `${d.day.slice(8)} ${month}`,
                            `${formatCurrency(d.revenue)}`,
                            `${formatNumber(d.units)} units`,
                        ]}
                        label={d.day.slice(8)}
                        showValueLabel={false}
                        valueLabel={formatCurrency(d.revenue)}
                        animationDelay={`${i * 20}ms`}
                    />
                ))}
            </div>
        </div>
    );
}

// --- Product Detail Expansion ---
function ProductDetailView({ product, onBack }: { product: TopProduct; onBack: () => void }) {
    const detail = generateProductDetail(product);

    return (
        <div className="card products-section" style={{ padding: 20 }}>
            <div className="drill-down-header">
                <button className="back-btn" onClick={onBack}>← Back to products</button>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <h3 className="chart-title" style={{ margin: 0 }}>{product.name} — Detail</h3>
                    <CardOptionsButton cardType={`Product ${product.name}`} cardData={detail} variant="inline" />
                </div>
            </div>
            <div className="product-detail">
                <h4 style={{ fontSize: 13, fontWeight: 600, marginBottom: 12 }}>Monthly Trend</h4>
                <div className="product-detail-grid">
                    {detail.months.map((m) => (
                        <div key={m.month}>
                            <div className="product-detail-label">{m.month}</div>
                            <div className="product-detail-value">{formatCurrency(m.revenue)}</div>
                            <div className="product-detail-label">{formatNumber(m.units)} units</div>
                        </div>
                    ))}
                    <div>
                        <div className="product-detail-label">Overall Margin</div>
                        <div className="product-detail-value" style={{ color: product.margin_pct > 30 ? "var(--color-success)" : "var(--color-warning)" }}>
                            {product.margin_pct.toFixed(1)}%
                        </div>
                    </div>
                </div>
                <h4 style={{ fontSize: 13, fontWeight: 600, margin: "16px 0 8px" }}>Region Split</h4>
                <div style={{ display: "flex", gap: 12 }}>
                    {detail.topRegions.map((r) => (
                        <div key={r.name} className="badge badge-primary" style={{ padding: "4px 10px" }}>
                            {r.name}: {r.share.toFixed(0)}%
                        </div>
                    ))}
                </div>
            </div>
        </div>
    );
}

export default function DashboardPage() {
    const [kpis, setKpis] = useState<KPIs | null>(null);
    const [salesTrend, setSalesTrend] = useState<SalesTrendPoint[]>([]);
    const [categories, setCategories] = useState<CategoryData[]>([]);
    const [regions, setRegions] = useState<RegionData[]>([]);
    const [topProducts, setTopProducts] = useState<TopProduct[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    // Drill-down state
    const [dailyDrillMonth, setDailyDrillMonth] = useState<string | null>(null);
    const [selectedProduct, setSelectedProduct] = useState<TopProduct | null>(null);
    const [selectedCategory, setSelectedCategory] = useState<CategoryData | null>(null);

    useEffect(() => {
        async function loadData() {
            try {
                const [k, s, c, r, p] = await Promise.all([
                    dashboardAPI.getKPIs(),
                    dashboardAPI.getSalesTrend(),
                    dashboardAPI.getCategoryBreakdown(),
                    dashboardAPI.getRegionalPerformance(),
                    dashboardAPI.getTopProducts(),
                ]);
                setKpis(k); setSalesTrend(s); setCategories(c); setRegions(r); setTopProducts(p);
            } catch (err) {
                setError(err instanceof Error ? err.message : "Failed to load dashboard");
            } finally {
                setLoading(false);
            }
        }
        loadData();
    }, []);

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

    if (loading) {
        return (
            <div style={{ padding: 40, textAlign: "center" }}>
                <div style={{ fontSize: 48, marginBottom: 16, animation: "thinking 1.4s ease-in-out infinite" }}>📊</div>
                <div style={{ color: "var(--color-text-secondary)" }}>Loading dashboard...</div>
            </div>
        );
    }

    const maxCategoryRevenue = Math.max(...categories.map((c) => c.revenue));
    const barColors = [
        "var(--chart-category-1)", "var(--chart-category-2)", "var(--chart-category-3)",
        "var(--chart-category-4)", "var(--chart-category-5)",
    ];

    const maxSalesRev = Math.max(...salesTrend.map((s) => s.revenue));

    return (
        <div>
            <div className="dashboard-header">
                <h1 className="dashboard-title">Dashboard</h1>
                <p className="dashboard-subtitle">Baby &amp; Mother Care — Last 3 months overview</p>
            </div>

            {/* KPI Cards */}
            <div className="kpi-grid">
                {kpis && (
                    <>
                        {[
                            { title: "Total GMV", value: formatCurrency(kpis.total_gmv), change: kpis.gmv_change_pct, icon: "💰" },
                            { title: "Avg Margin", value: `${kpis.avg_margin_pct.toFixed(1)}%`, change: kpis.margin_change_pct, icon: "📈" },
                            { title: "Units Sold", value: formatNumber(kpis.total_units), icon: "📦" },
                            { title: "Active SKUs", value: kpis.active_skus.toString(), icon: "🏷️" },
                        ].map((card, i) => (
                            <div key={card.title} className="card kpi-card animate-fade-in" style={{ animationDelay: `${i * 100}ms` }}>
                                <div className="kpi-card-header">
                                    <div>
                                        <div className="kpi-card-label">{card.title}</div>
                                        <div className="kpi-card-value">{card.value}</div>
                                    </div>
                                    <div className="kpi-card-icon-wrap">
                                        <div className="kpi-card-icon">{card.icon}</div>
                                        <CardOptionsButton cardType={card.title} cardData={kpis} />
                                    </div>
                                </div>
                                {card.change !== undefined && (
                                    <div className={`kpi-change ${card.change >= 0 ? "positive" : "negative"}`}>
                                        <span>{card.change >= 0 ? "↑" : "↓"}</span>
                                        <span>{Math.abs(card.change).toFixed(1)}%</span>
                                        <span className="kpi-change-label">vs prev quarter</span>
                                    </div>
                                )}
                            </div>
                        ))}
                    </>
                )}
            </div>

            {/* Charts Row */}
            <div className="charts-row">
                {/* Sales Trend or Daily Drill-Down */}
                {dailyDrillMonth ? (
                    <DailyDrillDown month={dailyDrillMonth} onBack={() => setDailyDrillMonth(null)} />
                ) : (
                    <div className="card chart-card">
                        <div className="chart-header">
                            <h3 className="chart-title">Revenue Trend (Monthly)</h3>
                            <CardOptionsButton cardType="Sales Trend" cardData={salesTrend} />
                        </div>
                        <div className="bar-chart">
                            {salesTrend.map((point, i) => (
                                <BarWithTooltip
                                    key={point.month}
                                    height={maxSalesRev > 0 ? (point.revenue / maxSalesRev) * 160 : 0}
                                    tooltipLines={[
                                        point.month,
                                        `Revenue: ${formatCurrency(point.revenue)}`,
                                        `Units: ${formatNumber(point.units ?? 0)}`,
                                    ]}
                                    onClick={() => setDailyDrillMonth(point.month)}
                                    label={point.month.slice(5)}
                                    showValueLabel={true}
                                    valueLabel={formatCurrency(point.revenue)}
                                    animationDelay={`${i * 50}ms`}
                                />
                            ))}
                        </div>
                    </div>
                )}

                {/* Category Breakdown or Drill-Down */}
                {selectedCategory ? (
                    <div className="card chart-card">
                        <div className="drill-down-header">
                            <button className="back-btn" onClick={() => setSelectedCategory(null)}>← Back to categories</button>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                                <h3 className="chart-title" style={{ margin: 0 }}>{selectedCategory.name} — Details</h3>
                                <CardOptionsButton cardType={`Category ${selectedCategory.name}`} cardData={selectedCategory} variant="inline" />
                            </div>
                        </div>
                        <div className="product-detail">
                            <div className="product-detail-grid">
                                <div>
                                    <div className="product-detail-label">Revenue</div>
                                    <div className="product-detail-value">{formatCurrency(selectedCategory.revenue)}</div>
                                </div>
                                <div>
                                    <div className="product-detail-label">Units</div>
                                    <div className="product-detail-value">{formatNumber(selectedCategory.units)}</div>
                                </div>
                                <div>
                                    <div className="product-detail-label">Margin</div>
                                    <div className="product-detail-value">{selectedCategory.margin.toFixed(1)}%</div>
                                </div>
                                <div>
                                    <div className="product-detail-label">SKUs</div>
                                    <div className="product-detail-value">{selectedCategory.sku_count}</div>
                                </div>
                            </div>
                            <h4 style={{ fontSize: 13, fontWeight: 600, margin: "16px 0 8px" }}>Sub-Categories</h4>
                            <table className="products-table">
                                <thead>
                                    <tr><th>Sub-Category</th><th>Revenue</th><th>Units</th><th>Margin</th></tr>
                                </thead>
                                <tbody>
                                    {["Premium", "Standard", "Economy", "Organic"].map((sub, i) => (
                                        <tr key={sub}>
                                            <td>{sub} {selectedCategory.name}</td>
                                            <td>{formatCurrency(selectedCategory.revenue * (0.4 - i * 0.08))}</td>
                                            <td>{formatNumber(Math.floor(selectedCategory.units * (0.35 - i * 0.05)))}</td>
                                            <td>{(selectedCategory.margin + (2 - i) * 3).toFixed(1)}%</td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>
                    </div>
                ) : (
                    <div className="card chart-card">
                        <div className="chart-header">
                            <h3 className="chart-title">By Category</h3>
                            <CardOptionsButton cardType="Category Breakdown" cardData={categories} />
                        </div>
                        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
                            {categories.map((item, i) => (
                                <div
                                    key={item.name}
                                    className="category-row animate-fade-in"
                                    style={{ animationDelay: `${i * 100}ms`, cursor: "pointer" }}
                                    onClick={() => setSelectedCategory(item)}
                                    title={`Click to drill down into ${item.name}`}
                                >
                                    <div className="category-bar-header">
                                        <span className="category-bar-name">{item.name}</span>
                                        <span className="category-bar-value">{formatCurrency(item.revenue)}</span>
                                    </div>
                                    <div className="category-bar-track">
                                        <div
                                            className="category-bar-fill"
                                            style={{
                                                width: `${(item.revenue / maxCategoryRevenue) * 100}%`,
                                                background: barColors[i % barColors.length],
                                            }}
                                        />
                                    </div>
                                </div>
                            ))}
                        </div>
                    </div>
                )}
            </div>

            {/* Regional Performance */}
            <div className="region-section">
                <div className="chart-header">
                    <h3 className="chart-title">Regional Performance</h3>
                    <CardOptionsButton cardType="Regional Performance" cardData={regions} />
                </div>
                <div className="region-grid">
                    {regions.map((r, i) => {
                        const marginPct = r.revenue > 0 ? (r.margin / r.revenue) * 100 : 0;
                        return (
                            <div key={r.name} className="card region-card animate-fade-in" style={{ animationDelay: `${i * 100}ms` }}>
                                <div className="region-name">{r.name}</div>
                                <div className="region-stats">
                                    <div><div className="region-stat-label">Revenue</div><div className="region-stat-value">{formatCurrency(r.revenue)}</div></div>
                                    <div><div className="region-stat-label">Margin</div><div className="region-stat-value" style={{ color: marginPct > 30 ? "var(--color-success)" : "var(--color-warning)" }}>{marginPct.toFixed(1)}%</div></div>
                                    <div><div className="region-stat-label">Units</div><div className="region-stat-value">{formatNumber(r.units)}</div></div>
                                    <div><div className="region-stat-label">Avg Discount</div><div className="region-stat-value">{r.avg_discount.toFixed(1)}%</div></div>
                                </div>
                                <div style={{ marginTop: 12, borderTop: "1px solid var(--color-border)", paddingTop: 12 }}>
                                    <CardOptionsButton cardType={`Region ${r.name}`} cardData={r} variant="inline" />
                                </div>
                            </div>
                        );
                    })}
                </div>
            </div>

            {/* Top Products Table or Detail */}
            {selectedProduct ? (
                <ProductDetailView product={selectedProduct} onBack={() => setSelectedProduct(null)} />
            ) : (
                <div className="card products-section" style={{ padding: 20 }}>
                    <div className="chart-header">
                        <h3 className="chart-title">Top 10 Products</h3>
                        <CardOptionsButton cardType="Top Products" cardData={topProducts} />
                    </div>
                    <table className="products-table">
                        <thead>
                            <tr>
                                <th>Product</th>
                                <th>Category</th>
                                <th className="align-right">Revenue</th>
                                <th className="align-right">Units</th>
                                <th className="align-right">Margin %</th>
                            </tr>
                        </thead>
                        <tbody>
                            {topProducts.map((p, i) => (
                                <tr key={p.name} className="animate-fade-in" style={{ animationDelay: `${i * 50}ms` }} onClick={() => setSelectedProduct(p)}>
                                    <td>
                                        <div className="product-name">{p.name}</div>
                                        <div className="product-brand">{p.brand}</div>
                                    </td>
                                    <td><span className="badge badge-primary">{p.category}</span></td>
                                    <td className="align-right product-revenue">{formatCurrency(p.revenue)}</td>
                                    <td className="align-right">{formatNumber(p.units)}</td>
                                    <td className="align-right">
                                        <span style={{ color: p.margin_pct > 30 ? "var(--color-success)" : p.margin_pct > 20 ? "var(--color-warning)" : "var(--color-danger)", fontWeight: 600 }}>
                                            {p.margin_pct.toFixed(1)}%
                                        </span>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    );
}
