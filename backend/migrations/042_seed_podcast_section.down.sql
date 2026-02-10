-- Remove default Podcasts section if it is not in use.
DELETE FROM sections s
WHERE s.type = 'podcast'
  AND s.name = 'Podcasts'
  AND NOT EXISTS (
    SELECT 1
    FROM posts p
    WHERE p.section_id = s.id
  );
