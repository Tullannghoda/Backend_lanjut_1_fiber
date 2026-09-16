package service

import (
	"errors"
	"api-students/app/model"
	
)

func ValidateCreateAchievement(req model.CreateAchievementRequest) error {
	if req.StudentID == 0 {
		return errors.New("id studenty gk boleh kosong")
	}
	if req.Name == "" {
		return errors.New("nama prestasi gk boleh kosong")
	}
	if req.Rank <= 0 {
		return errors.New("peringkat harus lebih besar dari 0")
	}
	return nil
}
