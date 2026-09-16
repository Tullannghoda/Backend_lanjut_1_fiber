package service

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"
)

type AchievementService struct {
	repo repository.AchievementRepository
}

func NewAchievementService(repo repository.AchievementRepository) *AchievementService {
	return &AchievementService{repo: repo}
}

func (s *AchievementService) Create(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	var req model.CreateAchievementRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}

	if err := ValidateCreateAchievement(req); err != nil {
		return helper.Fail(c, fiber.StatusUnprocessableEntity, err.Error())
	}

	newAchievement, err := s.repo.Create(ctx, model.Achievement{
		StudentID: req.StudentID,
		Name:      strings.TrimSpace(req.Name),
		Rank:      req.Rank,
	})
	if err != nil {
		return translateError(c, err, "gagal menyimpan prestasi")
	}

	return helper.Created(c, "prestasi berhasil dibuat", newAchievement, "/api/v1/achievements/"+strconv.Itoa(newAchievement.ID))
}

