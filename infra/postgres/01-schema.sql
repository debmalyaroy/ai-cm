-- =============================================================================
-- AI-CM: Schema Definition
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";

-- DIMENSION TABLES
CREATE TABLE dim_locations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    region VARCHAR(50) NOT NULL,
    state VARCHAR(100) NOT NULL,
    city VARCHAR(100) NOT NULL,
    pincode VARCHAR(10),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE dim_sellers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL,
    email VARCHAR(200),
    region VARCHAR(50),
    rating DECIMAL(3,2) DEFAULT 0,
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE dim_products (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sku VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(200) NOT NULL,
    category VARCHAR(100) NOT NULL,
    sub_category VARCHAR(100),
    brand VARCHAR(100),
    mrp DECIMAL(12,2) NOT NULL,
    cost_price DECIMAL(12,2) NOT NULL,
    seller_id UUID REFERENCES dim_sellers(id),
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- FACT TABLES
CREATE TABLE fact_sales (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID REFERENCES dim_products(id),
    location_id UUID REFERENCES dim_locations(id),
    sale_date DATE NOT NULL,
    quantity INTEGER NOT NULL,
    selling_price DECIMAL(12,2) NOT NULL,
    discount_pct DECIMAL(5,2) DEFAULT 0,
    revenue DECIMAL(14,2) NOT NULL,
    margin DECIMAL(14,2) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE fact_inventory (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID REFERENCES dim_products(id),
    location_id UUID REFERENCES dim_locations(id),
    stock_date DATE NOT NULL,
    quantity_on_hand INTEGER NOT NULL,
    reorder_level INTEGER DEFAULT 50,
    days_of_supply INTEGER,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE fact_competitor_prices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID REFERENCES dim_products(id),
    competitor_name VARCHAR(200) NOT NULL,
    competitor_price DECIMAL(12,2) NOT NULL,
    price_date DATE NOT NULL,
    price_diff_pct DECIMAL(5,2),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE fact_forecasts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID REFERENCES dim_products(id),
    location_id UUID REFERENCES dim_locations(id),
    forecast_date DATE NOT NULL,
    predicted_quantity INTEGER NOT NULL,
    predicted_revenue DECIMAL(14,2),
    confidence_score DECIMAL(3,2) DEFAULT 0.5,
    model_version VARCHAR(50) DEFAULT 'v1',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- OPERATIONAL TABLES
CREATE TABLE chat_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_name VARCHAR(100) DEFAULT 'demo_user',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE chat_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE action_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(300) NOT NULL,
    description TEXT,
    action_type VARCHAR(50) NOT NULL,
    category VARCHAR(100),
    product_id UUID REFERENCES dim_products(id),
    confidence_score DECIMAL(3,2) DEFAULT 0,
    status VARCHAR(20) DEFAULT 'pending',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE action_comments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    action_id UUID REFERENCES action_log(id) ON DELETE CASCADE,
    user_name VARCHAR(100) DEFAULT 'demo_user',
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE cron_jobs (
    id VARCHAR(100) PRIMARY KEY,
    locked_by VARCHAR(100),
    locked_at TIMESTAMPTZ,
    last_run TIMESTAMPTZ,
    next_run TIMESTAMPTZ,
    status VARCHAR(20) DEFAULT 'idle',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- AGENT / MEMORY TABLES
CREATE TABLE agent_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(100) DEFAULT 'demo_user',
    agent_type VARCHAR(50) NOT NULL,
    status VARCHAR(20) DEFAULT 'active',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE agent_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID REFERENCES agent_sessions(id) ON DELETE CASCADE,
    agent_type VARCHAR(50) NOT NULL,
    role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    reasoning JSONB DEFAULT '[]',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE agent_actions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID REFERENCES agent_sessions(id),
    agent_type VARCHAR(50) NOT NULL,
    action_type VARCHAR(50) NOT NULL,
    title VARCHAR(300) NOT NULL,
    description TEXT,
    confidence_score DECIMAL(3,2) DEFAULT 0,
    status VARCHAR(20) DEFAULT 'pending',
    approved_by VARCHAR(100),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE business_context (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    content TEXT NOT NULL,
    embedding vector(1536),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE agent_memory (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_type VARCHAR(50) NOT NULL,
    memory_type VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    embedding vector(1536),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(300) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    category VARCHAR(100) NOT NULL,
    message TEXT NOT NULL,
    acknowledged BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- INDEXES
CREATE INDEX idx_fact_sales_date ON fact_sales(sale_date);
CREATE INDEX idx_fact_sales_product ON fact_sales(product_id);
CREATE INDEX idx_fact_sales_location ON fact_sales(location_id);
CREATE INDEX idx_fact_inventory_product ON fact_inventory(product_id);
CREATE INDEX idx_fact_inventory_date ON fact_inventory(stock_date);
CREATE INDEX idx_fact_competitor_product ON fact_competitor_prices(product_id);
CREATE INDEX idx_fact_competitor_date ON fact_competitor_prices(price_date);
CREATE INDEX idx_fact_forecasts_date ON fact_forecasts(forecast_date);
CREATE INDEX idx_fact_forecasts_product ON fact_forecasts(product_id);
CREATE INDEX idx_action_log_status ON action_log(status);
CREATE INDEX idx_chat_messages_session ON chat_messages(session_id);
CREATE INDEX idx_agent_sessions_user ON agent_sessions(user_id);
CREATE INDEX idx_agent_messages_session ON agent_messages(session_id);
CREATE INDEX idx_agent_memory_type ON agent_memory(memory_type);

CREATE INDEX idx_business_context_embedding ON business_context
    USING ivfflat (embedding vector_cosine_ops) WITH (lists = 10);
CREATE INDEX idx_agent_memory_embedding ON agent_memory
    USING ivfflat (embedding vector_cosine_ops) WITH (lists = 10);

CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(acknowledged);
CREATE INDEX IF NOT EXISTS idx_alerts_date ON alerts(created_at);
