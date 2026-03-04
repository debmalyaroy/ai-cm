-- =============================================================================
-- AI-CM: Sales Data Generator (~50K+ rows)
-- 200 products × 30 locations × ~10 sampled weeks ≈ 50K+ rows
-- Uses weighted random sampling for realistic patterns
-- =============================================================================

DO $$
DECLARE
    p_rec RECORD;
    l_rec RECORD;
    sale_week DATE;
    base_qty INTEGER;
    qty INTEGER;
    price DECIMAL;
    cost DECIMAL;
    disc DECIMAL;
    rev DECIMAL;
    mgn DECIMAL;
    cat TEXT;
    sub_cat TEXT;
    loc_region TEXT;
    seasonal_factor DECIMAL;
    month_num INTEGER;
    row_count INTEGER := 0;
BEGIN
    FOR sale_week IN
        SELECT generate_series(
            DATE '2024-08-05',
            DATE '2026-02-17',
            INTERVAL '1 week'
        )::DATE
    LOOP
        month_num := EXTRACT(MONTH FROM sale_week)::INT;

        seasonal_factor := CASE
            WHEN month_num IN (10, 11, 12) THEN 1.3
            WHEN month_num IN (1, 2)       THEN 0.85
            WHEN month_num IN (6, 7, 8)    THEN 1.1
            ELSE 1.0
        END;

        FOR p_rec IN SELECT id, category, sub_category, mrp, cost_price FROM dim_products LOOP
            cat := p_rec.category;
            sub_cat := p_rec.sub_category;
            price := p_rec.mrp;
            cost := p_rec.cost_price;

            FOR l_rec IN SELECT id, region FROM dim_locations LOOP
                loc_region := l_rec.region;

                -- Skip ~66% of product-location combos for sparse realistic data
                IF random() > 0.34 THEN CONTINUE; END IF;

                -- Base weekly quantity by category type
                base_qty := CASE
                    WHEN cat = 'Diapers'          THEN 8 + (random() * 15)::INT
                    WHEN cat = 'Baby Wipes'       THEN 5 + (random() * 12)::INT
                    WHEN cat = 'Baby Skincare'    THEN 4 + (random() * 10)::INT
                    WHEN cat = 'Baby Cereals'     THEN 6 + (random() * 14)::INT
                    WHEN cat = 'Baby Formula'     THEN 4 + (random() * 10)::INT
                    WHEN cat = 'Baby Snacks'      THEN 5 + (random() * 8)::INT
                    WHEN cat = 'Feeding Bottles'  THEN 3 + (random() * 7)::INT
                    WHEN cat = 'Baby Grooming'    THEN 4 + (random() * 8)::INT
                    WHEN cat = 'Baby Bath'        THEN 3 + (random() * 7)::INT
                    WHEN cat = 'Baby Clothing'    THEN 3 + (random() * 8)::INT
                    WHEN cat = 'Baby Bedding'     THEN 1 + (random() * 4)::INT
                    WHEN cat = 'Baby Toys'        THEN 3 + (random() * 8)::INT
                    WHEN cat = 'Baby Safety'      THEN 1 + (random() * 4)::INT
                    WHEN cat = 'Baby Carriers'    THEN 0 + (random() * 3)::INT
                    WHEN cat = 'Baby Furniture'   THEN 0 + (random() * 3)::INT
                    WHEN cat = 'Strollers'        THEN 0 + (random() * 2)::INT
                    WHEN cat = 'Car Seats'        THEN 0 + (random() * 2)::INT
                    WHEN cat = 'Breast Pumps'     THEN 1 + (random() * 4)::INT
                    WHEN cat = 'Maternity Care'   THEN 2 + (random() * 5)::INT
                    WHEN cat = 'Baby Gifting'     THEN 2 + (random() * 6)::INT
                    ELSE 3 + (random() * 7)::INT
                END;

                base_qty := GREATEST((base_qty * seasonal_factor)::INT, 0);

                -- Regional modifiers
                IF loc_region = 'South' AND cat = 'Diapers' THEN
                    base_qty := (base_qty * 1.5)::INT;
                END IF;
                IF loc_region = 'East' AND cat = 'Baby Furniture' THEN
                    base_qty := (base_qty * 0.4)::INT;
                END IF;
                IF loc_region = 'North' AND cat IN ('Baby Cereals','Baby Formula') THEN
                    base_qty := (base_qty * 1.3)::INT;
                END IF;
                IF loc_region = 'West' AND cat = 'Baby Gifting' THEN
                    base_qty := (base_qty * 1.4)::INT;
                END IF;

                -- Story: East region decline
                IF loc_region = 'East' AND sale_week >= DATE '2025-12-01' THEN
                    base_qty := (base_qty * 0.7)::INT;
                END IF;

                -- Story: MamyPoko gaining in South
                IF cat = 'Diapers' AND loc_region = 'South' AND sale_week >= DATE '2025-11-01' THEN
                    IF p_rec.id IN ('c0000001-0000-0000-0000-000000000001','c0000001-0000-0000-0000-000000000002','c0000001-0000-0000-0000-000000000003') THEN
                        base_qty := (base_qty * 0.75)::INT;
                    ELSIF p_rec.id IN ('c0000001-0000-0000-0000-000000000007','c0000001-0000-0000-0000-000000000008','c0000001-0000-0000-0000-000000000009') THEN
                        base_qty := (base_qty * 1.35)::INT;
                    END IF;
                END IF;

                IF base_qty <= 0 THEN CONTINUE; END IF;

                qty := base_qty;
                disc := (random() * 18)::DECIMAL(5,2);
                rev := qty * price * (1 - disc / 100);
                mgn := rev - (qty * cost);

                INSERT INTO fact_sales (product_id, location_id, sale_date, quantity, selling_price, discount_pct, revenue, margin)
                VALUES (p_rec.id, l_rec.id, sale_week, qty, price * (1 - disc / 100), disc, rev, mgn);

                row_count := row_count + 1;
            END LOOP;
        END LOOP;
    END LOOP;

    RAISE NOTICE 'Inserted % sales rows', row_count;
END $$;
