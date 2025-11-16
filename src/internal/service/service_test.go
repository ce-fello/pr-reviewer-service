package service

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/ce-fello/pr-reviewer-service/src/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

type MockRepositories struct {
	mock.Mock
}

func (m *MockRepositories) CreateTeam(ctx context.Context, t model.Team) (model.Team, error) {
	args := m.Called(ctx, t)
	return args.Get(0).(model.Team), args.Error(1)
}

func (m *MockRepositories) GetTeam(ctx context.Context, teamName string) (model.Team, error) {
	args := m.Called(ctx, teamName)
	return args.Get(0).(model.Team), args.Error(1)
}

func (m *MockRepositories) SetUserIsActive(ctx context.Context, userID string, isActive bool) (model.User, error) {
	args := m.Called(ctx, userID, isActive)
	return args.Get(0).(model.User), args.Error(1)
}

func (m *MockRepositories) GetUser(ctx context.Context, userID string) (model.User, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(model.User), args.Error(1)
}

func (m *MockRepositories) GetActiveTeamMembersExcept(ctx context.Context, teamName, excludeUserID string) ([]string, error) {
	args := m.Called(ctx, teamName, excludeUserID)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockRepositories) CreatePRWithReviewers(ctx context.Context, pr model.PullRequest) error {
	args := m.Called(ctx, pr)
	return args.Error(0)
}

func (m *MockRepositories) GetPR(ctx context.Context, prID string) (model.PullRequest, error) {
	args := m.Called(ctx, prID)
	return args.Get(0).(model.PullRequest), args.Error(1)
}

func (m *MockRepositories) UpdatePR(ctx context.Context, pr model.PullRequest) error {
	args := m.Called(ctx, pr)
	return args.Error(0)
}

func (m *MockRepositories) GetAssignedPRsForUser(ctx context.Context, userID string) ([]model.PullRequestShort, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]model.PullRequestShort), args.Error(1)
}

func (m *MockRepositories) GetReviewStats(ctx context.Context) (map[string]int, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockRepositories) GetPRReviewStats(ctx context.Context) (map[string]int, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]int), args.Error(1)
}

type MockRandSource struct {
	values []int64
	index  int
}

func NewMockRandSource(values ...int64) *MockRandSource {
	return &MockRandSource{values: values}
}

func (m *MockRandSource) Int63() int64 {
	if m.index >= len(m.values) {
		m.index = 0
	}
	val := m.values[m.index]
	m.index++
	return val
}

func (m *MockRandSource) Seed(int64) {}

func createTestService() (*Service, *MockRepositories) {
	logger := zap.NewNop()
	mockRepo := new(MockRepositories)

	mockSource := NewMockRandSource(0, 1, 0) // предсказуемые значения
	mockRand := rand.New(mockSource)

	service := &Service{
		repo: mockRepo,
		log:  logger,
		rnd:  mockRand,
	}

	return service, mockRepo
}

func TestCreateTeam_Success(t *testing.T) {
	service, mockRepo := createTestService()

	team := model.Team{
		TeamName: "backend",
		Members: []model.TeamMember{
			{UserID: "u1", Username: "Alice", IsActive: true},
			{UserID: "u2", Username: "Bob", IsActive: true},
		},
	}

	mockRepo.On("GetTeam", mock.Anything, "backend").Return(model.Team{}, model.ErrNotFound)
	mockRepo.On("GetUser", mock.Anything, "u1").Return(model.User{}, model.ErrNotFound)
	mockRepo.On("GetUser", mock.Anything, "u2").Return(model.User{}, model.ErrNotFound)
	mockRepo.On("CreateTeam", mock.Anything, team).Return(team, nil)

	result, err := service.CreateTeam(context.Background(), team)

	assert.NoError(t, err)
	assert.Equal(t, team, result)
	mockRepo.AssertExpectations(t)
}

