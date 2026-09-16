package service

import (
	"testing"

	"api-students/app/model"
)

func TestValidateRegister_WeakPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantKey  string
	}{
		{"terlalu pendek", "abc1", "password"},
		{"tanpa angka", "abcdefgh", "password"},
		{"tanpa huruf", "12345678", "password"},
		{"terlalu umum", "password123", "password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := model.RegisterRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: tt.password,
			}
			errs := ValidateRegister(req)

			if _, ok := errs[tt.wantKey]; !ok {
				t.Errorf("password %q seharusnya ditolak, tapi lolos", tt.password)
			}
		})
	}
}

func TestValidateRegister_StrongPassword(t *testing.T) {
	req := model.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "rahasia123",
	}
	errs := ValidateRegister(req)

	if msg, ok := errs["password"]; ok {
		t.Errorf("password kuat seharusnya diterima, tapi ditolak: %s", msg)
	}
}

func TestValidateRegister_InvalidUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
	}{
		{"kosong", ""},
		{"terlalu pendek", "ab"},
		{"karakter spesial", "user@name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := model.RegisterRequest{
				Username: tt.username,
				Email:    "test@example.com",
				Password: "rahasia123",
			}
			errs := ValidateRegister(req)

			if _, ok := errs["username"]; !ok {
				t.Errorf("username %q seharusnya ditolak, tapi lolos", tt.username)
			}
		})
	}
}

func TestValidateRegister_InvalidEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
	}{
		{"kosong", ""},
		{"tanpa @", "testexample.com"},
		{"tanpa domain", "test@"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := model.RegisterRequest{
				Username: "testuser",
				Email:    tt.email,
				Password: "rahasia123",
			}
			errs := ValidateRegister(req)

			if _, ok := errs["email"]; !ok {
				t.Errorf("email %q seharusnya ditolak, tapi lolos", tt.email)
			}
		})
	}
}

func TestValidateLogin_Empty(t *testing.T) {
	req := model.LoginRequest{Username: "", Password: ""}
	errs := ValidateLogin(req)

	if len(errs) != 2 {
		t.Errorf("seharusnya 2 error, tapi dapat %d: %v", len(errs), errs)
	}
}
