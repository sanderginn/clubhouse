-- Revert Series section back to Photos and photo type
UPDATE sections
SET name = 'Photos',
    type = 'photo'
WHERE type = 'series'
   OR name = 'Series';