func TestCreateTeam_AlreadyExists(t *testing.T) {
	service, mockRepo := createTestService()

	team := model.Team{TeamName: "existing"}
	existingTeam := model.Team{TeamName: "existing", Members: []model.TeamMember{}}

	mockRepo.On("GetTeam", mock.Anything, "existing").Return(existingTeam, nil)

	result, err := service.CreateTeam(context.Background(), team)

	assert.Error(t, err)
	assert.Equal(t, model.Team{}, result)
	mockRepo.AssertNotCalled(t, "CreateTeam")
	mockRepo.AssertNotCalled(t, "GetUser")
}

func TestCreateTeam_UserAlreadyExists(t *testing.T) {
	service, mockRepo := createTestService()

	team := model.Team{
		TeamName: "new-team",
		Members: []model.TeamMember{
			{UserID: "existing-user", Username: "Existing", IsActive: true},
		},
	}

	existingUser := model.User{UserID: "existing-user", Username: "Existing", TeamName: "other-team"}

	mockRepo.On("GetTeam", mock.Anything, "new-team").Return(model.Team{}, model.ErrNotFound)
	mockRepo.On("GetUser", mock.Anything, "existing-user").Return(existingUser, nil)

	result, err := service.CreateTeam(context.Background(), team)

	assert.Error(t, err)
	assert.Equal(t, model.Team{}, result)
	mockRepo.AssertNotCalled(t, "CreateTeam")
}

func TestCreatePR_Success(t *testing.T) {
	logger := zap.NewNop()
	mockRepo := new(MockRepositories)
	mockRand := rand.New(rand.NewSource(1))

	service := &Service{
		repo: mockRepo,
		log:  logger,
		rnd:  mockRand,
	}

	author := model.User{
		UserID: "u1", Username: "Alice", TeamName: "backend", IsActive: true,
	}

	mockRepo.On("GetUser", mock.Anything, "u1").Return(author, nil)
	mockRepo.On("GetPR", mock.Anything, "pr1").Return(model.PullRequest{}, model.ErrNotFound)
	mockRepo.On("GetActiveTeamMembersExcept", mock.Anything, "backend", "u1").Return([]string{"u2", "u3", "u4"}, nil)
	mockRepo.On("CreatePRWithReviewers", mock.Anything, mock.MatchedBy(func(pr model.PullRequest) bool {
		return pr.PullRequestID == "pr1" &&
			pr.PullRequestName == "Test PR" &&
			pr.AuthorID == "u1" &&
			pr.Status == "OPEN" &&
			len(pr.Assigned) == 2
	})).Return(nil)

	result, err := service.CreatePR(context.Background(), "pr1", "Test PR", "u1")

	assert.NoError(t, err)
	assert.Equal(t, "pr1", result.PullRequestID)
	assert.Equal(t, "Test PR", result.PullRequestName)
	assert.Equal(t, "u1", result.AuthorID)
	assert.Equal(t, "OPEN", result.Status)
	assert.Len(t, result.Assigned, 2)

	for _, reviewer := range result.Assigned {
		assert.Contains(t, []string{"u2", "u3", "u4"}, reviewer)
	}
	assert.NotContains(t, result.Assigned, "u1") // автор не должен быть в ревьюерах

	mockRepo.AssertExpectations(t)
}

func TestCreatePR_AuthorNotFound(t *testing.T) {
	service, mockRepo := createTestService()

	mockRepo.On("GetUser", mock.Anything, "unknown").Return(model.User{}, model.ErrNotFound)

	result, err := service.CreatePR(context.Background(), "pr1", "Test", "unknown")

	assert.Error(t, err)
	assert.Equal(t, model.PullRequest{}, result)
}

func TestCreatePR_NoReviewersAvailable(t *testing.T) {
	service, mockRepo := createTestService()

	author := model.User{UserID: "u1", TeamName: "solo", IsActive: true}

	mockRepo.On("GetUser", mock.Anything, "u1").Return(author, nil)
	mockRepo.On("GetPR", mock.Anything, "pr1").Return(model.PullRequest{}, model.ErrNotFound)
	mockRepo.On("GetActiveTeamMembersExcept", mock.Anything, "solo", "u1").Return([]string{}, nil)
	mockRepo.On("CreatePRWithReviewers", mock.Anything, mock.MatchedBy(func(pr model.PullRequest) bool {
		return len(pr.Assigned) == 0
	})).Return(nil)

	result, err := service.CreatePR(context.Background(), "pr1", "Solo PR", "u1")

	assert.NoError(t, err)
	assert.Empty(t, result.Assigned)
}

