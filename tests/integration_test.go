package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type Team struct {
	TeamName string       `json:"team_name"`
	Members  []TeamMember `json:"members"`
}

type TeamMember struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

type PullRequest struct {
	PullRequestID   string    `json:"pull_request_id"`
	PullRequestName string    `json:"pull_request_name"`
	AuthorID        string    `json:"author_id"`
	Status          string    `json:"status"`
	Assigned        []string  `json:"assigned_reviewers"`
	CreatedAt       time.Time `json:"createdAt,omitempty"`
	MergedAt        *string   `json:"mergedAt,omitempty"`
}

type PullRequestShort struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
	Status          string `json:"status"`
}

type IntegrationTestSuite struct {
	suite.Suite
	baseURL string
	client  *http.Client
}

func (suite *IntegrationTestSuite) SetupSuite() {
	suite.baseURL = "http://localhost:8080"
	suite.client = &http.Client{Timeout: 10 * time.Second}
	suite.waitForService()
}

func (suite *IntegrationTestSuite) waitForService() {
	for i := 0; i < 30; i++ {
		resp, err := http.Get(suite.baseURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			fmt.Println("✅ Service is ready!")
			return
		}
		fmt.Printf("⏳ Waiting for service... (attempt %d/30)\n", i+1)
		time.Sleep(1 * time.Second)
	}
	suite.T().Fatal("❌ Service failed to start within 30 seconds")
}

