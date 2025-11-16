package store

import (
	"context"
	"database/sql"
	"errors"
	"github.com/ce-fello/pr-reviewer-service/src/internal/model"
	"time"

	"go.uber.org/zap"
)

type PRRepo struct {
	db  *sql.DB
	log *zap.Logger
}

func NewPRRepo(db *sql.DB, logger *zap.Logger) *PRRepo {
	return &PRRepo{db: db, log: logger}
}

func (r *Repositories) CreatePRWithReviewers(ctx context.Context, pr model.PullRequest) error {
	r.Log.Debug("CreatePRWithReviewers: start", zap.String("pr_id", pr.PullRequestID), zap.String("author", pr.AuthorID))

	tx, err := r.BeginTx(ctx)
	if err != nil {
		r.Log.Error("CreatePRWithReviewers: begin tx failed", zap.Error(err))
		return err
	}

	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			r.Log.Warn("CreatePRWithReviewers: rollback failed", zap.Error(err))
		}
	}()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO pull_requests(pull_request_id, pull_request_name, author_id, status, created_at) VALUES($1,$2,$3,'OPEN', now())`,
		pr.PullRequestID, pr.PullRequestName, pr.AuthorID)
	if err != nil {
		r.Log.Error("CreatePRWithReviewers: insert pull_requests failed", zap.String("pr_id", pr.PullRequestID), zap.Error(err))
		return err
	}

	for _, u := range pr.Assigned {
		if _, err := tx.ExecContext(ctx, `INSERT INTO pr_reviewers(pull_request_id, user_id) VALUES($1,$2)`, pr.PullRequestID, u); err != nil {
			r.Log.Error("CreatePRWithReviewers: insert pr_reviewers failed", zap.String("pr_id", pr.PullRequestID), zap.String("user", u), zap.Error(err))
			return err
		}
		r.Log.Debug("CreatePRWithReviewers: inserted reviewer", zap.String("pr_id", pr.PullRequestID), zap.String("reviewer", u))
	}

	if err := tx.Commit(); err != nil {
		r.Log.Error("CreatePRWithReviewers: commit failed", zap.String("pr_id", pr.PullRequestID), zap.Error(err))
		return err
	}

	r.Log.Info("CreatePRWithReviewers: success", zap.String("pr_id", pr.PullRequestID), zap.Int("reviewers", len(pr.Assigned)))
	return nil
}

func (r *Repositories) GetPR(ctx context.Context, prID string) (model.PullRequest, error) {
	r.Log.Debug("GetPR: start", zap.String("pr_id", prID))
	var p model.PullRequest
	var mergedAt sql.NullTime
	if err := r.DB.QueryRowContext(ctx, `SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at FROM pull_requests WHERE pull_request_id=$1`, prID).
		Scan(&p.PullRequestID, &p.PullRequestName, &p.AuthorID, &p.Status, &p.CreatedAt, &mergedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.Log.Debug("GetPR: not found", zap.String("pr_id", prID))
			return model.PullRequest{}, model.ErrNotFound
		}
		r.Log.Error("GetPR: query failed", zap.String("pr_id", prID), zap.Error(err))
		return model.PullRequest{}, err
	}

	if mergedAt.Valid {
		t := mergedAt.Time
		p.MergedAt = &t
	}

	rows, err := r.DB.QueryContext(ctx, `SELECT user_id FROM pr_reviewers WHERE pull_request_id=$1 ORDER BY user_id`, prID)
	if err != nil {
		r.Log.Error("GetPR: query reviewers failed", zap.String("pr_id", prID), zap.Error(err))
		return model.PullRequest{}, err
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			r.Log.Error("GetPR: close rows failed", zap.String("pr_id", prID), zap.Error(err))
		}
	}(rows)

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			r.Log.Error("GetPR: scan reviewer failed", zap.String("pr_id", prID), zap.Error(err))
			return model.PullRequest{}, err
		}
		p.Assigned = append(p.Assigned, id)
	}

	r.Log.Debug("GetPR: success", zap.String("pr_id", prID), zap.Int("reviewer_count", len(p.Assigned)))
	return p, nil
}

func (r *Repositories) GetPRForUpdate(ctx context.Context, tx *sql.Tx, prID string) (model.PullRequest, error) {
	r.Log.Debug("GetPRForUpdate: start", zap.String("pr_id", prID))
	var p model.PullRequest
	var mergedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at FROM pull_requests WHERE pull_request_id=$1 FOR UPDATE`, prID).
		Scan(&p.PullRequestID, &p.PullRequestName, &p.AuthorID, &p.Status, &p.CreatedAt, &mergedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.Log.Debug("GetPRForUpdate: not found", zap.String("pr_id", prID))
			return model.PullRequest{}, model.ErrNotFound
		}
		r.Log.Error("GetPRForUpdate: select for update failed", zap.String("pr_id", prID), zap.Error(err))
		return model.PullRequest{}, err
	}

	if mergedAt.Valid {
		t := mergedAt.Time
		p.MergedAt = &t
	}

	rows, err := tx.QueryContext(ctx, `SELECT user_id FROM pr_reviewers WHERE pull_request_id=$1 ORDER BY user_id`, prID)
	if err != nil {
		r.Log.Error("GetPRForUpdate: query reviewers failed", zap.String("pr_id", prID), zap.Error(err))
		return model.PullRequest{}, err
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			r.Log.Error("GetPRForUpdate: close rows failed", zap.Error(err))
		}
	}(rows)

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			r.Log.Error("GetPRForUpdate: scan reviewer failed", zap.String("pr_id", prID), zap.Error(err))
			return model.PullRequest{}, err
		}
		p.Assigned = append(p.Assigned, id)
	}

	r.Log.Debug("GetPRForUpdate: success", zap.String("pr_id", prID), zap.Int("reviewer_count", len(p.Assigned)))
	return p, nil
}

