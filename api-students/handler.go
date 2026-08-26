package main

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

var users []User
var nextID = 1

func findUserIndex(id int) int {
	for i := range users {
		if users[i].ID == id {
			return i
		}
	}
	return -1
}


func paramID(c *fiber.Ctx) (int, bool) {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return 0, false
	}
	return id, true
}

// cocokPencarian memeriksa apakah kata kunci muncul di username atau email.
func cocokPencarian(s Student, kata string) bool {
	kata = strings.ToLower(kata)
	return strings.Contains(strings.ToLower(s.Name), kata) 
}

func getStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}
	i := findStudentIndex(id)
	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan") // 404 Not Found
	}
	return ok(c, "mahasiswa ditemukan", students[i])
}


func createStudent(c *fiber.Ctx) error {
	var req CreateStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid") // 400 Bad Request
	}
	errs := map[string]string{}
	req.NIM = strings.TrimSpace(req.NIM)
	req.Name = strings.TrimSpace(req.Name)
	req.Grade = strings.TrimSpace(req.Grade)
	// Validasi dasar
	if req.NIM == "" {
		errs["nim"] = "wajib diisi!"
	}
	if req.Name == "" {
		errs["name"] = "wajib diisi!"
	}
	// Cek duplikasi NIM (Syarat Tugas: Status 409 Conflict jika NIM ganda)
	for _, s := range students {
		if strings.EqualFold(s.NIM, req.NIM) {
			errs["nim"] = "NIM sudah dipakai"
		}
	}
	// Jika ada error validasi, kembalikan 422 Unprocessable Entity
	if len(errs) > 0 {
		return failValidation(c, errs)
	}
	// Buat objek mahasiswa baru
	baru := Student{
		ID:        nextID,
		NIM:       req.NIM,
		Name:      req.Name,
		Grade:     req.Grade,
		IsActive:  req.IsActive,
		CreatedAt: time.Now(),
	}
	students = append(students, baru)
	nextID++
	// 201 Created disertai header Location
	return created(c, "mahasiswa berhasil ditambahkan", baru, "/api/v1/students/"+strconv.Itoa(baru.ID))
}

func listStudents(c *fiber.Ctx) error {
	q := parseListQuery(c)
	// 1) Saring (Filter) berdasarkan active & nama
	hasil := []Student{}
	for _, s := range students {
		if q.IsActive != nil && s.IsActive != *q.IsActive {
			continue
		}
		if q.Search != "" && !cocokPencarian(s, q.Search) {
			continue
		}
		hasil = append(hasil, s)
	}
	// 2) Urutkan (Sort & Order)
	sort.SliceStable(hasil, func(i, j int) bool {
		var lebihKecil bool
		switch q.Sort {
		case "nim":
			lebihKecil = hasil[i].NIM < hasil[j].NIM
		case "name":
			lebihKecil = hasil[i].Name < hasil[j].Name
		case "grade":
			lebihKecil = hasil[i].Grade < hasil[j].Grade
		case "created_at":
			lebihKecil = hasil[i].CreatedAt.Before(hasil[j].CreatedAt)
		default: // "id"
			lebihKecil = hasil[i].ID < hasil[j].ID
		}
		if q.Order == "desc" {
			return !lebihKecil
		}
		return lebihKecil
	})
	// 3) Potong halaman (Pagination)
	total := len(hasil)
	totalPages := (total + q.Limit - 1) / q.Limit
	if totalPages == 0 {
		totalPages = 1
	}
	mulai := (q.Page - 1) * q.Limit
	if mulai > total {
		mulai = total
	}
	akhir := mulai + q.Limit
	if akhir > total {
		akhir = total
	}
	return okList(c, "daftar mahasiswa berhasil diambil", hasil[mulai:akhir], &Meta{
		Page: q.Page, Limit: q.Limit, Total: total, TotalPages: totalPages,
	})
}

func replaceStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}
	i := findStudentIndex(id)
	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}
	var req ReplaceStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}
	errs := map[string]string{}
	if strings.TrimSpace(req.NIM) == "" {
		errs["nim"] = "wajib diisi pada PUT"
	}
	if strings.TrimSpace(req.Name) == "" {
		errs["name"] = "wajib diisi pada PUT"
	}
	if len(errs) > 0 {
		return failValidation(c, errs)
	}
	students[i].NIM = req.NIM
	students[i].Name = req.Name
	students[i].Grade = req.Grade
	students[i].IsActive = req.IsActive
	return ok(c, "mahasiswa berhasil diganti seluruhnya", students[i])
}

func patchStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}
	i := findStudentIndex(id)
	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}
	var req PatchStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body JSON tidak valid") // Status 400
	}
	if req.NIM == nil && req.Name == nil && req.Grade == nil && req.IsActive == nil {
		return fail(c, fiber.StatusBadRequest, "tidak ada field yang diubah")
	}
	if req.NIM != nil {
		if strings.TrimSpace(*req.NIM) == "" {
			return failValidation(c, map[string]string{"nim": "tidak boleh kosong"}) // Status 422
		}
		students[i].NIM = *req.NIM
	}
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return failValidation(c, map[string]string{"name": "tidak boleh kosong"})
		}
		students[i].Name = *req.Name
	}
	if req.Grade != nil {
		students[i].Grade = *req.Grade
	}
	if req.IsActive != nil {
		students[i].IsActive = *req.IsActive
	}
	return ok(c, "mahasiswa berhasil diperbarui sebagian", students[i])
}

func deleteStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}
	i := findStudentIndex(id)
	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}
	students = append(students[:i], students[i+1:]...)
	return noContent(c) // Mengembalikan Status 204
}