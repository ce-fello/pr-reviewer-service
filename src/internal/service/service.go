package service

import (
	"context"
	"errors"
	"github.com/ce-fello/pr-reviewer-service/src/internal/api/apiErrors"
	"github.com/ce-fello/pr-reviewer-service/src/internal/model"
	"github.com/ce-fello/pr-reviewer-service/src/internal/store"
	"math/rand"
	"time"

	"go.uber.org/zap"
)

type Service struct {
	repo store.Repository
	log  *zap.Logger
	rnd  *rand.Rand
}

type Stats struct {
	UserAssignments map[string]int `json:"user_assignments"`
	PRAssignments   map[string]int `json:"pr_assignments"`
}

func NewService(repos store.Repository, logger *zap.Logger) *Service {
	src := rand.NewSource(time.Now().UnixNano())
	return &Service{
		repo: repos,
		log:  logger,
		rnd:  rand.New(src),
	}
}

func (s *Service) CreateTeam(ctx context.Context, t model.Team) (model.Team, error) {
	if existing, _ := s.repo.GetTeam(ctx, t.TeamName); existing.TeamName != "" {
		return model.Team{}, apiErrors.APIError{Code: apiErrors.TeamExists, Message: "team_name already exists"}
	}

	for _, m := range t.Members {
		if _, err := s.repo.GetUser(ctx, m.UserID); err == nil {
			return model.Team{}, apiErrors.APIError{Code: apiErrors.TeamExists, Message: "user_id " + m.UserID + " already exists"}
		} else if !errors.Is(err, model.ErrNotFound) {
			return model.Team{}, err
		}
	}
	if _, err := s.repo.CreateTeam(ctx, t); err != nil {
		return model.Team{}, err
	}
	return t, nil
}

func (s *Service) GetTeam(ctx context.Context, teamName string) (model.Team, error) {
	t, err := s.repo.GetTeam(ctx, teamName)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return model.Team{}, apiErrors.APIError{Code: apiErrors.NotFound, Message: "team not found"}
		}
		return model.Team{}, err
	}
	return t, nil
}

func (s *Service) SetUserIsActive(ctx context.Context, userID string, isActive bool) (model.User, error) {
	u, err := s.repo.SetUserIsActive(ctx, userID, isActive)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return model.User{}, apiErrors.APIError{Code: apiErrors.NotFound, Message: "user not found"}
		}
		return model.User{}, err
	}
	return u, nil
}

func (s *Service) CreatePR(ctx context.Context, prID, prName, authorID string) (model.PullRequest, error) {
	author, err := s.repo.GetUser(ctx, authorID)
	if err != nil {
		return model.PullRequest{}, apiErrors.APIError{Code: apiErrors.NotFound, Message: "author not found"}
	}

	if _, err := s.repo.GetPR(ctx, prID); err == nil {
		return model.PullRequest{}, apiErrors.APIError{Code: apiErrors.PRExists, Message: "PR id already exists"}
	} else if !errors.Is(err, model.ErrNotFound) {
		return model.PullRequest{}, err
	}

	candidates, err := s.repo.GetActiveTeamMembersExcept(ctx, author.TeamName, authorID)
	if err != nil {
		return model.PullRequest{}, err
	}
	selected := chooseUpToN(s.rnd, candidates, 2)

	pr := model.PullRequest{
		PullRequestID:   prID,
		PullRequestName: prName,
		AuthorID:        authorID,
		Status:          "OPEN",
		Assigned:        selected,
		CreatedAt:       time.Now().UTC(),
	}

	if err := s.repo.CreatePRWithReviewers(ctx, pr); err != nil {
		return model.PullRequest{}, err
	}
	return pr, nil
}

func (s *Service) MergePR(ctx context.Context, prID string) (model.PullRequest, error) {
	pr, err := s.repo.GetPR(ctx, prID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return model.PullRequest{}, apiErrors.APIError{Code: apiErrors.NotFound, Message: "PR not found"}
		}
		return model.PullRequest{}, err
	}
	if pr.Status == "MERGED" {
		return pr, nil
	}
	pr.Status = "MERGED"
	now := time.Now().UTC()
	pr.MergedAt = &now

	if err := s.repo.UpdatePR(ctx, pr); err != nil {
		return model.PullRequest{}, err
	}
	return pr, nil
}

func (s *Service) ReassignReviewer(ctx context.Context, prID, oldUserID string) (model.PullRequest, string, error) {
	pr, err := s.repo.GetPR(ctx, prID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return model.PullRequest{}, "", apiErrors.APIError{Code: apiErrors.NotFound, Message: "PR not found"}
		}
		return model.PullRequest{}, "", err
	}
	if pr.Status == "MERGED" {
		return model.PullRequest{}, "", apiErrors.APIError{Code: apiErrors.PRAlreadyMerged, Message: "cannot reassign on merged PR"}
	}

	assigned := false
	for _, u := range pr.Assigned {
		if u == oldUserID {
			assigned = true
			break
		}
	}
	if !assigned {
		return model.PullRequest{}, "", apiErrors.APIError{Code: apiErrors.NotAssigned, Message: "reviewer is not assigned to this PR"}
	}

	oldUser, err := s.repo.GetUser(ctx, oldUserID)
	if err != nil {
		return model.PullRequest{}, "", err
	}

	candidates, err := s.repo.GetActiveTeamMembersExcept(ctx, oldUser.TeamName, oldUserID)
	if err != nil {
		return model.PullRequest{}, "", err
	}

	var filtered []string
	for _, c := range candidates {
		if c == pr.AuthorID {
			continue
		}
		skip := false
		for _, a := range pr.Assigned {
			if a == c {
				skip = true
				break
			}
		}
		if !skip {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		return model.PullRequest{}, "", apiErrors.APIError{Code: apiErrors.NoCandidate, Message: "no active replacement candidate in team"}
	}

	newReviewer := filtered[s.rnd.Intn(len(filtered))]

	for i, u := range pr.Assigned {
		if u == oldUserID {
			pr.Assigned[i] = newReviewer
			break
		}
	}

	if err := s.repo.UpdatePR(ctx, pr); err != nil {
		return model.PullRequest{}, "", err
	}

	return pr, newReviewer, nil
}

func (s *Service) GetPRsForReviewer(ctx context.Context, userID string) ([]model.PullRequestShort, error) {
	return s.repo.GetAssignedPRsForUser(ctx, userID)
}

func chooseUpToN(r *rand.Rand, items []string, n int) []string {
	if len(items) <= n {
		out := append([]string(nil), items...)
		r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
		return out
	}
	out := append([]string(nil), items...)
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out[:n]
}

func (s *Service) GetStats(ctx context.Context) (Stats, error) {
	userStats, err := s.repo.GetReviewStats(ctx)
	if err != nil {
		return Stats{}, err
	}
	prStats, err := s.repo.GetPRReviewStats(ctx)
	if err != nil {
		return Stats{}, err
	}
	return Stats{
		UserAssignments: userStats,
		PRAssignments:   prStats,
	}, nil
}
