package service

import (
	"testing"
	"api-students/app/model"
)

// Test 1: menguji perhitungan total halaman
func TestCountTotalPages(t *testing.T) {
	cases := []struct{ total, limit, want int }{
		{0, 10, 0},
		{1, 10, 1},
		{10, 10, 1},
		{11, 10, 2},
		{137, 20, 7},
	}

	for _, tc := range cases {
		if got := CountTotalPages(tc.total, tc.limit); got != tc.want {
			t.Errorf("total=%d limit=%d: harap %d, dapat %d",
				tc.total, tc.limit, tc.want, got)
		}
	}
}

// Test 2: menguji validasi saat membuat data kosong
func TestValidateCreate_Empty(t *testing.T) {
	req := model.CreateStudentRequest{
		NIM:  "   ", // spasi kan aaa
		Name: "",
	}

	errs := ValidateCreate(req)
	
	if len(errs) != 2 {
		t.Errorf("seharusnya ada 2 error, tapi dapat %d", len(errs))
	}
	if errs["nim"] == "" {
		t.Error("error nim seharusnya muncul")
	}
	if errs["name"] == "" {
		t.Error("error name seharusnya muncul")
	}
}

// Test 3: menguji mekanisme ApplyPatch
func TestApplyPatch(t *testing.T) {
	initial := model.Student{ID: 1, NIM: "123", Name: "Budi", Grade: "A", IsActive: true}
	
	inactive := false
	result, errs := ApplyPatch(initial, model.PatchStudentRequest{IsActive: &inactive})

	if len(errs) != 0 {
		t.Fatalf("tidak seharusnya ada error: %v", errs)
	}
	if result.IsActive {
		t.Error("is_active seharusnya berubah menjadi false")
	}
	if result.Name != "Budi" {
		t.Error("field yang tidak dikirim seharusnya tidak berubah (tetap Budi)")
	}
}