-- 0002_wilayah.down.sql
DROP TRIGGER  IF EXISTS trg_wilayah_updated_at ON wilayah;
DROP FUNCTION IF EXISTS fn_set_updated_at;
DROP TABLE    IF EXISTS wilayah;
DROP TYPE     IF EXISTS wilayah_level;
