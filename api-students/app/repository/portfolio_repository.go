package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"api-students/app/model"
)

var (
	ErrProjectLimit = errors.New("jumlah proyek sudah mencapai batas maksimal 20")
)

const MaxProjectsPerPortfolio = 20

type PortfolioRepository interface {
	EnsureIndexes(ctx context.Context) error
	GetByStudentID(ctx context.Context, studentID int) (model.Portfolio, error)
	CreateOrReplace(ctx context.Context, p model.Portfolio) (model.Portfolio, error)
	AddProject(ctx context.Context, studentID int, proj model.Project) (model.Portfolio, error)
	DeleteProject(ctx context.Context, studentID int, projectID string) (model.Portfolio, error)
	TopTechnologies(ctx context.Context, limit int) ([]model.TechnologyStat, error)
}

type portfolioDocument struct {
	ID           bson.ObjectID         `bson:"_id,omitempty"`
	StudentID    int                   `bson:"student_id"`
	Headline     string                `bson:"headline"`
	Bio          string                `bson:"bio"`
	Projects     []projectDocument     `bson:"projects"`
	Certificates []certificateDocument `bson:"certificates"`
	CreatedAt    time.Time             `bson:"created_at"`
	UpdatedAt    time.Time             `bson:"updated_at"`
}

type projectDocument struct {
	ID           bson.ObjectID `bson:"_id"`
	Title        string        `bson:"title"`
	Technologies []string      `bson:"technologies"`
	Link         string        `bson:"link"`
	Year         int           `bson:"year"`
}

type certificateDocument struct {
	ID     bson.ObjectID `bson:"_id"`
	Name   string        `bson:"name"`
	Issuer string        `bson:"issuer"`
	Date   string        `bson:"date"`
}

func (d portfolioDocument) toModel() model.Portfolio {
	projects := make([]model.Project, 0, len(d.Projects))
	for _, p := range d.Projects {
		techs := p.Technologies
		if techs == nil {
			techs = []string{}
		}
		projects = append(projects, model.Project{
			ID:           p.ID.Hex(),
			Title:        p.Title,
			Technologies: techs,
			Link:         p.Link,
			Year:         p.Year,
		})
	}

	certs := make([]model.Certificate, 0, len(d.Certificates))
	for _, c := range d.Certificates {
		certs = append(certs, model.Certificate{
			ID:     c.ID.Hex(),
			Name:   c.Name,
			Issuer: c.Issuer,
			Date:   c.Date,
		})
	}

	return model.Portfolio{
		ID:           d.ID.Hex(),
		StudentID:    d.StudentID,
		Headline:     d.Headline,
		Bio:          d.Bio,
		Projects:     projects,
		Certificates: certs,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
}

type portfolioMongoRepository struct {
	coll *mongo.Collection
}

func NewPortfolioRepository(db *mongo.Database) PortfolioRepository {
	return &portfolioMongoRepository{coll: db.Collection("student_portfolios")}
}

func (r *portfolioMongoRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "student_id", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("student_portfolio_unique"),
		},
		{
			Keys:    bson.D{{Key: "projects.technologies", Value: 1}},
			Options: options.Index().SetName("projects_technologies_idx"),
		},
	})
	if err != nil {
		return fmt.Errorf("membuat index student_portfolios: %w", err)
	}
	return nil
}

func (r *portfolioMongoRepository) GetByStudentID(ctx context.Context, studentID int) (model.Portfolio, error) {
	var doc portfolioDocument
	if err := r.coll.FindOne(ctx, bson.M{"student_id": studentID}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return model.Portfolio{}, ErrNotFound
		}
		return model.Portfolio{}, fmt.Errorf("mengambil portofolio: %w", err)
	}
	return doc.toModel(), nil
}

