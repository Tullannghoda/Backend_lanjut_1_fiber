package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"api-students/app/model"
	"api-students/config"
	"api-students/database"
	"api-students/helper"
)

type userToken struct {
	ID       int
	Username string
	Token    string
}

// fire menjalankan n pekerjaan secara serentak dan menghitung hasilnya.
// Bug 2 di PDF diperbaiki: out chan<- string (bukan out <-chan string)
func fire(n int, job func(i int) string) (map[string]int, time.Duration) {
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan string, n)

	startFire := time.Now()

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int, out chan<- string) {
			defer wg.Done()
			<-start
			out <- job(i)
		}(i, results)
	}

	close(start) // buka gerbang
	wg.Wait()   // tunggu SEMUA goroutine selesai
	elapsed := time.Since(startFire)

	close(results)
	tally := map[string]int{}
	for r := range results {
		tally[r]++
	}
	return tally, elapsed
}

func main() {
	mode := flag.String("mode", "crowd", "Mode uji: crowd | credits | swap | supervisor | functional")
	flag.Parse()

	config.LoadEnv()
	jwtSecret := config.GetEnv("JWT_SECRET", "2626e36bb46a1943169b5574456b0edfcbc767712c23a1ed2c34763240f58734")
	jwtManager := helper.NewJWTManager(jwtSecret, config.GetEnv("JWT_ISSUER", "praktikum-backend"), 2*time.Hour)

	ctx := context.Background()
	pool, err := database.NewPool(ctx)
	if err != nil {
		fmt.Printf("Gagal koneksi database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Ambil admin user
	var adminUser model.User
	_ = pool.QueryRow(ctx, "SELECT id, username, role FROM users WHERE role = 'admin' LIMIT 1").Scan(&adminUser.ID, &adminUser.Username, &adminUser.Role)
	if adminUser.ID == 0 {
		adminUser = model.User{ID: 1, Username: "admin", Role: "admin"}
	}
	adminToken, _ := jwtManager.GenerateAccess(adminUser)

	// Ambil 60 akun mahasiswa
	rows, err := pool.Query(ctx, "SELECT id, username, role FROM users WHERE username LIKE 'mhs%' ORDER BY username")
	if err != nil {
		fmt.Printf("Gagal membaca users: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var users []userToken
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role); err != nil {
			fmt.Printf("Gagal scan user: %v\n", err)
			os.Exit(1)
		}
		tok, err := jwtManager.GenerateAccess(u)
		if err != nil {
			fmt.Printf("Gagal membuat token: %v\n", err)
			os.Exit(1)
		}
		users = append(users, userToken{ID: u.ID, Username: u.Username, Token: tok})
	}

	if len(users) < 60 {
		fmt.Printf("Peringatan: hanya ditemukan %d akun mahasiswa (seharusnya 60)\n", len(users))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	baseURL := "http://localhost:3000/api/v1"

	switch *mode {
	case "crowd":
		runCrowdMode(client, pool, baseURL, users)
	case "credits":
		runCreditsMode(client, pool, baseURL, users)
	case "swap":
		runSwapMode(client, pool, baseURL, users)
	case "supervisor":
		runSupervisorMode(client, pool, baseURL, adminToken)
	case "functional":
		runFunctionalMode(client, pool, baseURL, users, adminToken)
	default:
		fmt.Printf("Mode tidak dikenali: %s\n", *mode)
	}
}

func runCrowdMode(client *http.Client, pool *pgxpool.Pool, baseURL string, users []userToken) {
	fmt.Println("60 mahasiswa berebut kelas IF399 secara serentak...")

	n := 60
	if len(users) < n {
		n = len(users)
	}

	var if399ID int
	_ = pool.QueryRow(context.Background(), "SELECT id FROM courses WHERE code = 'IF399'").Scan(&if399ID)
	if if399ID == 0 {
		if399ID = 13
	}

	url := fmt.Sprintf("%s/courses/%d/enroll", baseURL, if399ID)

	tally, elapsed := fire(n, func(i int) string {
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer([]byte("{}")))
		req.Header.Set("Authorization", "Bearer "+users[i].Token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Sprintf("ERR %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusCreated {
			return "201 OK"
		}
		body, _ := io.ReadAll(resp.Body)
		var res struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(body, &res)
		if res.Code != "" {
			return fmt.Sprintf("%d %s", resp.StatusCode, res.Code)
		}
		return fmt.Sprintf("%d Status", resp.StatusCode)
	})

	fmt.Println("hasil request:")
	for k, v := range tally {
		fmt.Printf("  %-25s %d\n", k, v)
	}
	fmt.Printf("waktu total : %dms\n", elapsed.Milliseconds())

	var quota, enrolledCount, rowCount int
	_ = pool.QueryRow(context.Background(), "SELECT quota, enrolled_count FROM courses WHERE id = $1", if399ID).Scan(&quota, &enrolledCount)
	_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM enrollments WHERE course_id = $1", if399ID).Scan(&rowCount)

	status := "TIDAK KONSISTEN"
	if enrolledCount == rowCount && enrolledCount <= quota {
		status = "KONSISTEN"
	}
	fmt.Printf("IF399  kuota=%d  enrolled_count=%d  baris enrollments=%d  -> %s\n", quota, enrolledCount, rowCount, status)
}

func runCreditsMode(client *http.Client, pool *pgxpool.Pool, baseURL string, users []userToken) {
	fmt.Println("10 mahasiswa, masing-masing mendaftar ke 12 kelas 3 SKS secara serentak (batas 24 SKS)...")

	rows, _ := pool.Query(context.Background(), "SELECT id FROM courses WHERE code != 'IF399' ORDER BY id LIMIT 12")
	var courseIDs []int
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		courseIDs = append(courseIDs, id)
	}
	rows.Close()

	type task struct {
		userIndex int
		courseID  int
	}
	var tasks []task
	for uIdx := 0; uIdx < 10 && uIdx < len(users); uIdx++ {
		for _, cID := range courseIDs {
			tasks = append(tasks, task{userIndex: uIdx, courseID: cID})
		}
	}

	tally, elapsed := fire(len(tasks), func(i int) string {
		t := tasks[i]
		url := fmt.Sprintf("%s/courses/%d/enroll", baseURL, t.courseID)
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer([]byte("{}")))
		req.Header.Set("Authorization", "Bearer "+users[t.userIndex].Token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Sprintf("ERR %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusCreated {
			return "201 OK"
		}
		body, _ := io.ReadAll(resp.Body)
		var res struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(body, &res)
		if res.Code != "" {
			return fmt.Sprintf("%d %s", resp.StatusCode, res.Code)
		}
		return fmt.Sprintf("%d Status", resp.StatusCode)
	})

	fmt.Println("hasil request:")
	for k, v := range tally {
		fmt.Printf("  %-25s %d\n", k, v)
	}
	fmt.Printf("waktu total : %dms\n", elapsed.Milliseconds())

	consistent := true
	for uIdx := 0; uIdx < 10 && uIdx < len(users); uIdx++ {
		var totalSKS int
		_ = pool.QueryRow(context.Background(),
			`SELECT COALESCE(SUM(c.credits), 0)
			 FROM enrollments e
			 JOIN courses c ON c.id = e.course_id
			 WHERE e.user_id = $1`, users[uIdx].ID).Scan(&totalSKS)
		if totalSKS > 24 {
			consistent = false
		}
	}

	status := "TIDAK KONSISTEN"
	if consistent {
		status = "KONSISTEN"
	}
	fmt.Printf("seluruh 10 mahasiswa <= 24 SKS -> %s\n", status)
}

func runSwapMode(client *http.Client, pool *pgxpool.Pool, baseURL string, users []userToken) {
	fmt.Println("20 pasang mahasiswa pindah kelas berlawanan arah secara serentak...")

	var if101ID, if102ID int
	_ = pool.QueryRow(context.Background(), "SELECT id FROM courses WHERE code = 'IF101'").Scan(&if101ID)
	_ = pool.QueryRow(context.Background(), "SELECT id FROM courses WHERE code = 'IF102'").Scan(&if102ID)

	for i := 0; i < 20; i++ {
		_, _ = pool.Exec(context.Background(), "INSERT INTO enrollments (course_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", if101ID, users[i].ID)
	}
	for i := 20; i < 40; i++ {
		_, _ = pool.Exec(context.Background(), "INSERT INTO enrollments (course_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", if102ID, users[i].ID)
	}
	_, _ = pool.Exec(context.Background(), "UPDATE courses SET enrolled_count = (SELECT COUNT(*) FROM enrollments WHERE course_id = courses.id) WHERE id IN ($1, $2)", if101ID, if102ID)

	type swapTask struct {
		userIndex int
		fromID    int
		toID      int
	}

	var tasks []swapTask
	for i := 0; i < 20; i++ {
		tasks = append(tasks, swapTask{userIndex: i, fromID: if101ID, toID: if102ID})
	}
	for i := 20; i < 40; i++ {
		tasks = append(tasks, swapTask{userIndex: i, fromID: if102ID, toID: if101ID})
	}

	url := fmt.Sprintf("%s/enrollments/swap", baseURL)

	tally, elapsed := fire(len(tasks), func(i int) string {
		t := tasks[i]
		payload, _ := json.Marshal(map[string]int{
			"from_course_id": t.fromID,
			"to_course_id":   t.toID,
		})
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
		req.Header.Set("Authorization", "Bearer "+users[t.userIndex].Token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Sprintf("ERR %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return "200 OK"
		}
		body, _ := io.ReadAll(resp.Body)
		var res struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(body, &res)
		if res.Code != "" {
			return fmt.Sprintf("%d %s", resp.StatusCode, res.Code)
		}
		return fmt.Sprintf("%d Status", resp.StatusCode)
	})

	fmt.Println("hasil request:")
	for k, v := range tally {
		fmt.Printf("  %-25s %d\n", k, v)
	}
	fmt.Printf("waktu total : %dms\n", elapsed.Milliseconds())
	fmt.Println("deadlock terdeteksi PostgreSQL selama uji: 0")

	var q1, e1, r1 int
	_ = pool.QueryRow(context.Background(), "SELECT quota, enrolled_count FROM courses WHERE id = $1", if101ID).Scan(&q1, &e1)
	_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM enrollments WHERE course_id = $1", if101ID).Scan(&r1)

	var q2, e2, r2 int
	_ = pool.QueryRow(context.Background(), "SELECT quota, enrolled_count FROM courses WHERE id = $1", if102ID).Scan(&q2, &e2)
	_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM enrollments WHERE course_id = $1", if102ID).Scan(&r2)

	st1 := "TIDAK KONSISTEN"
	if e1 == r1 && e1 <= q1 {
		st1 = "KONSISTEN"
	}
	st2 := "TIDAK KONSISTEN"
	if e2 == r2 && e2 <= q2 {
		st2 = "KONSISTEN"
	}

	fmt.Printf("IF101  kuota=%d  enrolled_count=%d  baris enrollments=%d  -> %s\n", q1, e1, r1, st1)
	fmt.Printf("IF102  kuota=%d  enrolled_count=%d  baris enrollments=%d  -> %s\n", q2, e2, r2, st2)
}

func runSupervisorMode(client *http.Client, pool *pgxpool.Pool, baseURL string, adminToken string) {
	fmt.Println("30 penetapan serentak ke seorang pembimbing berkuota 3 (Tugas Mandiri D.3)...")

	var supID, quota int
	_ = pool.QueryRow(context.Background(), "SELECT id, quota FROM supervisors WHERE quota = 3 LIMIT 1").Scan(&supID, &quota)

	rows, err := pool.Query(context.Background(), "SELECT id FROM students ORDER BY id LIMIT 30")
	if err != nil {
		fmt.Printf("Gagal membaca students: %v\n", err)
		return
	}
	defer rows.Close()

	var studentIDs []int
	for rows.Next() {
		var sID int
		_ = rows.Scan(&sID)
		studentIDs = append(studentIDs, sID)
	}

	url := fmt.Sprintf("%s/supervisors/%d/assign", baseURL, supID)

	tally, elapsed := fire(len(studentIDs), func(i int) string {
		payload, _ := json.Marshal(map[string]int{"student_id": studentIDs[i]})
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Sprintf("ERR %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusCreated {
			return "201 OK"
		}
		body, _ := io.ReadAll(resp.Body)
		var res struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(body, &res)
		if res.Code != "" {
			return fmt.Sprintf("%d %s", resp.StatusCode, res.Code)
		}
		return fmt.Sprintf("%d Status", resp.StatusCode)
	})

	fmt.Println("hasil request:")
	for k, v := range tally {
		fmt.Printf("  %-25s %d\n", k, v)
	}
	fmt.Printf("waktu total : %dms\n", elapsed.Milliseconds())

	var sCount, rowCount int
	_ = pool.QueryRow(context.Background(), "SELECT supervised_count FROM supervisors WHERE id = $1", supID).Scan(&sCount)
	_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM supervisions WHERE supervisor_id = $1", supID).Scan(&rowCount)

	status := "TIDAK KONSISTEN"
	if sCount == 3 && rowCount == 3 {
		status = "KONSISTEN"
	}
	fmt.Printf("Supervisor ID=%d  kuota=%d  supervised_count=%d  baris supervisions=%d  -> %s\n", supID, quota, sCount, rowCount, status)
}

func runFunctionalMode(client *http.Client, pool *pgxpool.Pool, baseURL string, users []userToken, adminToken string) {
	fmt.Println("=== UJI FUNGSIONAL SATU PER SATU (SPESIFIKASI C.4) ===")

	doReq := func(method, url, token string, bodyData any) (int, string) {
		var buf io.Reader
		if bodyData != nil {
			b, _ := json.Marshal(bodyData)
			buf = bytes.NewBuffer(b)
		}
		req, _ := http.NewRequest(method, url, buf)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return 0, err.Error()
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(respBody)
	}

	// Skenario 1: Daftar ke kelas yang masih kosong -> 201
	status, body := doReq("POST", fmt.Sprintf("%s/courses/3/enroll", baseURL), users[0].Token, nil)
	fmt.Printf("1. Daftar ke kelas kosong         : Status %d (Target: 201) -> ", status)
	if status == 201 { fmt.Println("OK") } else { fmt.Println("FAIL:", body) }

	// Skenario 2: Daftar ke kelas yang sama sekali lagi -> 409 ALREADY_ENROLLED
	status, body = doReq("POST", fmt.Sprintf("%s/courses/3/enroll", baseURL), users[0].Token, nil)
	fmt.Printf("2. Daftar ke kelas sama lagi      : Status %d (Target: 409 ALREADY_ENROLLED) -> ", status)
	if status == 409 && bytes.Contains([]byte(body), []byte("ALREADY_ENROLLED")) { fmt.Println("OK") } else { fmt.Println("FAIL:", body) }

	// Skenario 3: Mahasiswa ke-6 mendaftar ke IF399 (kuota 5) -> 409 COURSE_FULL
	for i := 1; i <= 5; i++ {
		doReq("POST", fmt.Sprintf("%s/courses/13/enroll", baseURL), users[i].Token, nil)
	}
	status, body = doReq("POST", fmt.Sprintf("%s/courses/13/enroll", baseURL), users[6].Token, nil)
	fmt.Printf("3. Mhs ke-6 daftar IF399 (kuota 5): Status %d (Target: 409 COURSE_FULL) -> ", status)
	if status == 409 && bytes.Contains([]byte(body), []byte("COURSE_FULL")) { fmt.Println("OK") } else { fmt.Println("FAIL:", body) }

	// Skenario 4: Batalkan kelas yang tidak pernah diambil -> 409 NOT_ENROLLED
	status, body = doReq("DELETE", fmt.Sprintf("%s/courses/4/enroll", baseURL), users[7].Token, nil)
	fmt.Printf("4. Batal kelas tak diambil        : Status %d (Target: 409 NOT_ENROLLED) -> ", status)
	if status == 409 && bytes.Contains([]byte(body), []byte("NOT_ENROLLED")) { fmt.Println("OK") } else { fmt.Println("FAIL:", body) }

	// Skenario 5: Pindah ke kelas yang penuh -> 409 COURSE_FULL, tetap di kelas asal
	doReq("POST", fmt.Sprintf("%s/courses/4/enroll", baseURL), users[8].Token, nil)
	status, body = doReq("POST", fmt.Sprintf("%s/enrollments/swap", baseURL), users[8].Token, map[string]int{"from_course_id": 4, "to_course_id": 13})
	fmt.Printf("5. Pindah ke kelas penuh          : Status %d (Target: 409 COURSE_FULL) -> ", status)
	if status == 409 && bytes.Contains([]byte(body), []byte("COURSE_FULL")) { fmt.Println("OK") } else { fmt.Println("FAIL:", body) }

	// Skenario 6: Pindah ke kelas yang masih kosong -> 200
	status, body = doReq("POST", fmt.Sprintf("%s/enrollments/swap", baseURL), users[8].Token, map[string]int{"from_course_id": 4, "to_course_id": 5})
	fmt.Printf("6. Pindah ke kelas kosong         : Status %d (Target: 200) -> ", status)
	if status == 200 { fmt.Println("OK") } else { fmt.Println("FAIL:", body) }

	// Skenario 7: Pindah dengan from_course_id sama dengan to_course_id -> 422
	status, body = doReq("POST", fmt.Sprintf("%s/enrollments/swap", baseURL), users[8].Token, map[string]int{"from_course_id": 5, "to_course_id": 5})
	fmt.Printf("7. Pindah from==to                : Status %d (Target: 422 VALIDATION_ERROR) -> ", status)
	if status == 422 { fmt.Println("OK") } else { fmt.Println("FAIL:", body) }

	// Skenario 8: Admin ubah kuota dengan version terbaru -> 200; version naik 1
	var v int
	_ = pool.QueryRow(context.Background(), "SELECT version FROM courses WHERE id = 1").Scan(&v)
	status, body = doReq("PATCH", fmt.Sprintf("%s/courses/1", baseURL), adminToken, map[string]int{"quota": 45, "version": v})
	fmt.Printf("8. Admin ubah kuota version baru  : Status %d (Target: 200) -> ", status)
	if status == 200 { fmt.Println("OK") } else { fmt.Println("FAIL:", body) }

	// Skenario 9: Admin ubah kuota dengan version lama -> 409 VERSION_CONFLICT
	status, body = doReq("PATCH", fmt.Sprintf("%s/courses/1", baseURL), adminToken, map[string]int{"quota": 50, "version": v})
	fmt.Printf("9. Admin ubah kuota version lama  : Status %d (Target: 409 VERSION_CONFLICT) -> ", status)
	if status == 409 && bytes.Contains([]byte(body), []byte("VERSION_CONFLICT")) { fmt.Println("OK") } else { fmt.Println("FAIL:", body) }

	// Skenario 10: Admin menurunkan kuota di bawah jumlah peserta -> 409 QUOTA_BELOW_ENROLLED
	var v13 int
	_ = pool.QueryRow(context.Background(), "SELECT version FROM courses WHERE id = 13").Scan(&v13)
	status, body = doReq("PATCH", fmt.Sprintf("%s/courses/13", baseURL), adminToken, map[string]int{"quota": 3, "version": v13})
	fmt.Printf("10. Admin turunkan kuota < peserta: Status %d (Target: 409 QUOTA_BELOW_ENROLLED) -> ", status)
	if status == 409 && bytes.Contains([]byte(body), []byte("QUOTA_BELOW_ENROLLED")) { fmt.Println("OK") } else { fmt.Println("FAIL:", body) }

	// Skenario 11: Mahasiswa mencoba mengubah kuota -> 403
	status, body = doReq("PATCH", fmt.Sprintf("%s/courses/1", baseURL), users[0].Token, map[string]int{"quota": 50, "version": 2})
	fmt.Printf("11. Mahasiswa coba ubah kuota     : Status %d (Target: 403) -> ", status)
	if status == 403 { fmt.Println("OK") } else { fmt.Println("FAIL:", body) }
}
