package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"api-students/app/model"
)

type AchievementRepository interface {
	Create(ctx context.Context, ach model.Achievement) (model.Achievement, error)
}

type achievementRepository struct {
	pool *pgxpool.Pool
}

func NewAchievementRepository(pool *pgxpool.Pool) AchievementRepository {
	return &achievementRepository{pool: pool}
}

func (r *achievementRepository) Create(ctx context.Context, ach model.Achievement) (model.Achievement, error) {
	query := `
		INSERT INTO achievements (student_id, name, rank)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`
	
	err := r.pool.QueryRow(ctx, query, ach.StudentID, ach.Name, ach.Rank).Scan(&ach.ID, &ach.CreatedAt)
	if err != nil {
		return ach, err
	}
	
	return ach, nil
}
