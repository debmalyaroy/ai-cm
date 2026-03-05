-- =============================================================================
-- AI-CM: Inventory, Competitor, Forecasts, Business Context, Actions, Memory
-- =============================================================================

-- INVENTORY (200 products × 30 locations = 6000 rows)
DO $$
DECLARE
    p_rec RECORD; l_rec RECORD;
    stock INTEGER; reorder INTEGER; avg_daily INTEGER;
    loc_region TEXT; cat TEXT;
BEGIN
    FOR p_rec IN SELECT id, category FROM dim_products LOOP
        cat := p_rec.category;
        FOR l_rec IN SELECT id, region FROM dim_locations LOOP
            loc_region := l_rec.region;
            stock := CASE
                WHEN cat IN ('Diapers','Baby Wipes','Baby Skincare','Baby Cereals','Baby Formula','Baby Snacks','Baby Grooming','Baby Bath') THEN (80 + random() * 350)::INT
                WHEN cat IN ('Baby Furniture','Strollers','Car Seats') THEN (5 + random() * 20)::INT
                WHEN cat IN ('Baby Carriers','Baby Bedding','Baby Safety') THEN (10 + random() * 40)::INT
                WHEN cat IN ('Breast Pumps','Maternity Care') THEN (20 + random() * 100)::INT
                WHEN cat IN ('Baby Toys','Baby Clothing','Baby Gifting') THEN (30 + random() * 150)::INT
                ELSE (50 + random() * 200)::INT
            END;
            reorder := CASE
                WHEN cat IN ('Diapers','Baby Wipes','Baby Cereals','Baby Formula') THEN (40 + random() * 30)::INT
                WHEN cat IN ('Baby Furniture','Strollers','Car Seats') THEN (3 + random() * 5)::INT
                ELSE (20 + random() * 20)::INT
            END;
            avg_daily := GREATEST((stock / (10 + random() * 25))::INT, 1);
            IF loc_region = 'East' AND random() > 0.4 THEN stock := (3 + random() * 18)::INT; END IF;
            IF loc_region = 'West' AND cat = 'Baby Furniture' THEN stock := (30 + random() * 50)::INT; END IF;
            INSERT INTO fact_inventory (product_id, location_id, stock_date, quantity_on_hand, reorder_level, days_of_supply)
            VALUES (p_rec.id, l_rec.id, CURRENT_DATE, stock, reorder, stock / GREATEST(avg_daily, 1));
        END LOOP;
    END LOOP;
END $$;

-- COMPETITOR PRICES (~2000 rows)
INSERT INTO fact_competitor_prices (product_id, competitor_name, competitor_price, price_date, price_diff_pct)
SELECT p.id, comp.name, p.mrp * (1 - (comp.discount / 100.0)), d.price_date, -comp.discount
FROM dim_products p
CROSS JOIN (VALUES ('Flipkart',8.0),('Amazon',12.0),('Meesho',15.0),('BigBasket',5.0),('FirstCry',10.0)) AS comp(name, discount)
CROSS JOIN (VALUES (DATE '2026-02-14'),(DATE '2026-02-18'),(DATE '2026-02-22')) AS d(price_date)
WHERE p.category IN ('Diapers','Baby Wipes','Baby Skincare','Baby Cereals','Baby Formula','Baby Snacks','Feeding Bottles','Baby Grooming','Baby Bath');

INSERT INTO fact_competitor_prices (product_id, competitor_name, competitor_price, price_date, price_diff_pct)
SELECT p.id, comp.name, p.mrp * (1 - (comp.discount / 100.0)), comp.price_date, -comp.discount
FROM dim_products p
CROSS JOIN (VALUES ('Flipkart',6.0,DATE '2026-02-20'),('Amazon',9.0,DATE '2026-02-21'),('FirstCry',7.0,DATE '2026-02-19')) AS comp(name, discount, price_date)
WHERE p.category IN ('Baby Toys','Baby Clothing','Baby Gifting','Baby Safety','Maternity Care','Breast Pumps');

