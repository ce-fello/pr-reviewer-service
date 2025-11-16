package store

import (
	"context"
	"database/sql"
	"errors"
	"github.com/ce-fello/pr-reviewer-service/src/internal/model"

	"go.uber.org/zap"
)

type UserRepo struct {
	db  *sql.DB
	log *zap.Logger
}

func NewUserRepo(db *sql.DB, logger *zap.Logger) *UserRepo {
	return &UserRepo{db: db, log: logger}
}

func (r *Repositories) SetUserIsActive(ctx context.Context, userID string, isActive bool) (model.User, error) {
	r.Log.Debug("SetUserIsActive: start", zap.String("user", userID), zap.Bool("is_active", isActive))
	res, err := r.DB.ExecContext(ctx, `UPDATE users SET is_active=$2 WHERE user_id=$1`, userID, isActive)
	if err != nil {
		r.Log.Error("SetUserIsActive: update failed", zap.Error(err))
		return model.User{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		r.Log.Debug("SetUserIsActive: user not found", zap.String("user", userID))
		return model.User{}, model.ErrNotFound
	}
	var u model.User
	if err := r.DB.QueryRowContext(ctx, `SELECT user_id, username, team_name, is_active FROM users WHERE user_id=$1`, userID).
		Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive); err != nil {
		r.Log.Error("SetUserIsActive: fetch user failed", zap.Error(err))
		return model.User{}, err
	}
	r.Log.Info("SetUserIsActive: success", zap.String("user", userID), zap.Bool("is_active", u.IsActive))
	return u, nil
}

func (r *Repositories) GetUser(ctx context.Context, userID string) (model.User, error) {
	r.Log.Debug("GetUser: start", zap.String("user", userID))
	var u model.User
	if err := r.DB.QueryRowContext(ctx, `SELECT user_id, username, team_name, is_active FROM users WHERE user_id=$1`, userID).
		Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.Log.Debug("GetUser: not found", zap.String("user", userID))
			return model.User{}, model.ErrNotFound
		}
		r.Log.Error("GetUser: query failed", zap.Error(err))
		return model.User{}, err
	}
	r.Log.Debug("GetUser: success", zap.String("user", userID))
	return u, nil
}

func (r *Repositories) GetActiveTeamMembersExcept(ctx context.Context, teamName string, excludeUserID string) ([]string, error) {
	r.Log.Debug("GetActiveTeamMembersExcept: start", zap.String("team", teamName), zap.String("exclude", excludeUserID))
	rows, err := r.DB.QueryContext(ctx, `SELECT user_id FROM users WHERE team_name=$1 AND is_active=true AND user_id <> $2`, teamName, excludeUserID)
	if err != nil {
		r.Log.Error("GetActiveTeamMembersExcept: query failed", zap.Error(err))
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			r.Log.Error("GetActiveTeamMembersExcept: close rows failed", zap.Error(err))
		}
	}(rows)
	var users []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			r.Log.Error("GetActiveTeamMembersExcept: scan failed", zap.Error(err))
			return nil, err
		}
		users = append(users, id)
	}
	r.Log.Debug("GetActiveTeamMembersExcept: success", zap.Int("count", len(users)))
	return users, nil
}
