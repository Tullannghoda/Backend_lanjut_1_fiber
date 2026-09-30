import os

repo_path = r'D:\TI\SEMESTER 5\Backend Lanjut PRAK\latihan-fiber\api-students\app\repository\student_repository.go'
with open(repo_path, 'r', encoding='utf-8') as f:
    repo_code = f.read()

cursor_query_code = '''
func (r *studentPostgresRepository) FindAfterCursor(ctx context.Context, q model.CursorQuery) ([]model.Student, error) {
	args := []any{}
	where := " WHERE 1 = 1"

	if q.Search != "" {
		args = append(args, "%"+q.Search+"%")
		where += fmt.Sprintf(" AND name ILIKE $%d", len(args))
	}
	if q.IsActive != nil {
		args = append(args, *q.IsActive)
		where += fmt.Sprintf(" AND is_active = $%d", len(args))
	}
	if q.After != nil {
		args = append(args, q.After.CreatedAt, q.After.ID)
		where += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", len(args)-1, len(args))
	}

	args = append(args, q.Limit+1)
	query := fmt.Sprintf(
		`SELECT id, nim, name, grade, is_active, COALESCE(owner_id, 0), created_at
		 FROM students%s ORDER BY created_at DESC, id DESC LIMIT $%d`,
		where, len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mengambil daftar student cursor: %w", err)
	}
	defer rows.Close()

	var hasil []model.Student
	for rows.Next() {
		var s model.Student
		if err := rows.Scan(&s.ID, &s.NIM, &s.Name, &s.Grade, &s.IsActive, &s.OwnerID, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("membaca baris student cursor: %w", err)
		}
		hasil = append(hasil, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("membaca hasil query cursor: %w", err)
	}
	return hasil, nil
}
'''
if 'func (r *studentPostgresRepository) FindAfterCursor' not in repo_code:
    repo_code += cursor_query_code
with open(repo_path, 'w', encoding='utf-8') as f:
    f.write(repo_code)
