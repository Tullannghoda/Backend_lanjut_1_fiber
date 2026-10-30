-- Migration 007: Dosen Pembimbing Tugas Akhir (Tugas Mandiri D.2)
-- psql -U postgres -d backend_praktikum -f migrations/007_supervisors.sql

CREATE TABLE IF NOT EXISTS supervisors (
    id SERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    quota INTEGER NOT NULL CHECK (quota > 0),
    supervised_count INTEGER NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT supervisors_count_within_quota
    CHECK (supervised_count >= 0 AND supervised_count <= quota)
);

CREATE TABLE IF NOT EXISTS supervisions (
    id BIGSERIAL PRIMARY KEY,
    student_id INTEGER NOT NULL UNIQUE REFERENCES students(id) ON DELETE CASCADE,
    supervisor_id INTEGER NOT NULL REFERENCES supervisors(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS supervisions_supervisor_idx ON supervisions (supervisor_id);

INSERT INTO supervisors (name, quota) VALUES
    ('Prof. Dr. Ir. Budi Raharjo, M.Sc.', 10),
    ('Dr. Eng. Siti Aminah, S.Kom., M.Kom.', 10),
    ('Dosen Kuota Rebutan, M.T.', 3)
ON CONFLICT DO NOTHING;

INSERT INTO permissions (name, description) VALUES
    ('supervision:manage', 'Mengelola bimbingan tugas akhir')
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_name, permission_name) VALUES
    ('admin', 'supervision:manage')
ON CONFLICT DO NOTHING;
