-- Rename Photos section to Series and update type
UPDATE sections
SET name = 'Series',
    type = 'series'
WHERE type = 'photo'
   OR name = 'Photos';
