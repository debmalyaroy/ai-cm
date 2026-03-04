-- =============================================================================
-- AI-CM: Schema Descriptions (COMMENT ON)
-- Purpose: Provide semantic metadata for LLM-driven SQL generation.
-- The SchemaCache reads these via pg_description to build prompt context.
-- =============================================================================

-- ===================== DIMENSION TABLES =====================

-- dim_locations
COMMENT ON TABLE dim_locations IS 'Geographic hierarchy (region → state → city). JOIN with fact tables via location_id.';
COMMENT ON COLUMN dim_locations.id IS 'Primary key (UUID)';
COMMENT ON COLUMN dim_locations.region IS 'Top-level geographic region (e.g. North, South, East, West)';
COMMENT ON COLUMN dim_locations.state IS 'State within the region';
COMMENT ON COLUMN dim_locations.city IS 'City within the state';
COMMENT ON COLUMN dim_locations.pincode IS 'Postal/ZIP code (nullable)';

-- dim_sellers
COMMENT ON TABLE dim_sellers IS 'Seller/vendor directory. JOIN via dim_products.seller_id → dim_sellers.id.';
COMMENT ON COLUMN dim_sellers.id IS 'Primary key (UUID)';
COMMENT ON COLUMN dim_sellers.name IS 'Seller display name';
COMMENT ON COLUMN dim_sellers.email IS 'Seller contact email (nullable)';
COMMENT ON COLUMN dim_sellers.region IS 'Seller home region';
COMMENT ON COLUMN dim_sellers.rating IS 'Seller performance rating (0.00–5.00)';
COMMENT ON COLUMN dim_sellers.status IS 'active or inactive. Filter active sellers with status = ''active''';

-- dim_products
COMMENT ON TABLE dim_products IS 'Product catalog with pricing info. JOIN with fact tables via product_id.';
COMMENT ON COLUMN dim_products.id IS 'Primary key (UUID)';
COMMENT ON COLUMN dim_products.sku IS 'Unique Stock Keeping Unit identifier';
COMMENT ON COLUMN dim_products.name IS 'Product display name';
COMMENT ON COLUMN dim_products.category IS 'Top-level product category (e.g. Diapers, Formula, Breast Pumps)';
COMMENT ON COLUMN dim_products.sub_category IS 'Sub-category within category (nullable)';
COMMENT ON COLUMN dim_products.brand IS 'Product brand name (nullable)';
COMMENT ON COLUMN dim_products.mrp IS 'Maximum Retail Price — the list price before any discount';
COMMENT ON COLUMN dim_products.cost_price IS 'Wholesale cost to the platform — used to calculate margin';
COMMENT ON COLUMN dim_products.seller_id IS 'FK → dim_sellers.id — the vendor selling this product';
COMMENT ON COLUMN dim_products.status IS 'active or inactive';

-- ===================== FACT TABLES =====================

-- fact_sales
COMMENT ON TABLE fact_sales IS 'Individual sale transactions. Contains revenue and margin data. JOIN via product_id, location_id.';
COMMENT ON COLUMN fact_sales.id IS 'Primary key (UUID)';
COMMENT ON COLUMN fact_sales.product_id IS 'FK → dim_products.id';
COMMENT ON COLUMN fact_sales.location_id IS 'FK → dim_locations.id';
COMMENT ON COLUMN fact_sales.sale_date IS 'Date of the sale transaction';
COMMENT ON COLUMN fact_sales.quantity IS 'Number of units sold';
COMMENT ON COLUMN fact_sales.selling_price IS 'Actual unit price charged to customer (after discount)';
COMMENT ON COLUMN fact_sales.discount_pct IS 'Discount percentage applied (0–100), default 0';
COMMENT ON COLUMN fact_sales.revenue IS 'Total revenue = quantity × selling_price';
COMMENT ON COLUMN fact_sales.margin IS 'Profit margin = revenue − (quantity × cost_price). THIS is where margin data lives — NOT in dim_products';

