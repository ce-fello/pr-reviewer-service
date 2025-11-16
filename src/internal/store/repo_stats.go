package store

import (
	"context"
	"database/sql"

	"go.uber.org/zap"
)

func (r *Repositories) queryCountMap(ctx context.Context, query string, scanKey func(*sql.Rows) (string, error), logPrefix string) (map[string]int, error) {
	r.Log.Debug(logPrefix + ": start")
	rows, err := r.DB.QueryContext(ctx, query)
	if err != nil {
		r.Log.Error(logPrefix+": query failed", zap.Error(err))
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			r.Log.Info(logPrefix+": close rows failed", zap.Error(err))
		}
	}()

	result := make(map[string]int)
	for rows.Next() {
		key, err := scanKey(rows)
		if err != nil {
			r.Log.Error(logPrefix+": scan failed", zap.Error(err))
			return nil, err
		}
		result[key]++
	}

	r.Log.Debug(logPrefix+": success", zap.Int("items", len(result)))
	return result, nil
}

func (r *Repositories) GetReviewStats(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT user_id, COUNT(*) 
		FROM pr_reviewers
		GROUP BY user_id
	`
	return r.queryCountMap(ctx, query, func(rows *sql.Rows) (string, error) {
		var userID string
		var count int
		if err := rows.Scan(&userID, &count); err != nil {
			return "", err
		}
		return userID, nil
	}, "GetReviewStats")
}

func (r *Repositories) GetPRReviewStats(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT pull_request_id, COUNT(*) 
		FROM pr_reviewers
		GROUP BY pull_request_id
	`
	return r.queryCountMap(ctx, query, func(rows *sql.Rows) (string, error) {
		var prID string
		var count int
		if err := rows.Scan(&prID, &count); err != nil {
			return "", err
		}
		return prID, nil
	}, "GetPRReviewStats")
}
