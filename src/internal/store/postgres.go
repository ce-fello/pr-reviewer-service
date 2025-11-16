package store

import (
	"context"
	"database/sql"
	"github.com/ce-fello/pr-reviewer-service/src/internal/model"

	"go.uber.org/zap"
)

type Repository interface {
	CreateTeam(ctx context.Context, t model.Team) (model.Team, error)
	GetTeam(ctx context.Context, teamName string) (model.Team, error)
	SetUserIsActive(ctx context.Context, userID string, isActive bool) (model.User, error)
	GetUser(ctx context.Context, userID string) (model.User, error)
	GetActiveTeamMembersExcept(ctx context.Context, teamName, excludeUserID string) ([]string, error)
	CreatePRWithReviewers(ctx context.Context, pr model.PullRequest) error
	GetPR(ctx context.Context, prID string) (model.PullRequest, error)
	UpdatePR(ctx context.Context, pr model.PullRequest) error
	GetAssignedPRsForUser(ctx context.Context, userID string) ([]model.PullRequestShort, error)
	GetReviewStats(ctx context.Context) (map[string]int, error)
	GetPRReviewStats(ctx context.Context) (map[string]int, error)
}

type Repositories struct {
	DB           *sql.DB
	Log          *zap.Logger
	Teams        *TeamRepo
	Users        *UserRepo
	PullRequests *PRRepo
}

func NewRepositories(db *sql.DB, logger *zap.Logger) *Repositories {
	teamRepo := NewTeamRepo(db, logger)
	userRepo := NewUserRepo(db, logger)
	prRepo := NewPRRepo(db, logger)

	return &Repositories{
		DB:           db,
		Log:          logger,
		Teams:        teamRepo,
		Users:        userRepo,
		PullRequests: prRepo,
	}
}

func (r *Repositories) BeginTx(ctx context.Context) (*sql.Tx, error) {
	r.Log.Debug("BeginTx called")
	return r.DB.BeginTx(ctx, &sql.TxOptions{})
}
