package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

func main() {
	// Buat instance aplikasi Fiber (mirip express() di Node.js)
	app := fiber.New()

	// Daftarkan route GET untuk path "/"
	// Ketika user buka http://localhost:3000/, function ini yang jalan
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	// Jalankan server di port 3000
	// log.Fatal = kalau ada error, cetak lalu program berhenti
	log.Fatal(app.Listen(":3000"))
}