-- FORECASTS (~2000 rows)
DO $$
DECLARE p_rec RECORD; l_rec RECORD; fdate DATE; pred_qty INTEGER;
BEGIN
    FOR fdate IN SELECT generate_series(DATE '2026-02-24', DATE '2026-03-31', INTERVAL '1 week')::DATE
    LOOP
        FOR p_rec IN SELECT id, category, mrp FROM dim_products LOOP
            FOR l_rec IN SELECT id FROM dim_locations LOOP
                IF random() > 0.15 THEN CONTINUE; END IF;
                pred_qty := CASE
                    WHEN p_rec.category IN ('Diapers','Baby Wipes','Baby Cereals') THEN (10 + random() * 25)::INT
                    WHEN p_rec.category IN ('Baby Furniture','Strollers','Car Seats') THEN (0 + random() * 3)::INT
                    ELSE (3 + random() * 12)::INT
                END;
                INSERT INTO fact_forecasts (product_id, location_id, forecast_date, predicted_quantity, predicted_revenue, confidence_score, model_version)
                VALUES (p_rec.id, l_rec.id, fdate, pred_qty, pred_qty * p_rec.mrp * 0.9, (0.65 + random() * 0.3)::DECIMAL(3,2), 'v1.2');
            END LOOP;
        END LOOP;
    END LOOP;
END $$;

-- BUSINESS CONTEXT (policy & market docs)
INSERT INTO business_context (content, metadata) VALUES
('Pricing Policy: Products must maintain minimum 25% margin. Price matches allowed within 5% of competitor lowest. Seasonal promotions capped at 20% discount. Flash sales require senior manager approval.', '{"type":"policy","domain":"pricing","version":"2.1"}'),
('Inventory Policy: Reorder triggered when stock drops below reorder_level. Safety stock = 2 weeks average demand. Fast-moving categories (Diapers, Wipes) require 3 weeks buffer. Slow-moving (Furniture) = 6 weeks.', '{"type":"policy","domain":"inventory","version":"1.5"}'),
('Seller Compliance: All sellers must maintain 4.0+ rating. Response time SLA is 24 hours. Return rate must stay under 5%. Quarterly performance review determines tier.', '{"type":"policy","domain":"sellers","version":"1.0"}'),
('Market Trend Q1 2026: Baby care growing 14% YoY in India. Organic/natural products segment up 28%. Premium brands outperforming mass market in South and West. E-commerce penetration highest in metros.', '{"type":"market_intel","domain":"trends","quarter":"Q1-2026"}'),
('Regional Insight: South India strongest for premium diapers and feeding products. North favors baby food categories. East region showing 15% sales decline since Dec 2025. West over-indexed on gifting and furniture.', '{"type":"market_intel","domain":"regional","quarter":"Q1-2026"}'),
('Competitor Analysis: Amazon offering flat 12% discount on baby care. FirstCry bundling free wipes with diaper purchases. Flipkart loyalty program gives 8% cashback. Meesho aggressive on value segment.', '{"type":"competitor","domain":"pricing","date":"2026-02"}'),
('Category Strategy: Focus on expanding organic baby food portfolio. Reduce dependency on single-brand categories. Increase private label share in Grooming and Bath. Target 30% margin improvement in Clothing.', '{"type":"strategy","domain":"category","fiscal_year":"FY2026"}'),
('Communication Templates: Price match notifications go to seller within 2 hours. Stockout alerts escalate to category manager. Weekly performance reports auto-generated every Monday. Quarterly reviews in March, June, Sep, Dec.', '{"type":"policy","domain":"communication","version":"1.2"}');

