package main

import "fmt"

// FUNCTION-FUNCTION BUAT POINTER

// function ini tukarkan dua angka, kita kirim alamatnya
// biar yang asli yang ketuker, bukan fotokopinya
func swap(a, b *int) {
	*a, *b = *b, *a
}

// function ini nambah item baru ke dalam slice
// harus pakai pointer karena append() kadang bikin slice baru
// jadi kalo ga pakai pointer, slice aslinya ga ikut berubah
func updateSlice(s *[]string, newItem string) {
	*s = append(*s, newItem)
}

// function ini terima FOTOKOPI nilainya
// jadi mau diubah apapun di dalam sini, yang asli tetap aman
func ubahNilai(x int) {
	x = 999
}

// function ini terima ALAMAT nilainya
// jadi kita langsung ubah yang asli
func ubahLewatPointer(x *int) {
	*x = 999
}

// STRUCT STUDENT 
// di Go ga ada class, tapi pakai struct

type Student struct {
	ID       int     // nomor unik mahasiswa
	Name     string  // nama mahasiswa
	Grade    float64 // nilai mahasiswa
	IsActive bool    // statusnya aktif atau engga
}

// GetInfo — nampilin info lengkap student dalam satu baris
func (s Student) GetInfo() string {
	return fmt.Sprintf("ID: %d | Nama: %s | Nilai: %.2f | Aktif: %v", s.ID, s.Name, s.Grade, s.IsActive)
}

// UpdateGrade — ganti nilai student
func (s *Student) UpdateGrade(grade float64) {
	s.Grade = grade
}

// Activate — aktifkan student
func (s *Student) Activate() {
	s.IsActive = true
}

// Deactivate — nonaktifkan student
func (s *Student) Deactivate() {
	s.IsActive = false
}

func main() {
	// BAGIAN 1: Kenalan sama variabel di Go

	// cara pertama: tulis lengkap tipenya
	var nama string = "Glennovian"
	var umur int = 20
	var ipk float64 = 3.75

	// cara kedua: pake := biar Go yang tebak tipenya sendiri
	aktif := true
	hobi := []string{"coding", "gaming", "membaca"}

	fmt.Println(" DATA MAHASISWA ")
	fmt.Println("Nama    :", nama)
	fmt.Println("Umur    :", umur)
	fmt.Println("IPK     :", ipk)
	fmt.Println("Aktif   :", aktif)
	fmt.Println("Hobi    :", hobi)

	// BAGIAN 2: Map — kayak kamus, ada kata & artinya

	// bikin map kosong: kuncinya string (nama), isinya int (nilai)
	nilaiMahasiswa := make(map[string]int)

	// masukin data satu-satu
	nilaiMahasiswa["Glennovian"] = 90
	nilaiMahasiswa["Budi"] = 85
	nilaiMahasiswa["Sari"] = 92
	nilaiMahasiswa["Dewi"] = 78

	fmt.Println("\n NILAI MAHASISWA ")
	fmt.Println(nilaiMahasiswa)

	// cari nilai seseorang, sekalian cek orangnya ada atau engga
	nilai, ada := nilaiMahasiswa["Glennovian"]
	if ada {
		fmt.Println("\nNilai Glennovian:", nilai)
	} else {
		fmt.Println("\nGlennovian tidak ditemukan")
	}

	// coba cari orang yang ga ada di map
	nilai2, ada2 := nilaiMahasiswa["Andi"]
	if ada2 {
		fmt.Println("Nilai Andi:", nilai2)
	} else {
		fmt.Println("Andi tidak ditemukan")
	}

	// hapus data dari map
	fmt.Println("\nSebelum hapus Dewi:", nilaiMahasiswa)
	delete(nilaiMahasiswa, "Dewi")
	fmt.Println("Sesudah hapus Dewi:", nilaiMahasiswa)

	// jalan-jalan ke semua isi map satu per satu
	fmt.Println("\n DAFTAR LENGKAP ")
	for nama, nilai := range nilaiMahasiswa {
		fmt.Printf("  %s: %d\n", nama, nilai)
	}

	// BAGIAN 3: Pointer — kirim alamat, bukan fotokopi

	// contoh 1: tukar dua angka pakai pointer
	fmt.Println("\n POINTER: SWAP ")
	a, b := 10, 20
	fmt.Println("Sebelum swap: a =", a, ", b =", b)
	swap(&a, &b) // kita kirim alamat a dan b, bukan nilainya
	fmt.Println("Sesudah swap: a =", a, ", b =", b)

	// contoh 2: tambah item ke slice pakai pointer
	fmt.Println("\n POINTER: UPDATE SLICE ")
	daftarNama := []string{"Glennovian", "Budi"}
	fmt.Println("Sebelum update:", daftarNama)
	updateSlice(&daftarNama, "Sari") // kirim alamat slice-nya
	fmt.Println("Sesudah update:", daftarNama)

	// contoh 3: bedanya kirim fotokopi vs kirim alamat
	fmt.Println("\n PERBANDINGAN: FOTOKOPI vs ALAMAT ASLI ")
	angka := 42
	fmt.Println("Nilai awal:", angka)

	ubahNilai(angka) // kirim fotokopi, yang asli ga keubah
	fmt.Println("Setelah ubahNilai (fotokopi):", angka, "-> tidak berubah!")

	ubahLewatPointer(&angka) // kirim alamat, yang asli keubah
	fmt.Println("Setelah ubahLewatPointer (alamat):", angka, "-> berubah!")

	// BAGIAN 4: Struct Student — bikin dan pakai "cetakan" data

	fmt.Println("\n STRUCT STUDENT ")

	// bikin student baru dari struct, kayak isi formulir
	student1 := Student{
		ID:   1,
		Name: "Glennovian",
	}
	// Grade dan IsActive ga diisi, otomatis jadi 0 dan false (zero value)

	fmt.Println("Baru dibuat:", student1.GetInfo())

	// aktifkan student pakai method Activate
	student1.Activate()
	fmt.Println("Setelah Activate:", student1.GetInfo())

	// update nilai pakai method UpdateGrade
	student1.UpdateGrade(92.5)
	fmt.Println("Setelah UpdateGrade:", student1.GetInfo())

	// nonaktifkan student
	student1.Deactivate()
	fmt.Println("Setelah Deactivate:", student1.GetInfo())

	// bikin student kedua buat buktiin mereka independent
	student2 := Student{ID: 2, Name: "Budi", Grade: 80.0, IsActive: true}
	fmt.Println("\nStudent 2:", student2.GetInfo())
}
