DELETE FROM enrollments
WHERE user_id IN (SELECT id FROM users WHERE username LIKE 'mhs%');

UPDATE courses c
SET enrolled_count = (SELECT COUNT(*) FROM enrollments e WHERE e.course_id = c.id);

DELETE FROM supervisions;

UPDATE supervisors s
SET supervised_count = (SELECT COUNT(*) FROM supervisions sv WHERE sv.supervisor_id = s.id);
