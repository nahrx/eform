-- Tambah nilai 'kota' ke enum wilayah_level.
-- Kota setara dengan kabupaten dalam hierarki administrasi Indonesia.
ALTER TYPE wilayah_level ADD VALUE IF NOT EXISTS 'kota';
