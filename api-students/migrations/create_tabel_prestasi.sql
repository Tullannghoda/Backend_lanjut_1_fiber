CREATE TABLE IF NOT EXISTS achievements (
    id SERIAL PRIMARY KEY,
    student_id INT NOT NULL,
    name VARCHAR(150) NOT NULL,
    rank INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT fk_student FOREIGN KEY(student_id) REFERENCES students(id) ON DELETE CASCADE
);