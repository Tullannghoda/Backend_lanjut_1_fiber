package model

import "time"

type Supervisor struct {
	ID              int       `json:"id"`
	Name            string    `json:"name"`
	Quota           int       `json:"quota"`
	SupervisedCount int       `json:"supervised_count"`
	Version         int       `json:"version"`
	CreatedAt       time.Time `json:"created_at"`
}

type Supervision struct {
	ID           int64     `json:"id"`
	StudentID    int       `json:"student_id"`
	SupervisorID int       `json:"supervisor_id"`
	CreatedAt    time.Time `json:"created_at"`
}

type AssignSupervisorRequest struct {
	StudentID int `json:"student_id" validate:"required,min=1"`
}

type SwitchSupervisorRequest struct {
	StudentID        int `json:"student_id" validate:"required,min=1"`
	FromSupervisorID int `json:"from_supervisor_id" validate:"required,min=1"`
	ToSupervisorID   int `json:"to_supervisor_id" validate:"required,min=1"`
}

type UpdateSupervisorQuotaRequest struct {
	Quota   int `json:"quota" validate:"required,min=1,max=500"`
	Version int `json:"version" validate:"required,min=1"`
}