-- INITIAL ACTIONS (for demo)
INSERT INTO action_log (title, description, action_type, category, product_id, confidence_score, status) VALUES
('Price Match: Pampers Active Baby Small', 'Amazon selling at 12% lower (₹703 vs MRP ₹799). Market share at risk. Recommend matching to ₹710.', 'price_match', 'Diapers', 'c0000001-0000-0000-0000-000000000001', 0.87, 'pending'),
('Price Match: Huggies Wonder Pants Medium', 'Meesho offering 15% discount. Three competitors undercutting MRP. Urgent price action needed.', 'price_match', 'Diapers', 'c0000001-0000-0000-0000-000000000005', 0.91, 'pending'),
('Restock Alert: Huggies Wonder Pants Small (East)', 'East India stock critically low (<15 units across 3 locations). Predicted stockout in 5 days.', 'restock', 'Diapers', 'c0000001-0000-0000-0000-000000000004', 0.92, 'pending'),
('Restock Alert: Cerelac Wheat Apple (East)', 'Kolkata and Patna below reorder level. Baby food demand steady. Recommend immediate PO.', 'restock', 'Baby Cereals', 'c0000001-0000-0000-0000-000000000031', 0.85, 'pending'),
('Run Promotion: LuvLap Galaxy Cradle (West)', 'West region inventory 2.5x monthly demand. 45 days supply. Recommend 10-15% discount.', 'promotion', 'Baby Furniture', 'c0000001-0000-0000-0000-000000000061', 0.74, 'pending'),
('Run Promotion: Chicco Polly Chair', 'Slow-moving SKU with 60+ days supply. Consider bundling with stroller for clearance.', 'promotion', 'Baby Furniture', 'c0000001-0000-0000-0000-000000000065', 0.68, 'pending'),
('Delist: Prenatal Vitamins', 'Sales declined 40% over 3 months. Lowest margin in Maternity Care category.', 'delist', 'Maternity Care', 'c0000001-0000-0000-0000-000000000111', 0.65, 'pending'),
('Brand Switch: Pampers to MamyPoko (South)', 'MamyPoko gaining 4% weekly share in South at Pampers expense.', 'price_match', 'Diapers', NULL, 0.79, 'pending');

-- AGENT MEMORY SEEDS
INSERT INTO agent_memory (agent_type, memory_type, content, metadata) VALUES
('analyst', 'semantic', 'Table dim_products has columns: id, sku, name, category, sub_category, brand, mrp, cost_price, seller_id, status. Categories include Diapers, Baby Wipes, Baby Skincare, Baby Cereals, Baby Formula, Baby Snacks, Baby Furniture, Strollers, Car Seats, Feeding Bottles, Breast Pumps, Maternity Care, Baby Toys, Baby Clothing, Baby Bedding, Baby Carriers, Baby Bath, Baby Safety, Baby Grooming, Baby Gifting.', '{"source":"schema","table":"dim_products"}'),
('analyst', 'semantic', 'Table fact_sales has columns: id, product_id, location_id, sale_date, quantity, selling_price, discount_pct, revenue, margin. Date range: Aug 2024 to Feb 2026.', '{"source":"schema","table":"fact_sales"}'),
('analyst', 'semantic', 'Table dim_locations has 30 cities across 4 regions: North (8), South (7), East (7), West (8). All Indian cities.', '{"source":"schema","table":"dim_locations"}'),
('strategist', 'episodic', 'User asked about South region performance. Analysis showed Diapers and Baby Toys growing fastest. MamyPoko gaining market share from Pampers since Nov 2025.', '{"session":"demo","timestamp":"2026-02-20"}'),
('watchdog', 'episodic', 'Detected price anomaly: Amazon undercutting Pampers by 12%. Generated alert and price match recommendation.', '{"session":"demo","timestamp":"2026-02-22"}');

-- INITIAL ALERTS
INSERT INTO alerts (title, severity, category, message, acknowledged) VALUES
('Competitor Price Drop Detected', 'warning', 'Pricing', 'Amazon has dropped the price of Pampers Active Baby Small by 12% below our MRP. This triggers our minimum margin threshold.', FALSE),
('Critical Stockout Warning', 'critical', 'Inventory', 'East India warehouse for Huggies Wonder Pants Small is below 15 units. Expected stockout in 5 days based on current demand velocity.', FALSE),
('Regional Margin Squeeze', 'info', 'Margin', 'South region is seeing a 4% decline in margin for Baby Toys due to increased logistics costs. Recommendation: Review fulfillment routes.', FALSE);