-- fact_inventory
COMMENT ON TABLE fact_inventory IS 'Daily inventory snapshots per product per location. JOIN via product_id, location_id.';
COMMENT ON COLUMN fact_inventory.id IS 'Primary key (UUID)';
COMMENT ON COLUMN fact_inventory.product_id IS 'FK → dim_products.id';
COMMENT ON COLUMN fact_inventory.location_id IS 'FK → dim_locations.id';
COMMENT ON COLUMN fact_inventory.stock_date IS 'Date of the inventory snapshot';
COMMENT ON COLUMN fact_inventory.quantity_on_hand IS 'Current units in stock. There is NO column called "stock" — always use quantity_on_hand';
COMMENT ON COLUMN fact_inventory.reorder_level IS 'Minimum stock threshold before reorder is triggered (default 50)';
COMMENT ON COLUMN fact_inventory.days_of_supply IS 'Estimated days the current stock will last (nullable)';

-- fact_competitor_prices
COMMENT ON TABLE fact_competitor_prices IS 'Competitor pricing intelligence per product. JOIN via product_id.';
COMMENT ON COLUMN fact_competitor_prices.id IS 'Primary key (UUID)';
COMMENT ON COLUMN fact_competitor_prices.product_id IS 'FK → dim_products.id';
COMMENT ON COLUMN fact_competitor_prices.competitor_name IS 'Name of the competing retailer/platform';
COMMENT ON COLUMN fact_competitor_prices.competitor_price IS 'Price offered by the competitor';
COMMENT ON COLUMN fact_competitor_prices.price_date IS 'Date the competitor price was observed';
COMMENT ON COLUMN fact_competitor_prices.price_diff_pct IS 'Percentage difference: (our_price − competitor_price) / our_price × 100';

-- fact_forecasts
COMMENT ON TABLE fact_forecasts IS 'Demand forecasts per product per location. JOIN via product_id, location_id.';
COMMENT ON COLUMN fact_forecasts.id IS 'Primary key (UUID)';
COMMENT ON COLUMN fact_forecasts.product_id IS 'FK → dim_products.id';
COMMENT ON COLUMN fact_forecasts.location_id IS 'FK → dim_locations.id';
COMMENT ON COLUMN fact_forecasts.forecast_date IS 'Target date for the forecast';
COMMENT ON COLUMN fact_forecasts.predicted_quantity IS 'Predicted units of demand';
COMMENT ON COLUMN fact_forecasts.predicted_revenue IS 'Predicted revenue (nullable)';
COMMENT ON COLUMN fact_forecasts.confidence_score IS 'Model confidence (0.00–1.00, default 0.50)';
COMMENT ON COLUMN fact_forecasts.model_version IS 'ML model version that produced this forecast';

-- ===================== OPERATIONAL TABLES =====================

-- alerts
COMMENT ON TABLE alerts IS 'System alerts and anomaly notifications generated by the Watchdog agent.';
COMMENT ON COLUMN alerts.id IS 'Primary key (UUID)';
COMMENT ON COLUMN alerts.title IS 'Alert headline';
COMMENT ON COLUMN alerts.severity IS 'Alert severity: low, medium, high, critical';
COMMENT ON COLUMN alerts.category IS 'Alert category (e.g. pricing, inventory, sales)';
COMMENT ON COLUMN alerts.message IS 'Detailed alert description';
COMMENT ON COLUMN alerts.acknowledged IS 'Whether the alert has been reviewed (true/false)';

-- action_log
COMMENT ON TABLE action_log IS 'Audit log of actions proposed and executed by agents.';
COMMENT ON COLUMN action_log.id IS 'Primary key (UUID)';
COMMENT ON COLUMN action_log.title IS 'Action title';
COMMENT ON COLUMN action_log.description IS 'Detailed action description (nullable)';
COMMENT ON COLUMN action_log.action_type IS 'Type: price_update, inventory_adjustment, promotion, etc.';
COMMENT ON COLUMN action_log.category IS 'Product category this action relates to';
COMMENT ON COLUMN action_log.product_id IS 'FK → dim_products.id (nullable)';
COMMENT ON COLUMN action_log.confidence_score IS 'Agent confidence in this action (0.00–1.00)';
COMMENT ON COLUMN action_log.status IS 'pending, approved, rejected, executed';
