package service

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"
)

type PortfolioService struct {
	portfolios repository.PortfolioRepository
	students   repository.StudentRepository
	perms      *helper.PermissionSet
}

func NewPortfolioService(
	portfolios repository.PortfolioRepository,
	students repository.StudentRepository,
	perms *helper.PermissionSet,
) *PortfolioService {
	return &PortfolioService{portfolios: portfolios, students: students, perms: perms}
}

// GET /students/:id/portfolio
func (s *PortfolioService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	studentID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id mahasiswa harus berupa angka positif")
	}

	if _, err := s.students.FindByID(ctx, studentID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("mahasiswa tidak ditemukan di PostgreSQL")
		}
		return helper.Internal(err)
	}

	portfolio, err := s.portfolios.GetByStudentID(ctx, studentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("portofolio belum dibuat untuk mahasiswa ini")
		}
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "portofolio berhasil diambil", portfolio)
}

// PUT /students/:id/portfolio
func (s *PortfolioService) CreateOrReplace(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	studentID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id mahasiswa harus berupa angka positif")
	}

	if _, err := s.students.FindByID(ctx, studentID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("mahasiswa tidak ditemukan di PostgreSQL")
		}
		return helper.Internal(err)
	}

	var req model.CreateOrUpdatePortfolioRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Headline = strings.TrimSpace(req.Headline)
	req.Bio = strings.TrimSpace(req.Bio)

	for i := range req.Projects {
		req.Projects[i].Title = strings.TrimSpace(req.Projects[i].Title)
		req.Projects[i].Technologies = NormalizeTags(req.Projects[i].Technologies)
	}
	for i := range req.Certificates {
		req.Certificates[i].Name = strings.TrimSpace(req.Certificates[i].Name)
		req.Certificates[i].Issuer = strings.TrimSpace(req.Certificates[i].Issuer)
	}

	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	projects := make([]model.Project, 0, len(req.Projects))
	for _, pr := range req.Projects {
		projects = append(projects, model.Project{
			Title:        pr.Title,
			Technologies: pr.Technologies,
			Link:         pr.Link,
			Year:         pr.Year,
		})
	}

	certs := make([]model.Certificate, 0, len(req.Certificates))
	for _, cr := range req.Certificates {
		certs = append(certs, model.Certificate{
			Name:   cr.Name,
			Issuer: cr.Issuer,
			Date:   cr.Date,
		})
	}

	portfolio := model.Portfolio{
		StudentID:    studentID,
		Headline:     req.Headline,
		Bio:          req.Bio,
		Projects:     projects,
		Certificates: certs,
	}

	saved, err := s.portfolios.CreateOrReplace(ctx, portfolio)
	if err != nil {
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "portofolio berhasil disimpan", saved)
}

// POST /students/:id/portfolio/projects
func (s *PortfolioService) AddProject(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	studentID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id mahasiswa harus berupa angka positif")
	}

	if _, err := s.students.FindByID(ctx, studentID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("mahasiswa tidak ditemukan di PostgreSQL")
		}
		return helper.Internal(err)
	}

	var req model.AddProjectRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Technologies = NormalizeTags(req.Technologies)

	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	updated, err := s.portfolios.AddProject(ctx, studentID, model.Project{
		Title:        req.Title,
		Technologies: req.Technologies,
		Link:         req.Link,
		Year:         req.Year,
	})
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("portofolio belum dibuat untuk mahasiswa ini")
		}
		if errors.Is(err, repository.ErrProjectLimit) {
			return helper.NewError(fiber.StatusConflict, "PROJECT_LIMIT", "jumlah proyek sudah mencapai batas maksimal 20")
		}
		return helper.Internal(err)
	}

	return helper.Created(c, "proyek berhasil ditambahkan ke portofolio", updated, "/api/v1/students/"+c.Params("id")+"/portfolio")
}

// DELETE /students/:id/portfolio/projects/:projectId
func (s *PortfolioService) DeleteProject(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	studentID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id mahasiswa harus berupa angka positif")
	}

	projectID := c.Params("projectId")
	if projectID == "" {
		return helper.BadRequest("id proyek tidak boleh kosong")
	}

	if _, err := s.students.FindByID(ctx, studentID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("mahasiswa tidak ditemukan di PostgreSQL")
		}
		return helper.Internal(err)
	}

	updated, err := s.portfolios.DeleteProject(ctx, studentID, projectID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("proyek atau portofolio tidak ditemukan")
		}
		if errors.Is(err, repository.ErrInvalidID) {
			return helper.BadRequest("format id proyek tidak valid")
		}
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "proyek berhasil dihapus dari portofolio", updated)
}

// GET /portfolios/stats/technologies
func (s *PortfolioService) TopTechnologies(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	limit := c.QueryInt("limit", 10)
	if limit < 1 || limit > 50 {
		limit = 10
	}

	stats, err := s.portfolios.TopTechnologies(ctx, limit)
	if err != nil {
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "statistik teknologi berhasil diambil", stats)
}
