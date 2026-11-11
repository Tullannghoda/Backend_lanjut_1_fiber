//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/app/service"
	"api-students/config"
	"api-students/helper"
	"api-students/route"
)

// api adalah aplikasi lengkap yang memakai PostgreSQL sungguhan. Bedanya
// dengan production hanya satu: database-nya praktikum_backend_test.
type api struct {
	app *fiber.App
	jwt *helper.JWTManager
}

func newAPI(t *testing.T) api {
	t.Helper()

	raw, err := repository.NewRoleRepository(pool).LoadPermissions(context.Background())
	if err != nil {
		t.Fatalf("memuat permission: %v", err)
	}
	perms := helper.NewPermissionSet(raw)
	jwtManager := helper.NewJWTManager("secret-khusus-test-minimal-32-karakter", "test", time.Minute)

	courseService := service.NewCourseService(
		repository.NewCourseRepository(pool),
		repository.NewOutboxRepository(pool),
		repository.NewTxManager(pool),
	)

	app := config.NewApp(slog.New(slog.NewTextHandler(io.Discard, nil)), route.Dependencies{
		Pool:          pool,
		JWT:           jwtManager,
		Permissions:   perms,
		CourseService: courseService,
	})
	return api{app: app, jwt: jwtManager}
}

// enroll mengirim POST /courses/:id/enroll atas nama student dan
// mengembalikan status code beserta field "code" pada response.
func (a api) enroll(t *testing.T, student model.User, courseID int) (int, string) {
	t.Helper()

	token, err := a.jwt.GenerateAccess(model.User{ID: student.ID, Username: student.Username, Role: "user"})
	if err != nil {
		t.Errorf("membuat token: %v", err)
		return 0, ""
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/courses/"+strconv.Itoa(courseID)+"/enroll", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.app.Test(req, -1)
	if err != nil {
		// t.Errorf, bukan t.Fatalf: fungsi ini juga dipanggil dari
		// goroutine, dan t.Fatalf hanya boleh dipanggil dari goroutine test.
		t.Errorf("request enroll: %v", err)
		return 0, ""
	}
	defer resp.Body.Close()

	var body struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body.Code
}

func countRows(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("query %q: %v", sql, err)
	}
	return n
}

func TestEnrollEndpoint_WritesOutboxOnlyOnSuccess(t *testing.T) {
	a := newAPI(t)
	course := seedCourse(t, 3, 40)
	student := seedStudent(t)

	if status, _ := a.enroll(t, student, course.ID); status != http.StatusCreated {
		t.Fatalf("pendaftaran pertama: status = %d, seharusnya 201", status)
	}
	if status, code := a.enroll(t, student, course.ID); status != http.StatusConflict || code != "ALREADY_ENROLLED" {
		t.Fatalf("pendaftaran kedua: dapat %d %s, seharusnya 409 ALREADY_ENROLLED", status, code)
	}

	events := countRows(t,
		`SELECT COUNT(*) FROM outbox WHERE (payload->>'user_id')::INT = $1 AND event_type = $2`,
		student.ID, model.EventEnrollmentCreated)
	if events != 1 {
		t.Errorf("event enrollment.created = %d, seharusnya 1 (pendaftaran yang gagal tidak boleh meninggalkan jejak)", events)
	}
}

// Satu mahasiswa mengirim sepuluh pendaftaran SERENTAK ke sepuluh kelas
// 3 SKS. Batasnya 24 SKS, maka tepat delapan yang boleh berhasil.
// Test ini adalah versi otomatis dari uji beban mode credits pada Modul 9.
func TestEnrollEndpoint_ConcurrentCreditLimit(t *testing.T) {
	a := newAPI(t)
	student := seedStudent(t)

	const attempts = 10
	courses := make([]model.Course, attempts)
	for i := range courses {
		courses[i] = seedCourse(t, 3, 40)
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results = map[int]int{}
		start   = make(chan struct{})
	)
	for _, c := range courses {
		wg.Add(1)
		go func(courseID int) {
			defer wg.Done()
			<-start // tunggu aba-aba agar semua request berangkat bersamaan
			status, _ := a.enroll(t, student, courseID)
			mu.Lock()
			results[status]++
			mu.Unlock()
		}(c.ID)
	}
	close(start) // aba-aba: semua goroutine dilepas pada saat yang sama
	wg.Wait()

	maxCourses := service.MaxCreditsPerSemester / 3
	if results[http.StatusCreated] != maxCourses {
		t.Errorf("berhasil = %d, seharusnya %d (hasil lengkap: %v)", results[http.StatusCreated], maxCourses, results)
	}
	enrolled := countRows(t, `SELECT COUNT(*) FROM enrollments WHERE user_id = $1`, student.ID)
	if enrolled != maxCourses {
		t.Errorf("baris enrollments = %d, seharusnya %d", enrolled, maxCourses)
	}
}

