import { useState } from "react";

interface NewsItem {
    id: string;
    title: string;
    category: string;
    summary: string;
    timestamp: string;
    impact: "high" | "medium" | "low";
    source: string;
}

const mockNews: NewsItem[] = [
    {
        id: "1",
        title: "Pampers launches new premium diaper line in India",
        category: "Competitor",
        summary: "P&G announced the launch of Pampers Premium Care Ultra in Q2 2026, targeting the premium baby care segment in metro cities. The new line features advanced absorption technology and eco-friendly materials, with pricing set 15-20% above current market offerings. This move signals P&G's intent to capture the rapidly growing premium segment that has been expanding at 25% annually.",
        timestamp: "2 hours ago",
        impact: "high",
        source: "Economic Times",
    },
    {
        id: "2",
        title: "Baby care market in India projected to grow 14% YoY",
        category: "Market",
        summary: "Research firm Redseer projects the Indian baby care market to reach ₹45,000 Cr by 2027, driven by rising urbanization and nuclear families. Key growth drivers include increasing disposable income in Tier-2 cities, rising awareness about baby hygiene, and growing preference for branded products over unorganized alternatives.",
        timestamp: "5 hours ago",
        impact: "high",
        source: "Redseer Report",
    },
    {
        id: "3",
        title: "GST rate reduction on baby food products proposed",
        category: "Regulatory",
        summary: "The GST Council is considering reducing rates on baby food from 18% to 5%, which could significantly impact pricing strategies. Industry bodies have long advocated for this reduction, arguing that baby food is an essential item. If passed, this could reduce retail prices by 10-12% and boost volumes by an estimated 20%.",
        timestamp: "1 day ago",
        impact: "high",
        source: "Mint",
    },
    {
        id: "4",
        title: "FirstCry expands to Tier-2 cities with aggressive pricing",
        category: "Competitor",
        summary: "FirstCry opened 50 new stores in Tier-2 cities with 20-30% discounts on baby care products, intensifying competition. The company is reportedly investing ₹500 Cr in expansion over the next 18 months, targeting cities like Lucknow, Jaipur, Chandigarh, and Bhubaneswar.",
        timestamp: "1 day ago",
        impact: "medium",
        source: "Livemint",
    },
    {
        id: "5",
        title: "Organic baby skincare products gain traction in South India",
        category: "Trend",
        summary: "Sales of organic baby skincare products in Chennai and Bangalore grew 45% QoQ, driven by health-conscious millennial parents. Brands like Mamaearth and The Moms Co. are leading this trend with natural ingredient-based formulations. This shift presents opportunities for category expansion.",
        timestamp: "2 days ago",
        impact: "medium",
        source: "Internal Analytics",
    },
    {
        id: "6",
        title: "Monsoon season expected to impact supply chain in East",
        category: "Supply Chain",
        summary: "Heavy rainfall forecasts for Eastern India may disrupt logistics and warehouse operations through August 2026. Advance stocking and alternative routing plans are recommended for the Kolkata and Patna distribution centers to minimize impact on delivery timelines.",
        timestamp: "3 days ago",
        impact: "low",
        source: "Weather Service",
    },
];

const impactColors: Record<string, string> = { high: "#ef4444", medium: "#f59e0b", low: "#22c55e" };
const categoryColors: Record<string, string> = {
    Competitor: "var(--color-primary)", Market: "#3b82f6",
    Regulatory: "#ef4444", Trend: "#22c55e", "Supply Chain": "#f59e0b",
};

export default function NewsPage() {
    const [filter, setFilter] = useState<string>("all");
    const [selectedNews, setSelectedNews] = useState<NewsItem | null>(null);

    const filteredNews = filter === "all" ? mockNews : mockNews.filter((n) => n.category === filter);
    const categories = ["all", ...Array.from(new Set(mockNews.map((n) => n.category)))];

    return (
        <div className="news-page">
            <div className="news-header">
                <div>
                    <h1 className="news-title">📰 Market News &amp; Intelligence</h1>
                    <p className="news-subtitle">Real-time market intelligence and competitor tracking</p>
                </div>
            </div>

            <div className="news-filters">
                {categories.map((cat) => (
                    <button
                        key={cat}
                        onClick={() => setFilter(cat)}
                        className={`news-filter-btn ${filter === cat ? "active" : ""}`}
                    >
                        {cat}
                    </button>
                ))}
            </div>

            <div className="news-list">
                {filteredNews.map((item) => (
                    <div key={item.id} className="news-card" onClick={() => setSelectedNews(item)}>
                        <div className="news-card-top">
                            <div className="news-card-badges">
                                <span
                                    className="news-badge"
                                    style={{
                                        background: (categoryColors[item.category] || "#666") + "20",
                                        color: categoryColors[item.category] || "#666",
                                    }}
                                >
                                    {item.category}
                                </span>
                                <span
                                    className="news-impact-badge"
                                    style={{
                                        background: impactColors[item.impact] + "20",
                                        color: impactColors[item.impact],
                                    }}
                                >
                                    {item.impact} impact
                                </span>
                            </div>
                            <span className="news-timestamp">{item.timestamp}</span>
                        </div>
                        <h3 className="news-card-title">{item.title}</h3>
                        <p className="news-card-summary">{item.summary.slice(0, 120)}...</p>
                        <div className="news-card-source">Source: {item.source}</div>
                    </div>
                ))}
            </div>

            {/* News Detail Modal */}
            {selectedNews && (
                <div className="modal-backdrop" onClick={() => setSelectedNews(null)}>
                    <div className="modal-content" onClick={(e) => e.stopPropagation()}>
                        <div className="modal-header">
                            <h2 className="modal-title">{selectedNews.title}</h2>
                            <button className="modal-close" onClick={() => setSelectedNews(null)}>✕</button>
                        </div>
                        <div className="modal-meta">
                            <span
                                className="news-badge"
                                style={{
                                    background: (categoryColors[selectedNews.category] || "#666") + "20",
                                    color: categoryColors[selectedNews.category] || "#666",
                                }}
                            >
                                {selectedNews.category}
                            </span>
                            <span
                                className="news-impact-badge"
                                style={{
                                    background: impactColors[selectedNews.impact] + "20",
                                    color: impactColors[selectedNews.impact],
                                }}
                            >
                                {selectedNews.impact} impact
                            </span>
                            <span className="news-timestamp">{selectedNews.timestamp}</span>
                            <span className="news-card-source">Source: {selectedNews.source}</span>
                        </div>
                        <div className="modal-body">
                            <p>{selectedNews.summary}</p>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
