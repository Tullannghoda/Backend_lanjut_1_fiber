package main 
  
import "time" 
  
type Student struct { 
    ID        int       `json:"id"` 
    Username  string    `json:"username"` 
    Email     string    `json:"email"` 
    Password  string    `json:"-"` 
    IsActive  bool      `json:"is_active"` 
    CreatedAt time.Time `json:"created_at"` 
} 
  
// POST — semua field wajib 
type CreateStudentRequest struct {
	NIM      string `json:"nim"`
	Name     string `json:"name"`
	Grade    string `json:"grade"`
	IsActive bool   `json:"is_active"`
}
  
// PUT — ganti seluruh isi, jadi field bertipe biasa dan semuanya wajib 
type ReplaceStudentRequest struct {
	NIM      string `json:"nim"`
	Name     string `json:"name"`
	Grade    string `json:"grade"`
	IsActive bool   `json:"is_active"`
}
  
// PATCH — ubah sebagian, jadi field bertipe pointer supaya bisa dibedakan 
// antara "tidak dikirim" (nil) dan "dikirim bernilai kosong" 
type PatchStudentRequest struct {
	NIM      *string `json:"nim,omitempty"`
	Name     *string `json:"name,omitempty"`
	Grade    *string `json:"grade,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}
  
// Amplop baku untuk semua respons 
type WebResponse struct { 
    Success bool   `json:"success"` 
    Message string `json:"message"` 
    Data    any    `json:"data,omitempty"` 
    Meta    *Meta  `json:"meta,omitempty"` 
    Errors  any    `json:"errors,omitempty"` 
} 
  
type Meta struct { 
    Page       int `json:"page"` 
    Limit      int `json:"limit"` 
    Total      int `json:"total"` 
    TotalPages int `json:"total_pages"` 
} 
  
type ListQuery struct { 
    Page     int 
    Limit    int 
    Search   string 
    Sort     string 
    Order    string 
    IsActive *bool 
} 