func (a api) doJSON(t *testing.T, student model.User, method, path, reqBody string) (int, string) {
	t.Helper()

	token, err := a.jwt.GenerateAccess(model.User{ID: student.ID, Username: student.Username, Role: "user"})
	if err != nil {
		t.Fatalf("membuat token: %v", err)
	}

	req := httptest.NewRequest(method, path, strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	var body struct {
		Code string `json:"code"`
	}
	if resp.StatusCode != http.StatusNoContent {
		_ = json.NewDecoder(resp.Body).Decode(&body)
	}
	return resp.StatusCode, body.Code
}

// Tugas Mandiri D.2 (butir 1): swap ke kelas tujuan yang sudah penuh.
func TestSwapEndpoint_TargetFull(t *testing.T) {
	a := newAPI(t)
	fromCourse := seedCourse(t, 3, 5)
	toCourse := seedCourse(t, 3, 1)
	student := seedStudent(t)
	other := seedStudent(t)

	if status, _ := a.enroll(t, other, toCourse.ID); status != http.StatusCreated {
		t.Fatalf("isi kelas tujuan: status = %d, seharusnya 201", status)
	}
	if status, _ := a.enroll(t, student, fromCourse.ID); status != http.StatusCreated {
		t.Fatalf("daftar kelas asal: status = %d, seharusnya 201", status)
	}

	payload := fmt.Sprintf(`{"from_course_id": %d, "to_course_id": %d}`, fromCourse.ID, toCourse.ID)
	status, code := a.doJSON(t, student, http.MethodPost, "/api/v1/enrollments/swap", payload)
	if status != http.StatusConflict || code != "COURSE_FULL" {
		t.Fatalf("swap ke kelas penuh: dapat %d %s, seharusnya 409 COURSE_FULL", status, code)
	}

	if n := countRows(t, `SELECT COUNT(*) FROM enrollments WHERE user_id = $1 AND course_id = $2`,
		student.ID, fromCourse.ID); n != 1 {
		t.Errorf("mahasiswa keluar dari kelas asal: baris enrollments = %d, seharusnya 1", n)
	}
	if n := countRows(t, `SELECT COUNT(*) FROM enrollments WHERE user_id = $1 AND course_id = $2`,
		student.ID, toCourse.ID); n != 0 {
		t.Errorf("mahasiswa masuk ke kelas penuh: baris enrollments = %d, seharusnya 0", n)
	}

	if c := countRows(t, `SELECT enrolled_count FROM courses WHERE id = $1`, fromCourse.ID); c != 1 {
		t.Errorf("enrolled_count kelas asal = %d, seharusnya tetap 1", c)
	}
	if c := countRows(t, `SELECT enrolled_count FROM courses WHERE id = $1`, toCourse.ID); c != 1 {
		t.Errorf("enrolled_count kelas tujuan = %d, seharusnya tetap 1", c)
	}

	swappedEvents := countRows(t,
		`SELECT COUNT(*) FROM outbox WHERE (payload->>'user_id')::INT = $1 AND event_type = $2`,
		student.ID, model.EventEnrollmentSwapped)
	if swappedEvents != 0 {
		t.Errorf("event enrollment.swapped = %d, seharusnya 0", swappedEvents)
	}
}

// Tugas Mandiri D.2 (butir 2): drop kelas yang sukses.
func TestDropEndpoint_Success(t *testing.T) {
	a := newAPI(t)
	course := seedCourse(t, 3, 40)
	student := seedStudent(t)

	if status, _ := a.enroll(t, student, course.ID); status != http.StatusCreated {
		t.Fatalf("daftar awal: status = %d, seharusnya 201", status)
	}

	status, _ := a.doJSON(t, student, http.MethodDelete, "/api/v1/courses/"+strconv.Itoa(course.ID)+"/enroll", "")
	if status != http.StatusNoContent {
		t.Fatalf("drop kelas: status = %d, seharusnya 204", status)
	}

	if n := countRows(t, `SELECT COUNT(*) FROM enrollments WHERE user_id = $1 AND course_id = $2`,
		student.ID, course.ID); n != 0 {
		t.Errorf("baris enrollments = %d, seharusnya 0 setelah drop", n)
	}
	if c := countRows(t, `SELECT enrolled_count FROM courses WHERE id = $1`, course.ID); c != 0 {
		t.Errorf("enrolled_count = %d, seharusnya 0 setelah drop", c)
	}

	droppedEvents := countRows(t,
		`SELECT COUNT(*) FROM outbox WHERE (payload->>'user_id')::INT = $1 AND event_type = $2`,
		student.ID, model.EventEnrollmentDropped)
	if droppedEvents != 1 {
		t.Errorf("event enrollment.dropped = %d, seharusnya 1", droppedEvents)
	}
}
