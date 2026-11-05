-- Migration 009: Idempotency key per-user untuk notifikasi massal (course.quota_changed)
--   psql -U postgres -d backend_praktikum -f migrations/009_quota_notifications.sql

-- 1. Mengganti UNIQUE constraint dari (outbox_id) menjadi (outbox_id, user_id)
--    agar satu event massal (seperti perubahan kuota kelas) dapat mencatat
--    banyak notifikasi (satu per peserta) tanpa melanggar idempotency key.
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_outbox_unique;
ALTER TABLE notifications ADD CONSTRAINT notifications_outbox_user_unique UNIQUE (outbox_id, user_id);
