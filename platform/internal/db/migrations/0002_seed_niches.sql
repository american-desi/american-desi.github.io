-- Migration 0002: Pre-load the 10 starter niches with rough scores.
-- Numbers are conservative estimates to bootstrap the analyzer.

INSERT OR IGNORE INTO niche_analyses
    (niche, monthly_search_vol, competition_level, monetization_paths, avg_affiliate_comm, est_rpm, content_velocity, time_to_revenue, score, rationale)
VALUES
    ('personal-finance-tools',      450000, 'high',   '["affiliate","ads","lead_gen"]', 35.0, 22.0, 80, '6-9 months',  8.9, 'High affiliate payouts (credit cards $50-150 CPA, brokerages $100+ CPA). Requires YMYL-grade authority content.'),
    ('home-office-gear',            320000, 'medium', '["affiliate","ads"]',            6.0,  18.0, 60, '4-6 months',  8.1, 'Amazon + brand affiliate. Reviews and comparison content convert well. Lower EEAT bar than finance.'),
    ('online-learning-platforms',   280000, 'medium', '["affiliate","ads"]',            40.0, 14.0, 70, '5-7 months',  8.3, 'Coursera/Udemy/MasterClass affiliate pay $10-40 per signup. Evergreen comparison content.'),
    ('pet-care-products',           520000, 'medium', '["affiliate","ads"]',            8.0,  17.0, 70, '4-6 months',  7.9, 'Recurring purchase intent (food, insurance). Amazon + Chewy + Petco affiliate stack.'),
    ('smart-home-security',         310000, 'medium', '["affiliate","ads"]',            10.0, 20.0, 65, '5-7 months',  8.0, 'High AOV products ($200-1000). Ring/SimpliSafe/Arlo affiliate programs with 4-8% rates.'),
    ('vpn-privacy-tools',           410000, 'high',   '["affiliate","ads"]',            50.0, 16.0, 50, '4-6 months',  8.6, 'One of the highest-paying affiliate verticals — $30-120 per signup. Crowded but evergreen.'),
    ('web-hosting',                 370000, 'high',   '["affiliate","ads"]',            65.0, 15.0, 60, '6-9 months',  8.5, 'Extremely high commissions ($65-200 per sign-up). Saturated but recession-resistant.'),
    ('home-fitness-equipment',      290000, 'medium', '["affiliate","ads"]',            8.0,  16.0, 65, '5-7 months',  7.8, 'High AOV (treadmills, bikes). Supplement cross-sells available via Amazon/iHerb.'),
    ('baby-gear-parenting',         400000, 'medium', '["affiliate","ads"]',            6.0,  19.0, 75, '5-7 months',  7.9, 'Amazon-dominant. Strong seasonal spikes (baby registries). Trust/EEAT matters.'),
    ('rv-vanlife-gear',             180000, 'low',    '["affiliate","ads"]',            7.0,  14.0, 55, '3-5 months',  7.4, 'Lower volume but low competition. Fast to rank. Niche loyal audience, strong email potential.');
