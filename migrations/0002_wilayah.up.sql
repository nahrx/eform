-- 0002_wilayah.up.sql — region reference table (province → regency → district → village)
-- Uses kode_wilayah (the official BPS region code) as the natural primary key.

DO $$ BEGIN
    CREATE TYPE wilayah_level AS ENUM ('provinsi', 'kabupaten', 'kecamatan', 'desa');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS wilayah (
    kode_wilayah  VARCHAR(20)   PRIMARY KEY,
    nama_wilayah  TEXT          NOT NULL,
    level         wilayah_level NOT NULL,
    kode_parent   VARCHAR(20)   REFERENCES wilayah(kode_wilayah) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_wilayah_parent ON wilayah(kode_parent);
CREATE INDEX IF NOT EXISTS idx_wilayah_level  ON wilayah(level);

-- Stand-in trigger for ON UPDATE CURRENT_TIMESTAMP, which PostgreSQL lacks natively.
CREATE OR REPLACE FUNCTION fn_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_wilayah_updated_at
    BEFORE UPDATE ON wilayah
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();
