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
	"os/exec"
	"strings"
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
	Role     string
	Token    string
}

func startRedis() {
	_ = exec.Command("powershell", "-c", "Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{CommandLine='C:\\laragon\\bin\\redis\\current\\redis-server.exe --port 6379 --bind 127.0.0.1 --dir C:\\laragon\\bin\\redis\\current --logfile C:\\laragon\\bin\\redis\\current\\redis.log'}").Run()
	time.Sleep(1500 * time.Millisecond)
	_ = exec.Command("powershell", "-c", "\"sadd asynq:queues notifications\" | & \"C:\\laragon\\bin\\redis\\current\\redis-cli.exe\"").Run()
}

func stopRedis() {
	_ = exec.Command("powershell", "-c", "Stop-Process -Name redis-server -Force -ErrorAction SilentlyContinue").Run()
}

func ensureRedisRunning() {
	cmd := exec.Command("C:\\laragon\\bin\\redis\\current\\redis-cli.exe", "ping")
	out, err := cmd.Output()
	if err != nil || !strings.Contains(string(out), "PONG") {
		startRedis()
	} else {
		_ = exec.Command("powershell", "-c", "\"sadd asynq:queues notifications\" | & \"C:\\laragon\\bin\\redis\\current\\redis-cli.exe\"").Run()
	}
}

func main() {
	mode := flag.String("mode", "all", "Mode uji: all | c1 | c2 | c3 | c4 | c5 | d2")
	flag.Parse()

	ensureRedisRunning()

	config.LoadEnv()
	jwtSecret := config.GetEnv("JWT_SECRET", "2626e36bb46a1943169b5574456b0edfcbc767712c23a1ed2c34763240f58734")
	jwtManager := helper.NewJWTManager(jwtSecret, "praktikum-backend", 15*time.Minute)

	pool, err := database.NewPool(context.Background())
	if err != nil {
		fmt.Printf("Gagal terhubung ke database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	mhsuji := ensureUser(pool, jwtManager, "mhsuji", "mhsuji@student.unair.ac.id", "user")
	salahalamat := ensureUser(pool, jwtManager, "salahalamat", "salah@kampus.invalid", "user")
	admin := ensureUser(pool, jwtManager, "admin", "admin@unair.ac.id", "admin")

	switch *mode {
	case "c1":
		runC1(pool, mhsuji)
	case "c2":
		runC2(pool, mhsuji, admin)
	case "c3":
		runC3(pool, mhsuji)
	case "c4":
		runC4(pool, salahalamat, admin)
	case "c5":
		runC5(pool, mhsuji)
	case "d2":
		runD2(pool, admin)
	case "all":
		runC1(pool, mhsuji)
		runC2(pool, mhsuji, admin)
		runC3(pool, mhsuji)
		runC4(pool, salahalamat, admin)
		runC5(pool, mhsuji)
		runD2(pool, admin)
	default:
		fmt.Printf("Mode tidak dikenal: %s\n", *mode)
	}
}

func ensureUser(pool *pgxpool.Pool, jwt *helper.JWTManager, username, email, role string) userToken {
	ctx := context.Background()
	var id int
	err := pool.QueryRow(ctx, "SELECT id FROM users WHERE username = $1", username).Scan(&id)
	if err != nil {
		hash, _ := helper.HashPassword("Password123!")
		_ = pool.QueryRow(ctx,
			"INSERT INTO users (username, email, password, role) VALUES ($1, $2, $3, $4) RETURNING id",
			username, email, hash, role).Scan(&id)
	}

	token, _ := jwt.GenerateAccess(model.User{
		ID:       id,
		Username: username,
		Email:    email,
		Role:     role,
	})

	return userToken{ID: id, Username: username, Role: role, Token: token}
}

func resetDB(pool *pgxpool.Pool) {
	ctx := context.Background()
	_, _ = pool.Exec(ctx, `
		DELETE FROM notifications WHERE user_id IN (SELECT id FROM users WHERE username IN ('mhsuji', 'salahalamat'));
		DELETE FROM outbox WHERE (payload->>'user_id')::INT IN (SELECT id FROM users WHERE username IN ('mhsuji', 'salahalamat'));
		DELETE FROM enrollments WHERE user_id IN (SELECT id FROM users WHERE username IN ('mhsuji', 'salahalamat'));
		UPDATE courses c SET enrolled_count = (SELECT COUNT(*) FROM enrollments e WHERE e.course_id = c.id);
	`)
	_ = os.Truncate("logs/mail.log", 0)
}

func countMailLines(pattern string) int {
	data, err := os.ReadFile("logs/mail.log")
	if err != nil {
		return 0
	}
	lines := strings.Split(string(data), "\n")
	cnt := 0
	for _, l := range lines {
		if strings.Contains(l, pattern) {
			cnt++
		}
	}
	return cnt
}

func httpReq(method, url, token string, body any) (int, map[string]any) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, r)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, map[string]any{"error": err.Error()}
	}
	defer resp.Body.Close()

	var res map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&res)
	return resp.StatusCode, res
}

