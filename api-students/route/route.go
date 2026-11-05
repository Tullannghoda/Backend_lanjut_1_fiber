package route

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"api-students/app/service"
	"api-students/helper"
	"api-students/middleware"
)

type Dependencies struct {
	Pool               *pgxpool.Pool
	MongoClient        *mongo.Client
	JWT                *helper.JWTManager
	Permissions        *helper.PermissionSet
	StudentService     *service.StudentService
	AchievementService *service.AchievementService
	UserService        *service.UserService
	AuthService        *service.AuthService
	CourseService      *service.CourseService
	SupervisorService  *service.SupervisorService
	ReviewService      *service.ReviewService
	PortfolioService   *service.PortfolioService
	NotificationService *service.NotificationService
}

func Register(app *fiber.App, deps Dependencies) {
	api := app.Group("/api/v1")

	api.Get("/health", healthCheck(deps.Pool, deps.MongoClient))

	auth := api.Group("/auth", middleware.RequireJSON)
	auth.Post("/register", deps.AuthService.Register)
	auth.Post("/login", middleware.LoginRateLimiter(), deps.AuthService.Login)
	auth.Post("/refresh", deps.AuthService.Refresh)
	auth.Post("/logout", deps.AuthService.Logout)
	auth.Get("/me", middleware.RequireAuth(deps.JWT), deps.AuthService.Me)

	perms := deps.Permissions

	users := api.Group("/users",
		middleware.RequireJSON,
		middleware.RequireAuth(deps.JWT))
	users.Get("/",
		middleware.RequirePermission(perms, "user:list"),
		deps.UserService.List)
	users.Post("/",
		middleware.RequirePermission(perms, "user:update:any"),
		deps.UserService.Create)
	users.Delete("/:id",
		middleware.RequirePermission(perms, "user:delete"),
		deps.UserService.Delete)
	users.Patch("/:id/role",
		middleware.RequirePermission(perms, "role:assign"),
		deps.UserService.AssignRole)
	users.Get("/:id", deps.UserService.Get)
	users.Put("/:id", deps.UserService.Replace)
	users.Patch("/:id", deps.UserService.Patch)

	students := api.Group("/students",
		middleware.RequireJSON,
		middleware.RequireAuth(deps.JWT))
	students.Get("/",
		middleware.RequirePermission(perms, "student:list"),
		deps.StudentService.List)
	students.Post("/",
		middleware.RequirePermission(perms, "student:create"),
		deps.StudentService.Create)
	students.Delete("/:id",
		middleware.RequirePermission(perms, "student:delete"),
		deps.StudentService.Delete)
	students.Get("/:id", deps.StudentService.Get)
	students.Put("/:id", deps.StudentService.Replace)
	students.Patch("/:id", deps.StudentService.Patch)

	// --- Portofolio Mahasiswa (Tugas Mandiri C) ---
	students.Get("/:id/portfolio", deps.PortfolioService.Get)
	students.Put("/:id/portfolio", deps.PortfolioService.CreateOrReplace)
	students.Post("/:id/portfolio/projects", deps.PortfolioService.AddProject)
	students.Delete("/:id/portfolio/projects/:projectId", deps.PortfolioService.DeleteProject)

	achievements := api.Group("/achievements",
		middleware.RequireJSON, middleware.RequireAuth(deps.JWT))
	achievements.Post("/", deps.AchievementService.Create)

	// --- kelas dan KRS ---
	courses := api.Group("/courses",
		middleware.RequireJSON,
		middleware.RequireAuth(deps.JWT))
	courses.Get("/", deps.CourseService.List)
	courses.Get("/mine", deps.CourseService.Mine)
	courses.Get("/:id", deps.CourseService.Get)
	courses.Post("/:id/enroll", deps.CourseService.Enroll)
	courses.Delete("/:id/enroll", deps.CourseService.Drop)
	courses.Patch("/:id", middleware.RequirePermission(perms, "course:manage"), deps.CourseService.UpdateQuota)

	// --- ulasan kelas pada courses ---
	courses.Get("/:id/reviews", deps.ReviewService.ListByCourse)
	courses.Get("/:id/reviews/summary", deps.ReviewService.Summary)
	courses.Post("/:id/reviews", deps.ReviewService.Create)

	// --- ulasan kelas (MongoDB) ---
	reviews := api.Group("/reviews",
		middleware.RequireJSON,
		middleware.RequireAuth(deps.JWT))
	reviews.Get("/:id", deps.ReviewService.Get)
	reviews.Patch("/:id", deps.ReviewService.Update)
	reviews.Delete("/:id", deps.ReviewService.Delete)
	reviews.Post("/:id/replies", deps.ReviewService.Reply)
	reviews.Post("/:id/helpful", deps.ReviewService.MarkHelpful)

	// --- statistik portofolio teknologi (MongoDB) ---
	api.Get("/portfolios/stats/technologies", deps.PortfolioService.TopTechnologies)

	enrollments := api.Group("/enrollments",
		middleware.RequireJSON,
		middleware.RequireAuth(deps.JWT))
	enrollments.Post("/swap", deps.CourseService.Swap)

	// --- Pembimbing Tugas Akhir (Tugas Mandiri D.2 Modul 9) ---
	supervisors := api.Group("/supervisors",
		middleware.RequireJSON,
		middleware.RequireAuth(deps.JWT))
	supervisors.Get("/", deps.SupervisorService.List)
	supervisors.Get("/:id", deps.SupervisorService.Get)
	supervisors.Post("/:id/assign",
		middleware.RequirePermission(perms, "supervision:manage"),
		deps.SupervisorService.Assign)
	supervisors.Patch("/:id",
		middleware.RequirePermission(perms, "supervision:manage"),
		deps.SupervisorService.UpdateQuota)

	supervisions := api.Group("/supervisions",
		middleware.RequireJSON,
		middleware.RequireAuth(deps.JWT))
	supervisions.Post("/switch",
		middleware.RequirePermission(perms, "supervision:manage"),
		deps.SupervisorService.Switch)

	// --- notifikasi (diisi oleh worker, dibaca lewat API) ---
	if deps.NotificationService != nil {
		notifications := api.Group("/notifications",
			middleware.RequireJSON,
			middleware.RequireAuth(deps.JWT))
		notifications.Get("/mine", deps.NotificationService.Mine)

		admin := api.Group("/admin",
			middleware.RequireJSON,
			middleware.RequireAuth(deps.JWT))
		admin.Get("/queue", middleware.RequirePermission(perms, "queue:monitor"), deps.NotificationService.Monitor)
	}
}

func healthCheck(pool *pgxpool.Pool, mongoClient *mongo.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			return helper.ServiceUnavailable("PostgreSQL tidak dapat dihubungi")
		}
		if mongoClient != nil {
			if err := mongoClient.Ping(ctx, nil); err != nil {
				return helper.ServiceUnavailable("MongoDB tidak dapat dihubungi")
			}
		}

		return helper.Success(c, fiber.StatusOK, "server, PostgreSQL, dan MongoDB berjalan", nil)
	}
}