func TestCreatePR_OnlyOneReviewerAvailable(t *testing.T) {
	service, mockRepo := createTestService()

	author := model.User{UserID: "u1", TeamName: "small", IsActive: true}

	mockRepo.On("GetUser", mock.Anything, "u1").Return(author, nil)
	mockRepo.On("GetPR", mock.Anything, "pr1").Return(model.PullRequest{}, model.ErrNotFound)
	mockRepo.On("GetActiveTeamMembersExcept", mock.Anything, "small", "u1").Return([]string{"u2"}, nil)
	mockRepo.On("CreatePRWithReviewers", mock.Anything, mock.MatchedBy(func(pr model.PullRequest) bool {
		return len(pr.Assigned) == 1 && pr.Assigned[0] == "u2"
	})).Return(nil)

	result, err := service.CreatePR(context.Background(), "pr1", "Small Team PR", "u1")

	assert.NoError(t, err)
	assert.Len(t, result.Assigned, 1)
	assert.Equal(t, "u2", result.Assigned[0])
}

func TestMergePR_Success(t *testing.T) {
	service, mockRepo := createTestService()

	openPR := model.PullRequest{
		PullRequestID: "pr1",
		Status:        "OPEN",
		Assigned:      []string{"u2"},
		CreatedAt:     time.Now().UTC(),
	}

	mockRepo.On("GetPR", mock.Anything, "pr1").Return(openPR, nil)
	mockRepo.On("UpdatePR", mock.Anything, mock.MatchedBy(func(pr model.PullRequest) bool {
		return pr.Status == "MERGED" && pr.MergedAt != nil
	})).Return(nil)

	result, err := service.MergePR(context.Background(), "pr1")

	assert.NoError(t, err)
	assert.Equal(t, "MERGED", result.Status)
	assert.NotNil(t, result.MergedAt)
}

func TestMergePR_Idempotent(t *testing.T) {
	service, mockRepo := createTestService()

	mergedTime := time.Now().UTC()
	mergedPR := model.PullRequest{
		PullRequestID: "pr1",
		Status:        "MERGED",
		MergedAt:      &mergedTime,
	}

	mockRepo.On("GetPR", mock.Anything, "pr1").Return(mergedPR, nil)

	result, err := service.MergePR(context.Background(), "pr1")

	assert.NoError(t, err)
	assert.Equal(t, "MERGED", result.Status)
	mockRepo.AssertNotCalled(t, "UpdatePR")
}

func TestReassignReviewer_Success(t *testing.T) {
	service, mockRepo := createTestService()

	pr := model.PullRequest{
		PullRequestID: "pr1",
		Status:        "OPEN",
		Assigned:      []string{"u2", "u3"},
		AuthorID:      "u1",
	}
	oldUser := model.User{UserID: "u2", TeamName: "backend", IsActive: true}

	mockRepo.On("GetPR", mock.Anything, "pr1").Return(pr, nil)
	mockRepo.On("GetUser", mock.Anything, "u2").Return(oldUser, nil)
	mockRepo.On("GetActiveTeamMembersExcept", mock.Anything, "backend", "u2").Return([]string{"u4", "u5"}, nil)
	mockRepo.On("UpdatePR", mock.Anything, mock.AnythingOfType("model.PullRequest")).Return(nil)

	result, newReviewer, err := service.ReassignReviewer(context.Background(), "pr1", "u2")

	assert.NoError(t, err)
	assert.Contains(t, []string{"u4", "u5"}, newReviewer)
	assert.NotContains(t, result.Assigned, "u2")
	assert.Contains(t, result.Assigned, newReviewer)
}

func TestReassignReviewer_MergedPR(t *testing.T) {
	service, mockRepo := createTestService()

	mergedPR := model.PullRequest{PullRequestID: "pr1", Status: "MERGED"}

	mockRepo.On("GetPR", mock.Anything, "pr1").Return(mergedPR, nil)

	_, _, err := service.ReassignReviewer(context.Background(), "pr1", "u2")

	assert.Error(t, err)
}

