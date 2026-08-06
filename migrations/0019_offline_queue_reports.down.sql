-- 0019_offline_queue_reports.down.sql
DROP INDEX IF EXISTS idx_queue_reports_stale;
DROP INDEX IF EXISTS idx_queue_reports_form;
DROP TABLE IF EXISTS offline_queue_reports;
