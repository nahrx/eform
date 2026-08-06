-- Add the 'kota' value to the wilayah_level enum.
-- A 'kota' (city) sits at the same level as a 'kabupaten' (regency) in Indonesia's administrative hierarchy.
ALTER TYPE wilayah_level ADD VALUE IF NOT EXISTS 'kota';
