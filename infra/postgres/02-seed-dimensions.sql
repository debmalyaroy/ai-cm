-- =============================================================================
-- AI-CM: Seed Data — Locations (30 Indian cities) and Sellers (12 sellers)
-- =============================================================================

INSERT INTO dim_locations (id, region, state, city, pincode) VALUES
    -- North (8 cities)
    ('a0000001-0000-0000-0000-000000000001', 'North', 'Delhi', 'New Delhi', '110001'),
    ('a0000001-0000-0000-0000-000000000002', 'North', 'Uttar Pradesh', 'Lucknow', '226001'),
    ('a0000001-0000-0000-0000-000000000003', 'North', 'Haryana', 'Gurugram', '122001'),
    ('a0000001-0000-0000-0000-000000000004', 'North', 'Punjab', 'Chandigarh', '160001'),
    ('a0000001-0000-0000-0000-000000000005', 'North', 'Uttar Pradesh', 'Noida', '201301'),
    ('a0000001-0000-0000-0000-000000000006', 'North', 'Rajasthan', 'Jaipur', '302001'),
    ('a0000001-0000-0000-0000-000000000007', 'North', 'Uttar Pradesh', 'Varanasi', '221001'),
    ('a0000001-0000-0000-0000-000000000008', 'North', 'Madhya Pradesh', 'Bhopal', '462001'),
    -- South (8 cities)
    ('a0000001-0000-0000-0000-000000000009', 'South', 'Karnataka', 'Bangalore', '560001'),
    ('a0000001-0000-0000-0000-000000000010', 'South', 'Tamil Nadu', 'Chennai', '600001'),
    ('a0000001-0000-0000-0000-000000000011', 'South', 'Kerala', 'Kochi', '682001'),
    ('a0000001-0000-0000-0000-000000000012', 'South', 'Telangana', 'Hyderabad', '500001'),
    ('a0000001-0000-0000-0000-000000000013', 'South', 'Tamil Nadu', 'Coimbatore', '641001'),
    ('a0000001-0000-0000-0000-000000000014', 'South', 'Karnataka', 'Mysuru', '570001'),
    ('a0000001-0000-0000-0000-000000000015', 'South', 'Andhra Pradesh', 'Visakhapatnam', '530001'),
    -- East (7 cities)
    ('a0000001-0000-0000-0000-000000000016', 'East', 'West Bengal', 'Kolkata', '700001'),
    ('a0000001-0000-0000-0000-000000000017', 'East', 'Odisha', 'Bhubaneswar', '751001'),
    ('a0000001-0000-0000-0000-000000000018', 'East', 'Bihar', 'Patna', '800001'),
    ('a0000001-0000-0000-0000-000000000019', 'East', 'Jharkhand', 'Ranchi', '834001'),
    ('a0000001-0000-0000-0000-000000000020', 'East', 'Assam', 'Guwahati', '781001'),
    ('a0000001-0000-0000-0000-000000000021', 'East', 'West Bengal', 'Siliguri', '734001'),
    ('a0000001-0000-0000-0000-000000000022', 'East', 'Odisha', 'Cuttack', '753001'),
    -- West (7 cities)
    ('a0000001-0000-0000-0000-000000000023', 'West', 'Maharashtra', 'Mumbai', '400001'),
    ('a0000001-0000-0000-0000-000000000024', 'West', 'Gujarat', 'Ahmedabad', '380001'),
    ('a0000001-0000-0000-0000-000000000025', 'West', 'Maharashtra', 'Pune', '411001'),
    ('a0000001-0000-0000-0000-000000000026', 'West', 'Goa', 'Panaji', '403001'),
    ('a0000001-0000-0000-0000-000000000027', 'West', 'Gujarat', 'Surat', '395001'),
    ('a0000001-0000-0000-0000-000000000028', 'West', 'Maharashtra', 'Nagpur', '440001'),
    ('a0000001-0000-0000-0000-000000000029', 'West', 'Gujarat', 'Vadodara', '390001'),
    ('a0000001-0000-0000-0000-000000000030', 'West', 'Madhya Pradesh', 'Indore', '452001');

INSERT INTO dim_sellers (id, name, email, region, rating, status) VALUES
    ('b0000001-0000-0000-0000-000000000001', 'BabyWorld Stores', 'sales@babyworld.in', 'North', 4.5, 'active'),
    ('b0000001-0000-0000-0000-000000000002', 'KiddieMart India', 'contact@kiddiemart.in', 'South', 4.2, 'active'),
    ('b0000001-0000-0000-0000-000000000003', 'TinyTots Retail', 'info@tinytots.in', 'East', 3.8, 'active'),
    ('b0000001-0000-0000-0000-000000000004', 'MomCare Essentials', 'hello@momcare.in', 'West', 4.7, 'active'),
    ('b0000001-0000-0000-0000-000000000005', 'NestBaby Online', 'support@nestbaby.in', 'South', 4.0, 'active'),
    ('b0000001-0000-0000-0000-000000000006', 'BabyBliss Mart', 'orders@babybliss.in', 'North', 4.3, 'active'),
    ('b0000001-0000-0000-0000-000000000007', 'LittleAngels Hub', 'care@littleangels.in', 'West', 3.9, 'active'),
    ('b0000001-0000-0000-0000-000000000008', 'MamaKids Express', 'hello@mamakids.in', 'East', 4.1, 'active'),
    ('b0000001-0000-0000-0000-000000000009', 'SmartParent Store', 'info@smartparent.in', 'North', 4.4, 'active'),
    ('b0000001-0000-0000-0000-000000000010', 'JoyBaby Mart', 'sales@joybaby.in', 'South', 4.6, 'active'),
    ('b0000001-0000-0000-0000-000000000011', 'KidSafe India', 'hello@kidsafe.in', 'West', 3.7, 'active'),
    ('b0000001-0000-0000-0000-000000000012', 'GrowHappy Retail', 'support@growhappy.in', 'East', 4.0, 'active');