func TestReassignReviewer_NotAssigned(t *testing.T) {
	service, mockRepo := createTestService()

	pr := model.PullRequest{PullRequestID: "pr1", Status: "OPEN", Assigned: []string{"u3"}}

	mockRepo.On("GetPR", mock.Anything, "pr1").Return(pr, nil)

	_, _, err := service.ReassignReviewer(context.Background(), "pr1", "u2")

	assert.Error(t, err)
}

func TestReassignReviewer_NoCandidates(t *testing.T) {
	service, mockRepo := createTestService()

	pr := model.PullRequest{
		PullRequestID: "pr1",
		Status:        "OPEN",
		Assigned:      []string{"u2"},
		AuthorID:      "u1",
	}
	oldUser := model.User{UserID: "u2", TeamName: "small", IsActive: true}

	mockRepo.On("GetPR", mock.Anything, "pr1").Return(pr, nil)
	mockRepo.On("GetUser", mock.Anything, "u2").Return(oldUser, nil)
	mockRepo.On("GetActiveTeamMembersExcept", mock.Anything, "small", "u2").Return([]string{}, nil)

	_, _, err := service.ReassignReviewer(context.Background(), "pr1", "u2")

	assert.Error(t, err)
}

func TestReassignReviewer_ExcludeAuthorFromCandidates(t *testing.T) {
	service, mockRepo := createTestService()

	pr := model.PullRequest{
		PullRequestID: "pr1",
		Status:        "OPEN",
		Assigned:      []string{"u2"},
		AuthorID:      "u1",
	}
	oldUser := model.User{UserID: "u2", TeamName: "team", IsActive: true}

	mockRepo.On("GetPR", mock.Anything, "pr1").Return(pr, nil)
	mockRepo.On("GetUser", mock.Anything, "u2").Return(oldUser, nil)
	mockRepo.On("GetActiveTeamMembersExcept", mock.Anything, "team", "u2").Return([]string{"u1", "u3", "u4"}, nil)
	mockRepo.On("UpdatePR", mock.Anything, mock.MatchedBy(func(pr model.PullRequest) bool {
		for _, reviewer := range pr.Assigned {
			if reviewer == "u1" {
				return false
			}
		}
		return true
	})).Return(nil)

	_, newReviewer, err := service.ReassignReviewer(context.Background(), "pr1", "u2")

	assert.NoError(t, err)
	assert.NotEqual(t, "u1", newReviewer) // Автор не должен быть назначен
	assert.Contains(t, []string{"u3", "u4"}, newReviewer)
}

func TestGetPRsForReviewer(t *testing.T) {
	service, mockRepo := createTestService()

	expectedPRs := []model.PullRequestShort{
		{PullRequestID: "pr1", PullRequestName: "Test PR", AuthorID: "u1", Status: "OPEN"},
	}

	mockRepo.On("GetAssignedPRsForUser", mock.Anything, "u2").Return(expectedPRs, nil)

	result, err := service.GetPRsForReviewer(context.Background(), "u2")

	assert.NoError(t, err)
	assert.Equal(t, expectedPRs, result)
	mockRepo.AssertExpectations(t)
}

func TestSetUserIsActive(t *testing.T) {
	service, mockRepo := createTestService()

	_ = model.User{UserID: "u1", Username: "Alice", TeamName: "backend", IsActive: true}
	updatedUser := model.User{UserID: "u1", Username: "Alice", TeamName: "backend", IsActive: false}

	mockRepo.On("SetUserIsActive", mock.Anything, "u1", false).Return(updatedUser, nil)

	result, err := service.SetUserIsActive(context.Background(), "u1", false)

	assert.NoError(t, err)
	assert.Equal(t, updatedUser, result)
	mockRepo.AssertExpectations(t)
}

func TestSetUserIsActive_UserNotFound(t *testing.T) {
	service, mockRepo := createTestService()

	mockRepo.On("SetUserIsActive", mock.Anything, "unknown", true).Return(model.User{}, model.ErrNotFound)

	result, err := service.SetUserIsActive(context.Background(), "unknown", true)

	assert.Error(t, err)
	assert.Equal(t, model.User{}, result)
}