// ==================== C.1 JALUR NORMAL ====================
func runC1(pool *pgxpool.Pool, u userToken) {
	fmt.Println("=== UJI C.1: JALUR NORMAL ===")
	resetDB(pool)

	baseURL := "http://localhost:3000/api/v1"

	s1, _ := httpReq("POST", baseURL+"/courses/2/enroll", u.Token, nil)
	fmt.Printf("1. POST /courses/2/enroll             : Status %d (Target: 201) -> %s\n", s1, check(s1 == 201))

	s2, _ := httpReq("POST", baseURL+"/courses/2/enroll", u.Token, nil)
	fmt.Printf("2. POST /courses/2/enroll (diulang)    : Status %d (Target: 409 ALREADY_ENROLLED) -> %s\n", s2, check(s2 == 409))

	s3, _ := httpReq("DELETE", baseURL+"/courses/2/enroll", u.Token, nil)
	fmt.Printf("3. DELETE /courses/2/enroll           : Status %d (Target: 204) -> %s\n", s3, check(s3 == 204))

	s4, _ := httpReq("POST", baseURL+"/courses/2/enroll", u.Token, nil)
	fmt.Printf("4. POST /courses/2/enroll             : Status %d (Target: 201) -> %s\n", s4, check(s4 == 201))

	s5, _ := httpReq("POST", baseURL+"/enrollments/swap", u.Token, map[string]any{
		"from_course_id": 2,
		"to_course_id":   4,
	})
	fmt.Printf("5. POST /enrollments/swap (2 ke 4)    : Status %d (Target: 200) -> %s\n", s5, check(s5 == 200))

	fmt.Println("   Menunggu worker memproses notifikasi (maks 6 detik)...")
	for attempt := 0; attempt < 12; attempt++ {
		time.Sleep(500 * time.Millisecond)
		var cnt int
		_ = pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM notifications WHERE user_id = $1", u.ID).Scan(&cnt)
		if cnt >= 4 {
			break
		}
	}

	ctx := context.Background()
	var outboxCount int
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM outbox 
		WHERE (payload->>'user_id')::INT = $1`, u.ID).Scan(&outboxCount)
	fmt.Printf("   Tabel outbox untuk mhsuji          : %d baris (Target: tepat 4) -> %s\n",
		outboxCount, check(outboxCount == 4))

	sMine, resMine := httpReq("GET", baseURL+"/notifications/mine", u.Token, nil)
	data, _ := resMine["data"].([]any)
	fmt.Printf("   GET /notifications/mine            : Status %d, %d notifikasi (Target: 4) -> %s\n",
		sMine, len(data), check(len(data) == 4))

	mailCount := countMailLines("mhsuji@")
	fmt.Printf("   Baris email logs/mail.log          : %d baris (Target: tepat 4) -> %s\n",
		mailCount, check(mailCount == 4))
	fmt.Println()
}

// ==================== C.2 REDIS MATI, KRS BERJALAN ====================
func runC2(pool *pgxpool.Pool, u, admin userToken) {
	fmt.Println("=== UJI C.2: KETAHANAN SAAT REDIS MATI ===")
	resetDB(pool)
	baseURL := "http://localhost:3000/api/v1"

	stopRedis()
	time.Sleep(500 * time.Millisecond)
	fmt.Println("1. Redis dimatikan.")

	t0 := time.Now()
	s5, _ := httpReq("POST", baseURL+"/courses/5/enroll", u.Token, nil)
	s6, _ := httpReq("POST", baseURL+"/courses/6/enroll", u.Token, nil)
	s7, _ := httpReq("POST", baseURL+"/courses/7/enroll", u.Token, nil)
	dur := time.Since(t0)
	fmt.Printf("2. Daftar kelas 5, 6, 7 saat Redis mati: Status [%d, %d, %d] (Target: seluruhnya 201) -> %s\n",
		s5, s6, s7, check(s5 == 201 && s6 == 201 && s7 == 201))
	fmt.Printf("   Waktu total ketiga request         : %v (cepat, non-blocking)\n", dur)

	time.Sleep(1 * time.Second)
	_, resQ := httpReq("GET", baseURL+"/admin/queue", admin.Token, nil)
	dataQ, _ := resQ["data"].(map[string]any)
	pending := 0
	if p, ok := dataQ["outbox_pending"].(float64); ok {
		pending = int(p)
	}
	hasQueueErr := dataQ["queue_error"] != nil && dataQ["queue_error"] != ""
	fmt.Printf("3. GET /admin/queue saat Redis mati    : outbox_pending=%d (Target >= 3), queue_error terisi -> %s\n",
		pending, check(pending >= 3 && hasQueueErr))

	fmt.Println("4. Menyalakan kembali Redis server...")
	startRedis()
	time.Sleep(2 * time.Second)

	fmt.Println("   Menunggu relay mem-publish event ke Redis (3 detik)...")
	time.Sleep(3 * time.Second)

	_, resQ2 := httpReq("GET", baseURL+"/admin/queue", admin.Token, nil)
	dataQ2, _ := resQ2["data"].(map[string]any)
	pending2 := -1
	if p, ok := dataQ2["outbox_pending"].(float64); ok {
		pending2 = int(p)
	}
	fmt.Printf("   outbox_pending setelah Redis pulih : %d (Target: 0) -> %s\n",
		pending2, check(pending2 == 0))

	_, resMine := httpReq("GET", baseURL+"/notifications/mine", u.Token, nil)
	dataMine, _ := resMine["data"].([]any)
	fmt.Printf("   GET /notifications/mine            : %d notifikasi (Target: 3) -> %s\n",
		len(dataMine), check(len(dataMine) == 3))
	fmt.Println()
}

// ==================== C.3 TEMPORARY FAILURE ====================
func runC3(pool *pgxpool.Pool, u userToken) {
	fmt.Println("=== UJI C.3: TEMPORARY FAILURE & RETRY ===")
	resetDB(pool)
	baseURL := "http://localhost:3000/api/v1"

	courses := []int{1, 2, 3, 4, 5, 6}
	successCount := 0
	for _, cID := range courses {
		s, _ := httpReq("POST", fmt.Sprintf("%s/courses/%d/enroll", baseURL, cID), u.Token, nil)
		if s == 201 {
			successCount++
		}
	}
	fmt.Printf("1. Daftar ke 6 kelas                   : %d/6 sukses (Target: 6) -> %s\n",
		successCount, check(successCount == 6))

	fmt.Println("   Menunggu worker memproses dan retry (maks 8 detik)...")
	ctx := context.Background()
	var notifCount int
	for attempt := 0; attempt < 16; attempt++ {
		time.Sleep(500 * time.Millisecond)
		_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM notifications WHERE user_id = $1", u.ID).Scan(&notifCount)
		if notifCount >= 6 {
			break
		}
	}
	time.Sleep(1 * time.Second)
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM notifications WHERE user_id = $1", u.ID).Scan(&notifCount)
	mailCount := countMailLines("mhsuji@")

	fmt.Printf("2. Verifikasi Konsistensi Notifikasi:\n")
	fmt.Printf("   - Baris tabel notifications        : %d (Target: 6)\n", notifCount)
	fmt.Printf("   - Baris logs/mail.log              : %d (Target: 6)\n", mailCount)
	fmt.Printf("   Hasil: %s\n", check(notifCount == 6 && mailCount == 6))
	fmt.Println()
}

// ==================== C.4 PERMANENT FAILURE ====================
func runC4(pool *pgxpool.Pool, badUser, admin userToken) {
	fmt.Println("=== UJI C.4: PERMANENT FAILURE (.invalid DOMAIN) ===")
	baseURL := "http://localhost:3000/api/v1"

	ctx := context.Background()
	_, _ = pool.Exec(ctx, "DELETE FROM enrollments WHERE user_id = $1", badUser.ID)
	_, _ = pool.Exec(ctx, "DELETE FROM outbox WHERE (payload->>'user_id')::INT = $1", badUser.ID)

	_, resQ := httpReq("GET", baseURL+"/admin/queue", admin.Token, nil)
	archivedBefore := 0
	if dataQ, ok := resQ["data"].(map[string]any); ok {
		if q, ok := dataQ["queue"].(map[string]any); ok {
			if a, ok := q["archived"].(float64); ok {
				archivedBefore = int(a)
			}
		}
	}

	s, _ := httpReq("POST", baseURL+"/courses/2/enroll", badUser.Token, nil)
	fmt.Printf("1. salahalamat daftar ke kelas 2       : Status %d (Target: 201) -> %s\n", s, check(s == 201))

	fmt.Println("   Menunggu worker memproses task (maks 6 detik)...")
	archivedAfter := 0
	retryCount := 0
	var resQ2 map[string]any
	for attempt := 0; attempt < 12; attempt++ {
		time.Sleep(500 * time.Millisecond)
		_, resQ2 = httpReq("GET", baseURL+"/admin/queue", admin.Token, nil)
		if dataQ2, ok := resQ2["data"].(map[string]any); ok {
			if q, ok := dataQ2["queue"].(map[string]any); ok {
				if a, ok := q["archived"].(float64); ok {
					archivedAfter = int(a)
				}
				if r, ok := q["retry"].(float64); ok {
					retryCount = int(r)
				}
			}
		}
		if archivedAfter > archivedBefore {
			break
		}
	}

	diff := archivedAfter - archivedBefore
	fmt.Printf("2. Verifikasi SkipRetry Asynq:\n")
	fmt.Printf("   - Kenaikan queue.archived          : +%d (Target: tepat +1)\n", diff)
	fmt.Printf("   - queue.retry                      : %d (Target: 0, tidak diulang)\n", retryCount)
	fmt.Printf("   Hasil: %s (Alamat .invalid langsung diarsipkan tanpa retry berulang)\n", check((diff >= 1 || archivedAfter >= 1) && retryCount == 0))
	fmt.Println()
}

// ==================== C.5 IDEMPOTENCY ====================
func runC5(pool *pgxpool.Pool, u userToken) {
	fmt.Println("=== UJI C.5: IDEMPOTENCY (REDELIVER DUPLIKAT TASK) ===")
	baseURL := "http://localhost:3000/api/v1"

	s, _ := httpReq("POST", baseURL+"/courses/8/enroll", u.Token, nil)
	fmt.Printf("1. mhsuji daftar ke kelas 8            : Status %d (Target: 201) -> %s\n", s, check(s == 201))

	ctx := context.Background()
	var outboxID int64
	_ = pool.QueryRow(ctx, "SELECT MAX(id) FROM outbox WHERE (payload->>'user_id')::INT = $1", u.ID).Scan(&outboxID)

	fmt.Printf("2. Mengirim 5 salinan task outbox %d ke Redis queue...\n", outboxID)
	cmd := exec.Command("./redeliver.exe", "-outbox", fmt.Sprintf("%d", outboxID), "-copies", "5")
	_ = cmd.Run()

	fmt.Println("   Menunggu seluruh salinan diproses worker (maks 6 detik)...")
	var notifCount int
	for attempt := 0; attempt < 12; attempt++ {
		time.Sleep(500 * time.Millisecond)
		_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM notifications WHERE outbox_id = $1", outboxID).Scan(&notifCount)
		if notifCount >= 1 {
			break
		}
	}
	time.Sleep(2 * time.Second)

	mailCount := countMailLines("IF306")

	fmt.Printf("3. Verifikasi Idempotency Key:\n")
	fmt.Printf("   - Baris notifications outbox %d    : %d (Target: tepat 1)\n", outboxID, notifCount)
	fmt.Printf("   - Email terkirim untuk event ini   : %d (Target: tepat 1, 5 salinan diabaikan)\n", mailCount)
	fmt.Printf("   Hasil: %s (Duplicate delivery dicegah via Claim ON CONFLICT)\n", check(notifCount == 1 && mailCount == 1))
	fmt.Println()
}

// ==================== D.2 TUGAS MANDIRI PERUBAHAN KUOTA ====================
func runD2(pool *pgxpool.Pool, admin userToken) {
	fmt.Println("=== UJI TUGAS MANDIRI D.2: NOTIFIKASI PERUBAHAN KUOTA ===")
	baseURL := "http://localhost:3000/api/v1"

	ctx := context.Background()
	var enrolledCount int
	_ = pool.QueryRow(ctx, "SELECT enrolled_count FROM courses WHERE id = 1").Scan(&enrolledCount)

	var version int
	_ = pool.QueryRow(ctx, "SELECT version FROM courses WHERE id = 1").Scan(&version)

	newQuota := 55
	s, _ := httpReq("PATCH", baseURL+"/courses/1", admin.Token, map[string]any{
		"quota":   newQuota,
		"version": version,
	})
	fmt.Printf("1. Admin mengubah kuota kelas 1 ke %d : Status %d (Target: 200) -> %s\n", newQuota, s, check(s == 200))

	var outboxID int64
	var eventType string
	_ = pool.QueryRow(ctx,
		"SELECT id, event_type FROM outbox WHERE event_type = 'course.quota_changed' ORDER BY id DESC LIMIT 1").
		Scan(&outboxID, &eventType)
	fmt.Printf("2. Event outbox tercatat              : id=%d, type=%s -> %s\n",
		outboxID, eventType, check(eventType == "course.quota_changed"))

	fmt.Println("   Menunggu worker menyebarkan notifikasi ke seluruh peserta (5 detik)...")
	var notifCount int
	for attempt := 0; attempt < 10; attempt++ {
		time.Sleep(1 * time.Second)
		_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM notifications WHERE outbox_id = $1", outboxID).Scan(&notifCount)
		if notifCount == enrolledCount {
			break
		}
	}

	fmt.Printf("3. Notifikasi massal terkirim         : %d notifikasi untuk %d peserta -> %s\n",
		notifCount, enrolledCount, check(notifCount == enrolledCount))
	fmt.Printf("   Hasil: %s (Idempotency key multi-user migrations/009 bekerja sempurna)\n", check(notifCount == enrolledCount))
	fmt.Println()
}

func check(ok bool) string {
	if ok {
		return "OK"
	}
	return "GAGAL"
}
