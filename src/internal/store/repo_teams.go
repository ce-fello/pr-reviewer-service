package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/ce-fello/pr-reviewer-service/src/internal/model"

	"go.uber.org/zap"
)

type TeamRepo struct {
	db  *sql.DB
	log *zap.Logger
}

func NewTeamRepo(db *sql.DB, logger *zap.Logger) *TeamRepo {
	return &TeamRepo{db: db, log: logger}
}

func (r *Repositories) CreateTeam(ctx context.Context, t model.Team) (model.Team, error) {
	r.Log.Debug("TeamRepo.CreateTeam: start", zap.String("team", t.TeamName))
	tx, err := r.Teams.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		r.Log.Error("TeamRepo.CreateTeam: begin tx failed", zap.Error(err))
		return model.Team{}, err
	}

	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			r.Log.Warn("TeamRepo.CreateTeam: rollback failed", zap.Error(err))
		}
	}()

	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM teams WHERE team_name=$1)`, t.TeamName).Scan(&exists); err != nil {
		r.Log.Error("TeamRepo.CreateTeam: check team exists failed", zap.Error(err))
		return model.Team{}, err
	}

	if exists {
		r.Log.Debug("TeamRepo.CreateTeam: team exists", zap.String("team", t.TeamName))
		return model.Team{}, model.ErrTeamExists
	}

	for _, m := range t.Members {
		var uexists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE user_id=$1)`, m.UserID).Scan(&uexists); err != nil {
			r.Log.Error("TeamRepo.CreateTeam: check user exists failed", zap.String("user", m.UserID), zap.Error(err))
			return model.Team{}, err
		}
		if uexists {
			r.Log.Debug("TeamRepo.CreateTeam: user_id conflict", zap.String("user", m.UserID))
			return model.Team{}, fmt.Errorf("user_id %s already exists", m.UserID)
		}
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO teams(team_name) VALUES($1)`, t.TeamName); err != nil {
		r.Log.Error("TeamRepo.CreateTeam: insert team failed", zap.Error(err))
		return model.Team{}, err
	}

	for _, m := range t.Members {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO users(user_id, username, team_name, is_active) VALUES($1,$2,$3,$4)`,
			m.UserID, m.Username, t.TeamName, m.IsActive); err != nil {
			r.Log.Error("TeamRepo.CreateTeam: insert user failed", zap.String("user", m.UserID), zap.Error(err))
			return model.Team{}, err
		}
		r.Log.Debug("TeamRepo.CreateTeam: inserted user", zap.String("user", m.UserID))
	}

	if err := tx.Commit(); err != nil {
		r.Log.Error("TeamRepo.CreateTeam: commit failed", zap.Error(err))
		return model.Team{}, err
	}

	r.Log.Info("TeamRepo.CreateTeam: success", zap.String("team", t.TeamName), zap.Int("members", len(t.Members)))
	return t, nil
}

func (r *Repositories) GetTeam(ctx context.Context, teamName string) (model.Team, error) {
	r.Log.Debug("TeamRepo.GetTeam: start", zap.String("team", teamName))
	var t model.Team
	t.TeamName = teamName

	rows, err := r.Teams.db.QueryContext(ctx, `SELECT user_id, username, is_active FROM users WHERE team_name=$1`, teamName)
	if err != nil {
		r.Log.Error("TeamRepo.GetTeam: query failed", zap.Error(err))
		return model.Team{}, err
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			r.Log.Error("TeamRepo.GetTeam: close rows failed", zap.Error(err))
		}
	}(rows)

	for rows.Next() {
		var m model.TeamMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.IsActive); err != nil {
			r.Log.Error("TeamRepo.GetTeam: scan failed", zap.Error(err))
			return model.Team{}, err
		}
		t.Members = append(t.Members, m)
	}

	if err := rows.Err(); err != nil {
		r.Log.Error("TeamRepo.GetTeam: rows error", zap.Error(err))
		return model.Team{}, err
	}

	if len(t.Members) == 0 {
		r.Log.Debug("TeamRepo.GetTeam: not found", zap.String("team", teamName))
		return model.Team{}, model.ErrNotFound
	}

	r.Log.Debug("TeamRepo.GetTeam: success", zap.String("team", teamName), zap.Int("members", len(t.Members)))
	return t, nil
}
