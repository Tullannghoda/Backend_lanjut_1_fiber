import os

base_dir = r'D:\TI\SEMESTER 5\Backend Lanjut PRAK\latihan-fiber\api-students'

# 1. Migrations
sql_code = '''CREATE INDEX IF NOT EXISTS students_created_at_id_desc_idx ON students (created_at DESC, id DESC);'''
with open(os.path.join(base_dir, 'migrations', '006_student_cursor_index.sql'), 'w') as f:
    f.write(sql_code)

# 2. app/model/student.go
model_path = os.path.join(base_dir, 'app', 'model', 'student.go')
with open(model_path, 'r', encoding='utf-8') as f:
    mod_code = f.read()

import re

mod_code = re.sub(
    r'type CreateStudentRequest struct \{.*?(?=\n\})', 
    r'type CreateStudentRequest struct {\n\tNIM      string `json:"nim" validate:"required,nim"`\n\tName     string `json:"name" validate:"required,min=3,max=50"`\n\tGrade    string `json:"grade" validate:"required,min=1,max=2"`\n\tIsActive bool   `json:"is_active"`', 
    mod_code, flags=re.DOTALL
)

mod_code = re.sub(
    r'type ReplaceStudentRequest struct \{.*?(?=\n\})', 
    r'type ReplaceStudentRequest struct {\n\tNIM      string `json:"nim" validate:"required,nim"`\n\tName     string `json:"name" validate:"required,min=3,max=50"`\n\tGrade    string `json:"grade" validate:"required,min=1,max=2"`\n\tIsActive bool   `json:"is_active"`', 
    mod_code, flags=re.DOTALL
)

mod_code = re.sub(
    r'type PatchStudentRequest struct \{.*?(?=\n\})', 
    r'type PatchStudentRequest struct {\n\tNIM      *string `json:"nim,omitempty" validate:"omitnil,nim"`\n\tName     *string `json:"name,omitempty" validate:"omitnil,min=3,max=50"`\n\tGrade    *string `json:"grade,omitempty" validate:"omitnil,min=1,max=2"`\n\tIsActive *bool   `json:"is_active,omitempty"`', 
    mod_code, flags=re.DOTALL
)

with open(model_path, 'w', encoding='utf-8') as f:
    f.write(mod_code)

# 3. helper/validator.go
val_path = os.path.join(base_dir, 'helper', 'validator.go')
with open(val_path, 'r', encoding='utf-8') as f:
    val_code = f.read()

nim_validation = '''	_ = v.RegisterValidation("nim", func(fl validator.FieldLevel) bool {
		nim := fl.Field().String()
		if len(nim) < 8 || len(nim) > 15 {
			return false
		}
		for _, r := range nim {
			if !unicode.IsDigit(r) {
				return false
			}
		}
		return true
	})
'''
val_code = val_code.replace('return v\n}', nim_validation + '\n\treturn v\n}')
val_code = val_code.replace('case "email":', 'case "nim":\n\t\treturn "format NIM tidak valid (harus angka 8-15 digit)"\n\tcase "email":')

with open(val_path, 'w', encoding='utf-8') as f:
    f.write(val_code)

# 4. helper/negotiate.go
neg_path = os.path.join(base_dir, 'helper', 'negotiate.go')
with open(neg_path, 'r', encoding='utf-8') as f:
    neg_code = f.read()

csv_students = '''
func WriteStudentsCSV(c *fiber.Ctx, students []model.Student) error {
	c.Set(fiber.HeaderContentType, FormatCSV+"; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="students.csv"`)

	var buffer strings.Builder
	writer := csv.NewWriter(&buffer)

	header := []string{"id", "nim", "name", "grade", "is_active", "owner_id", "created_at"}
	if err := writer.Write(header); err != nil {
		return Internal(err)
	}

	for _, s := range students {
		row := []string{
			strconv.Itoa(s.ID), s.NIM, s.Name, s.Grade,
			strconv.FormatBool(s.IsActive), strconv.Itoa(s.OwnerID),
			s.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
		if err := writer.Write(row); err != nil {
			return Internal(err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return Internal(err)
	}

	return c.SendString(buffer.String())
}
'''
if 'WriteStudentsCSV' not in neg_code:
    neg_code += csv_students
with open(neg_path, 'w', encoding='utf-8') as f:
    f.write(neg_code)

# 5. app/repository/student_repository.go
repo_path = os.path.join(base_dir, 'app', 'repository', 'student_repository.go')
with open(repo_path, 'r', encoding='utf-8') as f:
    repo_code = f.read()

interface_replace = '''	FindAfterCursor(ctx context.Context, q model.CursorQuery) ([]model.Student, error)
}'''
repo_code = repo_code.replace('Delete(ctx context.Context, id int) error\n}', interface_replace)

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
if 'FindAfterCursor' not in repo_code:
    repo_code += cursor_query_code
with open(repo_path, 'w', encoding='utf-8') as f:
    f.write(repo_code)

print("Update tahap 1 D.2-D.4 sukses.")
