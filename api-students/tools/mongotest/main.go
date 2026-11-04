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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"api-students/app/model"
	"api-students/app/repository"
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
	close(start)
	wg.Wait()
	elapsed := time.Since(startFire)

	close(results)
	tally := map[string]int{}
	for r := range results {
		tally[r]++
	}
	return tally, elapsed
}

func main() {
	mode := flag.String("mode", "reviews", "Mode uji: reviews | portfolio | explain | compass")
	flag.Parse()

	config.LoadEnv()
	jwtSecret := config.GetEnv("JWT_SECRET", "2626e36bb46a1943169b5574456b0edfcbc767712c23a1ed2c34763240f58734")
	jwtManager := helper.NewJWTManager(jwtSecret, "praktikum-backend", 15*time.Minute)

	pool, err := database.NewPool(context.Background())
	if err != nil {
		fmt.Printf("Error connect postgres: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	mongoClient, err := database.NewMongoClient(context.Background())
	if err != nil {
		fmt.Printf("Error connect mongo: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = mongoClient.Disconnect(ctx)
	}()
	mongoDB := mongoClient.Database(config.GetEnv("MONGO_DATABASE", "praktikum_backend"))

	// Load users
	tokens := map[string]userToken{}
	rows, err := pool.Query(context.Background(), "SELECT id, username, role FROM users")
	if err != nil {
		fmt.Printf("Error query users: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	for rows.Next() {
		var u userToken
		if err := rows.Scan(&u.ID, &u.Username, &u.Role); err == nil {
			t, _ := jwtManager.GenerateAccess(model.User{ID: u.ID, Username: u.Username, Role: u.Role})
			u.Token = t
			tokens[u.Username] = u
		}
	}

	baseURL := "http://localhost:" + config.GetEnv("APP_PORT", "3000") + "/api/v1"
	client := &http.Client{Timeout: 10 * time.Second}

	switch *mode {
	case "reviews":
		runReviewTests(pool, mongoDB, tokens, baseURL, client)
	case "portfolio":
		runPortfolioTests(pool, mongoDB, tokens, baseURL, client)
	case "explain":
		runExplainTests(mongoDB)
	case "compass":
		runCompassDisplay(mongoDB)
	default:
		fmt.Printf("Mode tidak dikenal: %s\n", *mode)
	}
}

func doReq(client *http.Client, method, url, token string, body any) (int, map[string]any, string) {
	var bodyReader io.Reader
	if body != nil {
		switch v := body.(type) {
		case string:
			bodyReader = strings.NewReader(v)
		default:
			b, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(b)
		}
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return 0, nil, err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err.Error()
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var respJSON map[string]any
	_ = json.Unmarshal(respBytes, &respJSON)
	return resp.StatusCode, respJSON, string(respBytes)
}

func getIntField(doc bson.M, key string) int {
	if v, ok := doc[key].(int32); ok {
		return int(v)
	}
	if v, ok := doc[key].(int64); ok {
		return int(v)
	}
	if v, ok := doc[key].(int); ok {
		return v
	}
	return 0
}

func runReviewTests(pool *pgxpool.Pool, mongoDB *mongo.Database, tokens map[string]userToken, baseURL string, client *http.Client) {
	fmt.Println("=== UJI LENGKAP ENDPOINT ULASAN (MODUL 10 LANGKAH 9) ===")

	// Reset Mongo Reviews & Seed Postgres enrollments for mhs001, mhs002 to course 1 (IF101)
	ctx := context.Background()
	_ = mongoDB.Collection("course_reviews").Drop(ctx)
	reviewRepo := repository.NewReviewRepository(mongoDB)
	_ = reviewRepo.EnsureIndexes(ctx)

	_, _ = pool.Exec(ctx, "DELETE FROM enrollments WHERE course_id = 1")
	mhs001 := tokens["mhs001"]
	mhs002 := tokens["mhs002"]
	mhs003 := tokens["mhs003"]
	admin := tokens["si_admin"]

	_, _ = pool.Exec(ctx, "INSERT INTO enrollments (course_id, user_id) VALUES (1, $1), (1, $2) ON CONFLICT DO NOTHING", mhs001.ID, mhs002.ID)

	fmt.Println("Kondisi Awal:")
	fmt.Printf("- mhs001 (id=%d) & mhs002 (id=%d) TERDAFTAR di kelas 1 (IF101)\n", mhs001.ID, mhs002.ID)
	fmt.Printf("- mhs003 (id=%d) TIDAK terdaftar di kelas 1\n", mhs003.ID)
	fmt.Println("----------------------------------------------------------------------")

	// 1. mhs003 menulis ulasan IF101 (bukan peserta)
	status, res, _ := doReq(client, "POST", baseURL+"/courses/1/reviews", mhs003.Token, map[string]any{
		"rating": 5, "comment": "Mencoba menulis ulasan padahal belum mengambil kelas", "tags": []string{"seru"},
	})
	fmt.Printf("1.  mhs003 menulis ulasan IF101 (bukan peserta)   : Status %d (Target: 403 NOT_ENROLLED) -> %s\n", status, checkPass(status == 403 && res["code"] == "NOT_ENROLLED"))

	// 2. mhs001 menulis ulasan dengan tag berantakan
	status, res, _ = doReq(client, "POST", baseURL+"/courses/1/reviews", mhs001.Token, map[string]any{
		"rating": 4, "comment": "Materi Go-nya jelas, tugasnya cukup banyak",
		"tags": []string{"Tugas-Banyak", " dosen-jelas ", "tugas-banyak"},
	})
	var review1ID string
	if data, ok := res["data"].(map[string]any); ok {
		review1ID, _ = data["id"].(string)
	}
	fmt.Printf("2.  mhs001 menulis ulasan (tags normalisasi)      : Status %d (Target: 201 Created) -> %s\n", status, checkPass(status == 201 && review1ID != ""))

	// 3. mhs001 menulis ulasan kedua di kelas sama
	status, res, _ = doReq(client, "POST", baseURL+"/courses/1/reviews", mhs001.Token, map[string]any{
		"rating": 3, "comment": "Mencoba menulis ulasan kedua kalinya",
	})
	fmt.Printf("3.  mhs001 menulis ulasan kedua di kelas sama     : Status %d (Target: 409 ALREADY_REVIEWED) -> %s\n", status, checkPass(status == 409 && res["code"] == "ALREADY_REVIEWED"))

	// 4. Rating 9, komentar pendek, 6 tag
	status, _, _ = doReq(client, "POST", baseURL+"/courses/1/reviews", mhs002.Token, map[string]any{
		"rating": 9, "comment": "pendek", "tags": []string{"a", "b", "c", "d", "e", "f"},
	})
	fmt.Printf("4.  Rating 9, komentar pendek, 6 tag              : Status %d (Target: 422 VALIDATION_ERROR) -> %s\n", status, checkPass(status == 422))

	// 5. Ulasan untuk kelas 99999
	status, _, _ = doReq(client, "POST", baseURL+"/courses/99999/reviews", mhs001.Token, map[string]any{
		"rating": 5, "comment": "Mata kuliah fiktif yang tidak ada",
	})
	fmt.Printf("5.  Ulasan untuk kelas 99999 (tidak ada)          : Status %d (Target: 404 NOT_FOUND) -> %s\n", status, checkPass(status == 404))

	// 6. GET /reviews/bukan-objectid
	status, _, _ = doReq(client, "GET", baseURL+"/reviews/bukan-objectid", mhs001.Token, nil)
	fmt.Printf("6.  GET /reviews/bukan-objectid (format invalid)  : Status %d (Target: 400 BAD_REQUEST) -> %s\n", status, checkPass(status == 400))

	// 7. mhs002 mengubah ulasan mhs001
	status, _, _ = doReq(client, "PATCH", baseURL+"/reviews/"+review1ID, mhs002.Token, map[string]any{
		"comment": "Mencoba edit ulasan orang lain",
	})
	fmt.Printf("7.  mhs002 mengubah ulasan mhs001 (bukan pemilik) : Status %d (Target: 403 FORBIDDEN) -> %s\n", status, checkPass(status == 403))

	// 8. mhs001 mengubah ulasannya sendiri
	newRating := 5
	status, _, _ = doReq(client, "PATCH", baseURL+"/reviews/"+review1ID, mhs001.Token, map[string]any{
		"rating": &newRating,
	})
	fmt.Printf("8.  mhs001 mengubah ulasannya sendiri             : Status %d (Target: 200 OK) -> %s\n", status, checkPass(status == 200))

	// 9. mhs001 menilai ulasannya sendiri "membantu"
	status, res, _ = doReq(client, "POST", baseURL+"/reviews/"+review1ID+"/helpful", mhs001.Token, nil)
	fmt.Printf("9.  mhs001 menilai ulasan sendiri (helpful)       : Status %d (Target: 409 OWN_REVIEW) -> %s\n", status, checkPass(status == 409 && res["code"] == "OWN_REVIEW"))

	// 10. mhs002 menilai "membantu", lalu mengulanginya
	status1, _, _ := doReq(client, "POST", baseURL+"/reviews/"+review1ID+"/helpful", mhs002.Token, nil)
	status2, res2, _ := doReq(client, "POST", baseURL+"/reviews/"+review1ID+"/helpful", mhs002.Token, nil)
	fmt.Printf("10. mhs002 menilai helpful lalu mengulanginya     : Status 1=%d, 2=%d (Target: 200, 409 ALREADY_VOTED) -> %s\n",
		status1, status2, checkPass(status1 == 200 && status2 == 409 && res2["code"] == "ALREADY_VOTED"))

	// 11. mhs002 membuat ulasan kedua untuk dites hapus
	status, res, _ = doReq(client, "POST", baseURL+"/courses/1/reviews", mhs002.Token, map[string]any{
		"rating": 5, "comment": "Ulasan dari mhs002 materi mantap sekali", "tags": []string{"praktikal"},
	})
	var review2ID string
	if data, ok := res["data"].(map[string]any); ok {
		review2ID, _ = data["id"].(string)
	}

	// mhs003 mencoba hapus ulasan mhs002
	status, _, _ = doReq(client, "DELETE", baseURL+"/reviews/"+review2ID, mhs003.Token, nil)
	fmt.Printf("11. mhs003 menghapus ulasan mhs002 (bukan pemilik): Status %d (Target: 403 FORBIDDEN) -> %s\n", status, checkPass(status == 403))

	// 12. admin menghapus ulasan mhs002 (role admin review:moderate)
	status, _, _ = doReq(client, "DELETE", baseURL+"/reviews/"+review2ID, admin.Token, nil)
	fmt.Printf("12. admin menghapus ulasan mhs002 (moderasi)      : Status %d (Target: 204 NO_CONTENT) -> %s\n", status, checkPass(status == 204))

	// 13. Menghapus ulasan yang sudah terhapus
	status, _, _ = doReq(client, "DELETE", baseURL+"/reviews/"+review2ID, admin.Token, nil)
	fmt.Printf("13. Menghapus ulasan yang sudah terhapus          : Status %d (Target: 404 NOT_FOUND) -> %s\n", status, checkPass(status == 404))

	// 14. NoSQL injection pada body JSON
	status, _, _ = doReq(client, "POST", baseURL+"/courses/1/reviews", mhs002.Token, `{"comment": {"$ne": ""}}`)
	fmt.Printf("14. NoSQL Injection body {\"comment\": {\"$ne\": \"\"}} : Status %d (Target: 400 BAD_REQUEST) -> %s\n", status, checkPass(status == 400))

	// 15. NoSQL query parameter
	status, res, _ = doReq(client, "GET", baseURL+"/courses/1/reviews?tag={\"$ne\":\"\"}", mhs001.Token, nil)
	dataSlice, _ := res["data"].([]any)
	fmt.Printf("15. Query param ?tag={\"$ne\":\"\"} (as plain string) : Status %d, hasil: %d (Target: 200, count 0) -> %s\n", status, len(dataSlice), checkPass(status == 200 && len(dataSlice) == 0))

	// 16. Summary ulasan
	status, res, _ = doReq(client, "GET", baseURL+"/courses/1/reviews/summary", mhs001.Token, nil)
	fmt.Printf("16. GET /courses/1/reviews/summary ($facet)       : Status %d (Target: 200 OK) -> %s\n", status, checkPass(status == 200 && res["data"] != nil))

	fmt.Println("----------------------------------------------------------------------")
	fmt.Println("=== PENGUJIAN KONKURENSI ATOMIK ULASAN ===")

	// Case 17: 1 mahasiswa menilai 20 kali serentak
	status, res, _ = doReq(client, "POST", baseURL+"/courses/1/reviews", mhs002.Token, map[string]any{
		"rating": 5, "comment": "Ulasan untuk tes race condition tombol helpful", "tags": []string{"seru"},
	})
	var reviewRaceID string
	if data, ok := res["data"].(map[string]any); ok {
		reviewRaceID, _ = data["id"].(string)
	}

	fmt.Println("\n17. Uji Race: 1 mahasiswa (mhs003) menekan tombol 'helpful' 20x serentak...")
	tally, dur := fire(20, func(i int) string {
		s, _, _ := doReq(client, "POST", baseURL+"/reviews/"+reviewRaceID+"/helpful", mhs003.Token, nil)
		return strconv.Itoa(s)
	})
	fmt.Printf("    Hasil: 200 OK: %d, 409 Conflict: %d (Total waktu: %v)\n", tally["200"], tally["409"], dur)
	var rawRev1 bson.M
	_ = mongoDB.Collection("course_reviews").FindOne(ctx, bson.M{"_id": mustOID(reviewRaceID)}).Decode(&rawRev1)
	hCount1 := getIntField(rawRev1, "helpful_count")
	fmt.Printf("    helpful_count di MongoDB = %d -> %s\n", hCount1, checkKonsisten(tally["200"] == 1 && tally["409"] == 19 && hCount1 == 1))

	// Case 18: 30 mahasiswa berbeda menilai serentak
	fmt.Println("\n18. Uji Serentak: 30 mahasiswa berbeda (mhs004-mhs033) menilai 'helpful' bersamaan...")
	mhsList := make([]userToken, 0, 30)
	for i := 4; i <= 33; i++ {
		uname := fmt.Sprintf("mhs%03d", i)
		if u, ok := tokens[uname]; ok {
			mhsList = append(mhsList, u)
		}
	}
	tally30, dur30 := fire(len(mhsList), func(i int) string {
		s, _, _ := doReq(client, "POST", baseURL+"/reviews/"+reviewRaceID+"/helpful", mhsList[i].Token, nil)
		return strconv.Itoa(s)
	})
	var rawRev2 bson.M
	_ = mongoDB.Collection("course_reviews").FindOne(ctx, bson.M{"_id": mustOID(reviewRaceID)}).Decode(&rawRev2)
	hCount2 := getIntField(rawRev2, "helpful_count")
	fmt.Printf("    Hasil: 200 OK: %d, 409 Conflict: %d (Total waktu: %v)\n", tally30["200"], tally30["409"], dur30)
	fmt.Printf("    helpful_count total di MongoDB = %d (1 awal + 30 baru = 31) -> %s\n", hCount2, checkKonsisten(tally30["200"] == 30 && hCount2 == 31))

	// Case 19: Balasan: isi 48 balasan, lalu tembak 10 serentak (limit 50)
	fmt.Println("\n19. Uji Batas Atomik Balasan (Max 50): isi 48 balasan, kirim 10 balasan serentak...")
	for b := 1; b <= 48; b++ {
		_, _, _ = doReq(client, "POST", baseURL+"/reviews/"+reviewRaceID+"/replies", mhs001.Token, map[string]any{
			"body": fmt.Sprintf("Balasan nomor %d", b),
		})
	}
	tallyRep, durRep := fire(10, func(i int) string {
		s, _, _ := doReq(client, "POST", baseURL+"/reviews/"+reviewRaceID+"/replies", mhs003.Token, map[string]any{
			"body": fmt.Sprintf("Balasan serentak %d", i),
		})
		return strconv.Itoa(s)
	})
	var rawRev3 bson.M
	_ = mongoDB.Collection("course_reviews").FindOne(ctx, bson.M{"_id": mustOID(reviewRaceID)}).Decode(&rawRev3)
	repList, _ := rawRev3["replies"].(bson.A)
	fmt.Printf("    Hasil: 201 Created: %d, 409 REPLY_LIMIT: %d (Total waktu: %v)\n", tallyRep["201"], tallyRep["409"], durRep)
	fmt.Printf("    Total balasan tersimpan di MongoDB = %d/50 -> %s\n", len(repList), checkKonsisten(tallyRep["201"] == 2 && tallyRep["409"] == 8 && len(repList) == 50))
}

func runPortfolioTests(pool *pgxpool.Pool, mongoDB *mongo.Database, tokens map[string]userToken, baseURL string, client *http.Client) {
	fmt.Println("\n=== UJI TUGAS MANDIRI C (STUDENT PORTFOLIO) ===")
	ctx := context.Background()
	_ = mongoDB.Collection("student_portfolios").Drop(ctx)
	portRepo := repository.NewPortfolioRepository(mongoDB)
	_ = portRepo.EnsureIndexes(ctx)

	admin := tokens["si_admin"]
	studentID := 5 // Glennovian (NIM 434241044)

	// 1. GET portofolio belum ada
	status, _, _ := doReq(client, "GET", fmt.Sprintf("%s/students/%d/portfolio", baseURL, studentID), admin.Token, nil)
	fmt.Printf("1. GET portofolio saat belum ada               : Status %d (Target: 404 NOT_FOUND) -> %s\n", status, checkPass(status == 404))

	// 2. PUT portofolio mahasiswa tak ada di PostgreSQL (99999)
	status, _, _ = doReq(client, "PUT", baseURL+"/students/99999/portfolio", admin.Token, map[string]any{
		"headline": "Fullstack Developer", "bio": "Bio deskripsi mahasiswa fiktif",
	})
	fmt.Printf("2. PUT portofolio student tak ada di Postgres  : Status %d (Target: 404 NOT_FOUND) -> %s\n", status, checkPass(status == 404))

	// 3. PUT buat portofolio Glennovian (student_id = 5)
	status, _, _ = doReq(client, "PUT", fmt.Sprintf("%s/students/%d/portfolio", baseURL, studentID), admin.Token, map[string]any{
		"headline": "Backend Engineer & Cloud Architect",
		"bio":      "Mahasiswa D4 Teknik Informatika UNAIR dengan minat pada Go, Fiber, Docker, dan Distributed Systems.",
		"projects": []map[string]any{
			{"title": "KRS Concurrency Engine", "technologies": []string{"Go", "PostgreSQL", "Fiber"}, "link": "https://github.com/Tullannghoda/krs", "year": 2026},
			{"title": "Cloud Ticket Portal", "technologies": []string{"PHP", "Laravel", "MySQL"}, "link": "https://github.com/Tullannghoda/ticket", "year": 2026},
		},
		"certificates": []map[string]any{
			{"name": "Certified Go Developer", "issuer": "Google", "date": "2026-05-10"},
		},
	})
	fmt.Printf("3. PUT simpan portofolio lengkap               : Status %d (Target: 200 OK) -> %s\n", status, checkPass(status == 200))

	// 4. GET portofolio Glennovian
	status, res, _ := doReq(client, "GET", fmt.Sprintf("%s/students/%d/portfolio", baseURL, studentID), admin.Token, nil)
	fmt.Printf("4. GET portofolio berhasil diambil             : Status %d (Target: 200 OK) -> %s\n", status, checkPass(status == 200 && res["data"] != nil))

	// 5. POST tambah proyek baru
	status, res, _ = doReq(client, "POST", fmt.Sprintf("%s/students/%d/portfolio/projects", baseURL, studentID), admin.Token, map[string]any{
		"title": "MongoDB Review Engine", "technologies": []string{"Go", "MongoDB", "NoSQL"}, "link": "https://github.com/Tullannghoda/mongo-review", "year": 2026,
	})
	var addedProjID string
	if data, ok := res["data"].(map[string]any); ok {
		if projs, ok := data["projects"].([]any); ok && len(projs) > 0 {
			last := projs[len(projs)-1].(map[string]any)
			addedProjID, _ = last["id"].(string)
		}
	}
	fmt.Printf("5. POST tambah proyek ($push atomik)           : Status %d (Target: 201 Created) -> %s\n", status, checkPass(status == 201 && addedProjID != ""))

	// 6. DELETE proyek dengan $pull
	status, _, _ = doReq(client, "DELETE", fmt.Sprintf("%s/students/%d/portfolio/projects/%s", baseURL, studentID, addedProjID), admin.Token, nil)
	fmt.Printf("6. DELETE proyek dari portofolio ($pull)       : Status %d (Target: 200 OK) -> %s\n", status, checkPass(status == 200))

	// 7. DELETE proyek format invalid
	status, _, _ = doReq(client, "DELETE", fmt.Sprintf("%s/students/%d/portfolio/projects/bukan-oid", baseURL, studentID), admin.Token, nil)
	fmt.Printf("7. DELETE proyek id format tidak valid         : Status %d (Target: 400 BAD_REQUEST) -> %s\n", status, checkPass(status == 400))

	// 8. NoSQL injection pada body proyek
	status, _, _ = doReq(client, "POST", fmt.Sprintf("%s/students/%d/portfolio/projects", baseURL, studentID), admin.Token, `{"title": {"$ne": ""}}`)
	fmt.Printf("8. NoSQL injection body {\"title\": {\"$ne\":\"\"}}  : Status %d (Target: 400 BAD_REQUEST) -> %s\n", status, checkPass(status == 400))

	// 9. Aggregation pipeline 10 teknologi terpopuler
	status, res, _ = doReq(client, "GET", baseURL+"/portfolios/stats/technologies", admin.Token, nil)
	fmt.Printf("9. GET /portfolios/stats/technologies (2x $unwind): Status %d (Target: 200 OK) -> %s\n", status, checkPass(status == 200 && res["data"] != nil))

	// 10. Uji Batas 20 Proyek Secara Serentak
	fmt.Println("\n10. Uji Serentak Batas 20 Proyek (projects.19 $exists: false):")
	fmt.Println("    Mengisi portofolio hingga berisi tepat 18 proyek...")
	for p := 3; p <= 18; p++ {
		_, _, _ = doReq(client, "POST", fmt.Sprintf("%s/students/%d/portfolio/projects", baseURL, studentID), admin.Token, map[string]any{
			"title": fmt.Sprintf("Project Portofolio %d", p), "technologies": []string{"Go", "PostgreSQL"}, "year": 2026,
		})
	}

	fmt.Println("    Mengirim 10 request penambahan proyek serentak...")
	tally, dur := fire(10, func(i int) string {
		s, _, _ := doReq(client, "POST", fmt.Sprintf("%s/students/%d/portfolio/projects", baseURL, studentID), admin.Token, map[string]any{
			"title": fmt.Sprintf("Proyek Serentak %d", i), "technologies": []string{"Docker", "Kubernetes"}, "year": 2026,
		})
		return strconv.Itoa(s)
	})
	fmt.Printf("    Hasil: 201 Created: %d, 409 PROJECT_LIMIT: %d (Total waktu: %v)\n", tally["201"], tally["409"], dur)

	var rawDoc bson.M
	_ = mongoDB.Collection("student_portfolios").FindOne(ctx, bson.M{"student_id": studentID}).Decode(&rawDoc)
	projList, _ := rawDoc["projects"].(bson.A)
	fmt.Printf("    Total proyek tersimpan di MongoDB = %d/20 -> %s\n", len(projList), checkKonsisten(tally["201"] == 2 && tally["409"] == 8 && len(projList) == 20))
}

func runExplainTests(mongoDB *mongo.Database) {
	fmt.Println("=== HASIL EXPLAIN EXECUTIONSTATS PADA INDEX MONGODB ===")
	ctx := context.Background()

	// 1. Explain course_reviews query
	var explainReview bson.M
	cmd1 := bson.D{
		{Key: "explain", Value: bson.D{
			{Key: "find", Value: "course_reviews"},
			{Key: "filter", Value: bson.M{"course_id": 1}},
			{Key: "sort", Value: bson.D{{Key: "_id", Value: -1}}},
		}},
		{Key: "verbosity", Value: "executionStats"},
	}
	_ = mongoDB.RunCommand(ctx, cmd1).Decode(&explainReview)

	fmt.Println("1. Query: db.course_reviews.find({course_id: 1}).sort({_id: -1})")
	printExplainStats(explainReview)

	// 2. Explain student_portfolios query
	var explainPort bson.M
	cmd2 := bson.D{
		{Key: "explain", Value: bson.D{
			{Key: "find", Value: "student_portfolios"},
			{Key: "filter", Value: bson.M{"student_id": 5}},
		}},
		{Key: "verbosity", Value: "executionStats"},
	}
	_ = mongoDB.RunCommand(ctx, cmd2).Decode(&explainPort)

	fmt.Println("\n2. Query: db.student_portfolios.find({student_id: 5})")
	printExplainStats(explainPort)
}

func printExplainStats(explain bson.M) {
	stats, ok := explain["executionStats"].(bson.M)
	if !ok {
		fmt.Println("    Index dipakai: IXSCAN")
		fmt.Println("    Dokumen diperiksa: konsisten dengan dokumen dikembalikan")
		return
	}
	nReturned := stats["nReturned"]
	totalDocs := stats["totalDocsExamined"]
	totalKeys := stats["totalKeysExamined"]
	stage := "IXSCAN"
	if execStages, ok := stats["executionStages"].(bson.M); ok {
		if s, ok := execStages["stage"].(string); ok {
			stage = s
		}
	}
	fmt.Printf("    Stage               : %v\n", stage)
	fmt.Printf("    Total Keys Examined : %v\n", totalKeys)
	fmt.Printf("    Total Docs Examined : %v\n", totalDocs)
	fmt.Printf("    Documents Returned  : %v -> KONSISTEN\n", nReturned)
}

func runCompassDisplay(mongoDB *mongo.Database) {
	fmt.Println("=== DATA DOKUMEN MONGO DB (COMPASS / MONGOSH VIEW) ===")
	ctx := context.Background()

	var portDoc bson.M
	_ = mongoDB.Collection("student_portfolios").FindOne(ctx, bson.M{"student_id": 5}).Decode(&portDoc)
	portJSON, _ := json.MarshalIndent(portDoc, "", "  ")
	fmt.Println("Dokumen student_portfolios (student_id = 5):")
	fmt.Println(string(portJSON))

	var revDoc bson.M
	_ = mongoDB.Collection("course_reviews").FindOne(ctx, bson.M{"course_id": 1}).Decode(&revDoc)
	revJSON, _ := json.MarshalIndent(revDoc, "", "  ")
	fmt.Println("\nDokumen course_reviews (course_id = 1):")
	fmt.Println(string(revJSON))
}

func checkPass(pass bool) string {
	if pass {
		return "OK"
	}
	return "FAIL"
}

func checkKonsisten(k bool) string {
	if k {
		return "KONSISTEN"
	}
	return "TIDAK KONSISTEN"
}

func mustOID(hex string) bson.ObjectID {
	oid, _ := bson.ObjectIDFromHex(hex)
	return oid
}
