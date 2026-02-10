-- Seed default Podcasts section if missing.
INSERT INTO sections (name, type)
SELECT 'Podcasts', 'podcast'
WHERE NOT EXISTS (
  SELECT 1
  FROM sections
  WHERE type = 'podcast'
);
