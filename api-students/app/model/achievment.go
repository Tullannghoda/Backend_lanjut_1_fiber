package model

import "time"

type Achievement struct {
	ID        int       `json:"id"`
	StudentID int       `json:"student_id"`
	Name      string    `json:"name"`       
	Rank      int       `json:"rank"`       
	CreatedAt time.Time `json:"created_at"`
}

type CreateAchievementRequest struct {
	StudentID int    `json:"student_id"`
	Name      string `json:"name"`
	Rank      int    `json:"rank"`
}

type PatchAchievementRequest struct {
	Name *string `json:"name,omitempty"`
	Rank *int    `json:"rank,omitempty"`
}