func (r *portfolioMongoRepository) CreateOrReplace(ctx context.Context, p model.Portfolio) (model.Portfolio, error) {
	t := now()

	pDocs := make([]projectDocument, 0, len(p.Projects))
	for _, pr := range p.Projects {
		oid := bson.NewObjectID()
		if pr.ID != "" {
			if parsed, err := parseID(pr.ID); err == nil {
				oid = parsed
			}
		}
		techs := pr.Technologies
		if techs == nil {
			techs = []string{}
		}
		pDocs = append(pDocs, projectDocument{
			ID:           oid,
			Title:        pr.Title,
			Technologies: techs,
			Link:         pr.Link,
			Year:         pr.Year,
		})
	}

	cDocs := make([]certificateDocument, 0, len(p.Certificates))
	for _, cr := range p.Certificates {
		oid := bson.NewObjectID()
		if cr.ID != "" {
			if parsed, err := parseID(cr.ID); err == nil {
				oid = parsed
			}
		}
		cDocs = append(cDocs, certificateDocument{
			ID:     oid,
			Name:   cr.Name,
			Issuer: cr.Issuer,
			Date:   cr.Date,
		})
	}

	update := bson.M{
		"$set": bson.M{
			"headline":     p.Headline,
			"bio":          p.Bio,
			"projects":     pDocs,
			"certificates": cDocs,
			"updated_at":   t,
		},
		"$setOnInsert": bson.M{
			"created_at": t,
		},
	}

	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var doc portfolioDocument
	err := r.coll.FindOneAndUpdate(ctx, bson.M{"student_id": p.StudentID}, update, opts).Decode(&doc)
	if err != nil {
		return model.Portfolio{}, fmt.Errorf("menyimpan portofolio: %w", err)
	}
	return doc.toModel(), nil
}

func (r *portfolioMongoRepository) AddProject(ctx context.Context, studentID int, proj model.Project) (model.Portfolio, error) {
	docProj := projectDocument{
		ID:           bson.NewObjectID(),
		Title:        proj.Title,
		Technologies: proj.Technologies,
		Link:         proj.Link,
		Year:         proj.Year,
	}
	if docProj.Technologies == nil {
		docProj.Technologies = []string{}
	}

	limitField := "projects." + strconv.Itoa(MaxProjectsPerPortfolio-1)

	filter := bson.M{
		"student_id": studentID,
		limitField:   bson.M{"$exists": false},
	}
	update := bson.M{
		"$push": bson.M{"projects": docProj},
		"$set":  bson.M{"updated_at": now()},
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated portfolioDocument
	err := r.coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updated)
	if err == nil {
		return updated.toModel(), nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return model.Portfolio{}, fmt.Errorf("menambah proyek portofolio: %w", err)
	}

	count, err := r.coll.CountDocuments(ctx, bson.M{"student_id": studentID})
	if err != nil {
		return model.Portfolio{}, fmt.Errorf("memeriksa portofolio: %w", err)
	}
	if count == 0 {
		return model.Portfolio{}, ErrNotFound
	}
	return model.Portfolio{}, ErrProjectLimit
}

func (r *portfolioMongoRepository) DeleteProject(ctx context.Context, studentID int, projectID string) (model.Portfolio, error) {
	oid, err := parseID(projectID)
	if err != nil {
		return model.Portfolio{}, err
	}

	filter := bson.M{"student_id": studentID}
	update := bson.M{
		"$pull": bson.M{"projects": bson.M{"_id": oid}},
		"$set":  bson.M{"updated_at": now()},
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated portfolioDocument
	err = r.coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updated)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return model.Portfolio{}, ErrNotFound
		}
		return model.Portfolio{}, fmt.Errorf("menghapus proyek: %w", err)
	}
	return updated.toModel(), nil
}

func (r *portfolioMongoRepository) TopTechnologies(ctx context.Context, limit int) ([]model.TechnologyStat, error) {
	if limit <= 0 {
		limit = 10
	}

	pipeline := mongo.Pipeline{
		{{Key: "$unwind", Value: "$projects"}},
		{{Key: "$unwind", Value: "$projects.technologies"}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$projects.technologies",
			"total": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.D{
			{Key: "total", Value: -1},
			{Key: "_id", Value: 1},
		}}},
		{{Key: "$limit", Value: limit}},
	}

	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("agregasi teknologi: %w", err)
	}
	defer cursor.Close(ctx)

	var out []struct {
		Tech  string `bson:"_id"`
		Total int    `bson:"total"`
	}
	if err := cursor.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("membaca hasil agregasi: %w", err)
	}

	stats := make([]model.TechnologyStat, 0, len(out))
	for _, row := range out {
		stats = append(stats, model.TechnologyStat{
			Technology: row.Tech,
			Total:      row.Total,
		})
	}
	return stats, nil
}