func (suite *IntegrationTestSuite) TestFullFlow() {
	t := suite.T()

	teamName := fmt.Sprintf("test-team-%d", time.Now().Unix())
	userPrefix := fmt.Sprintf("user-%d", time.Now().Unix())

	team := Team{
		TeamName: teamName,
		Members: []TeamMember{
			{UserID: userPrefix + "-1", Username: "Test User 1", IsActive: true},
			{UserID: userPrefix + "-2", Username: "Test User 2", IsActive: true},
			{UserID: userPrefix + "-3", Username: "Test User 3", IsActive: true},
			{UserID: userPrefix + "-4", Username: "Test User 4", IsActive: false}, // неактивный
		},
	}

	resp, err := suite.doRequest("POST", "/team/add", team)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Should create team successfully")

	var createTeamResp struct {
		Team Team `json:"team"`
	}
	err = json.NewDecoder(resp.Body).Decode(&createTeamResp)
	assert.NoError(t, err)
	assert.Len(t, createTeamResp.Team.Members, 4, "Team should have 4 members")
	fmt.Println("✅ Team created successfully")

	resp, err = suite.doRequest("GET", "/team/get?team_name="+teamName, nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should get team successfully")

	var getTeamResp Team
	err = json.NewDecoder(resp.Body).Decode(&getTeamResp)
	assert.NoError(t, err)
	assert.Len(t, getTeamResp.Members, 4, "Retrieved team should have 4 members")
	fmt.Println("✅ Team retrieved successfully")

	prID := fmt.Sprintf("pr-%d", time.Now().Unix())
	prCreateReq := map[string]string{
		"pull_request_id":   prID,
		"pull_request_name": "Integration Test PR",
		"author_id":         userPrefix + "-1",
	}

	resp, err = suite.doRequest("POST", "/pullRequest/create", prCreateReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Should create PR successfully")

	var prResp struct {
		PR PullRequest `json:"pr"`
	}
	err = json.NewDecoder(resp.Body).Decode(&prResp)
	assert.NoError(t, err)

	assert.Equal(t, "OPEN", prResp.PR.Status, "PR should be OPEN")
	assert.Len(t, prResp.PR.Assigned, 2, "Should assign 2 reviewers")
	assert.NotContains(t, prResp.PR.Assigned, userPrefix+"-1", "Author should not be assigned as reviewer")
	assert.NotContains(t, prResp.PR.Assigned, userPrefix+"-4", "Inactive user should not be assigned")
	fmt.Println("✅ PR created with reviewers")

	if len(prResp.PR.Assigned) > 0 {
		resp, err = suite.doRequest("GET", "/users/getReview?user_id="+prResp.PR.Assigned[0], nil)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should get user's PRs successfully")

		var userPRsResp struct {
			UserID       string             `json:"user_id"`
			PullRequests []PullRequestShort `json:"pull_requests"`
		}
		err = json.NewDecoder(resp.Body).Decode(&userPRsResp)
		assert.NoError(t, err)
		assert.Len(t, userPRsResp.PullRequests, 1, "User should have 1 assigned PR")
		assert.Equal(t, prID, userPRsResp.PullRequests[0].PullRequestID, "PR ID should match")
		fmt.Println("✅ User PRs retrieved successfully")
	}

	mergeReq := map[string]string{
		"pull_request_id": prID,
	}

	resp, err = suite.doRequest("POST", "/pullRequest/merge", mergeReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should merge PR successfully")

	var mergeResp struct {
		PR PullRequest `json:"pr"`
	}
	err = json.NewDecoder(resp.Body).Decode(&mergeResp)
	assert.NoError(t, err)
	assert.Equal(t, "MERGED", mergeResp.PR.Status, "PR should be MERGED")
	assert.NotNil(t, mergeResp.PR.MergedAt, "MergedAt should be set")
	fmt.Println("✅ PR merged successfully")

	reassignReq := map[string]string{
		"pull_request_id": prID,
		"old_user_id":     prResp.PR.Assigned[0],
	}

	resp, err = suite.doRequest("POST", "/pullRequest/reassign", reassignReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode, "Should not allow reassign on merged PR")
	fmt.Println("✅ Correctly prevented reassign on merged PR")
}

func (suite *IntegrationTestSuite) TestErrorScenarios() {
	t := suite.T()

	prReq := map[string]string{
		"pull_request_id":   "error-pr-1",
		"pull_request_name": "Error PR",
		"author_id":         "non-existent-user-123456",
	}

	resp, err := suite.doRequest("POST", "/pullRequest/create", prReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Should return 404 for non-existent author")
	fmt.Println("✅ Correctly handled non-existent author")

	resp, err = suite.doRequest("GET", "/team/get?team_name=non-existent-team-123456", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Should return 404 for non-existent team")
	fmt.Println("✅ Correctly handled non-existent team")

	team := Team{
		TeamName: "duplicate-team-test",
		Members: []TeamMember{
			{UserID: "dup-u1", Username: "Duplicate User 1", IsActive: true},
		},
	}

	resp, err = suite.doRequest("POST", "/team/add", team)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "First team creation should succeed")

	resp, err = suite.doRequest("POST", "/team/add", team)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode, "Second team creation should conflict")
	fmt.Println("✅ Correctly handled duplicate team creation")
}

func (suite *IntegrationTestSuite) TestEdgeCases() {
	t := suite.T()

	soloTeamName := fmt.Sprintf("solo-team-%d", time.Now().Unix())
	soloTeam := Team{
		TeamName: soloTeamName,
		Members: []TeamMember{
			{UserID: "solo-user", Username: "Solo User", IsActive: true},
		},
	}

	resp, err := suite.doRequest("POST", "/team/add", soloTeam)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	prReq := map[string]string{
		"pull_request_id":   "solo-pr",
		"pull_request_name": "Solo PR",
		"author_id":         "solo-user",
	}

	resp, err = suite.doRequest("POST", "/pullRequest/create", prReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var prResp struct {
		PR PullRequest `json:"pr"`
	}
	err = json.NewDecoder(resp.Body).Decode(&prResp)
	assert.NoError(t, err)
	assert.Empty(t, prResp.PR.Assigned, "Should have no reviewers for solo team")
	fmt.Println("✅ Correctly handled solo team case")

	teamName := fmt.Sprintf("deactivate-team-%d", time.Now().Unix())
	deactivateTeam := Team{
		TeamName: teamName,
		Members: []TeamMember{
			{UserID: "deact-u1", Username: "User 1", IsActive: true},
			{UserID: "deact-u2", Username: "User 2", IsActive: true},
			{UserID: "deact-u3", Username: "User 3", IsActive: true},
		},
	}

	resp, err = suite.doRequest("POST", "/team/add", deactivateTeam)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	deactivateReq := map[string]interface{}{
		"user_id":   "deact-u2",
		"is_active": false,
	}

	resp, err = suite.doRequest("POST", "/users/setIsActive", deactivateReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	prReq2 := map[string]string{
		"pull_request_id":   "deact-pr",
		"pull_request_name": "Deactivate Test PR",
		"author_id":         "deact-u1",
	}

	resp, err = suite.doRequest("POST", "/pullRequest/create", prReq2)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	err = json.NewDecoder(resp.Body).Decode(&prResp)
	assert.NoError(t, err)
	assert.NotContains(t, prResp.PR.Assigned, "deact-u2", "Deactivated user should not be assigned")
	fmt.Println("✅ Correctly handled user deactivation")
}

func (suite *IntegrationTestSuite) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var req *http.Request
	var err error

	if body != nil {
		jsonBody, _ := json.Marshal(body)
		req, err = http.NewRequest(method, suite.baseURL+path, bytes.NewBuffer(jsonBody))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, suite.baseURL+path, nil)
		if err != nil {
			return nil, err
		}
	}

	return suite.client.Do(req)
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