func (r *Repositories) SetPRMerged(ctx context.Context, tx *sql.Tx, prID string, mergedAt time.Time) error {
	r.Log.Debug("SetPRMerged: start", zap.String("pr_id", prID))
	_, err := tx.ExecContext(ctx, `UPDATE pull_requests SET status='MERGED', merged_at=$2 WHERE pull_request_id=$1`, prID, mergedAt)
	if err != nil {
		r.Log.Error("SetPRMerged: update failed", zap.String("pr_id", prID), zap.Error(err))
	}
	return err
}

func (r *Repositories) IsReviewerAssigned(ctx context.Context, tx *sql.Tx, prID, userID string) (bool, error) {
	r.Log.Debug("IsReviewerAssigned: check", zap.String("pr_id", prID), zap.String("user", userID))
	var exists bool
	if tx != nil {
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pr_reviewers WHERE pull_request_id=$1 AND user_id=$2)`, prID, userID).Scan(&exists); err != nil {
			r.Log.Error("IsReviewerAssigned: query failed (tx)", zap.Error(err))
			return false, err
		}
	} else {
		if err := r.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pr_reviewers WHERE pull_request_id=$1 AND user_id=$2)`, prID, userID).Scan(&exists); err != nil {
			r.Log.Error("IsReviewerAssigned: query failed", zap.Error(err))
			return false, err
		}
	}
	r.Log.Debug("IsReviewerAssigned: result", zap.Bool("exists", exists), zap.String("pr_id", prID), zap.String("user", userID))
	return exists, nil
}

func (r *Repositories) RemoveReviewer(ctx context.Context, tx *sql.Tx, prID, userID string) error {
	r.Log.Debug("RemoveReviewer: start", zap.String("pr_id", prID), zap.String("user", userID))
	_, err := tx.ExecContext(ctx, `DELETE FROM pr_reviewers WHERE pull_request_id=$1 AND user_id=$2`, prID, userID)
	if err != nil {
		r.Log.Error("RemoveReviewer: delete failed", zap.Error(err))
	}
	return err
}

func (r *Repositories) AddReviewer(ctx context.Context, tx *sql.Tx, prID, userID string) error {
	r.Log.Debug("AddReviewer: start", zap.String("pr_id", prID), zap.String("user", userID))
	_, err := tx.ExecContext(ctx, `INSERT INTO pr_reviewers(pull_request_id, user_id) VALUES($1,$2)`, prID, userID)
	if err != nil {
		r.Log.Error("AddReviewer: insert failed", zap.Error(err))
	}
	return err
}

func (r *Repositories) GetAssignedPRsForUser(ctx context.Context, userID string) ([]model.PullRequestShort, error) {
	r.Log.Debug("GetAssignedPRsForUser: start", zap.String("user", userID))
	rows, err := r.DB.QueryContext(ctx, `
        SELECT p.pull_request_id, p.pull_request_name, p.author_id, p.status
        FROM pull_requests p
        JOIN pr_reviewers r ON p.pull_request_id = r.pull_request_id
        WHERE r.user_id = $1
        ORDER BY p.created_at DESC
    `, userID)

	if err != nil {
		r.Log.Error("GetAssignedPRsForUser: query failed", zap.Error(err))
		return nil, err
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			r.Log.Error("GetAssignedPRsForUser: close rows failed", zap.Error(err))
		}
	}(rows)

	var out []model.PullRequestShort
	for rows.Next() {
		var s model.PullRequestShort
		if err := rows.Scan(&s.PullRequestID, &s.PullRequestName, &s.AuthorID, &s.Status); err != nil {
			r.Log.Error("GetAssignedPRsForUser: scan failed", zap.Error(err))
			return nil, err
		}
		out = append(out, s)
	}
	r.Log.Debug("GetAssignedPRsForUser: success", zap.Int("count", len(out)))
	return out, nil
}

func (r *Repositories) UpdatePR(ctx context.Context, pr model.PullRequest) error {
	r.Log.Debug("UpdatePR: start", zap.String("pr_id", pr.PullRequestID))
	var err error
	tx, err := r.BeginTx(ctx)
	if err != nil {
		r.Log.Error("UpdatePR: begin tx failed", zap.Error(err))
		return err
	}

	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			r.Log.Warn("UpdatePR: rollback failed", zap.Error(err))
		}
	}()

	_, err = tx.ExecContext(ctx,
		`UPDATE pull_requests 
		 SET pull_request_name=$1, status=$2, merged_at=$3 
		 WHERE pull_request_id=$4`,
		pr.PullRequestName, pr.Status, pr.MergedAt, pr.PullRequestID,
	)

	if err != nil {
		r.Log.Error("UpdatePR: update failed", zap.String("pr_id", pr.PullRequestID), zap.Error(err))
		return err
	}

	if err := tx.Commit(); err != nil {
		r.Log.Error("UpdatePR: commit failed", zap.String("pr_id", pr.PullRequestID), zap.Error(err))
		return err
	}
	r.Log.Info("UpdatePR: success", zap.String("pr_id", pr.PullRequestID))
	return nil
}
