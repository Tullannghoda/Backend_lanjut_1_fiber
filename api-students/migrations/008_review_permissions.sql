-- Migration 008: permission moderasi ulasan
--   psql -U postgres -d backend_praktikum -f migrations/008_review_permissions.sql

INSERT INTO permissions (name, description) VALUES
    ('review:moderate', 'Menghapus ulasan milik siapa pun')
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_name, permission_name) VALUES
    ('admin', 'review:moderate')
ON CONFLICT DO NOTHING;
