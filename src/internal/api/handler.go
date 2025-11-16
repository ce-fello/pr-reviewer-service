package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ce-fello/pr-reviewer-service/src/internal/api/apiErrors"
	"github.com/ce-fello/pr-reviewer-service/src/internal/model"
	"github.com/ce-fello/pr-reviewer-service/src/internal/service"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *service.Service
	log *zap.Logger
}

func NewHandler(svc *service.Service, logger *zap.Logger) *Handler {
	return &Handler{svc: svc, log: logger}
}

func RegisterRoutes(r *chi.Mux, h *Handler) {
	r.Post("/team/add", withTimeout(h.createTeam))
	r.Get("/team/get", withTimeout(h.getTeam))
	r.Post("/users/setIsActive", withTimeout(h.setIsActive))
	r.Post("/pullRequest/create", withTimeout(h.createPR))
	r.Post("/pullRequest/merge", withTimeout(h.mergePR))
	r.Post("/pullRequest/reassign", withTimeout(h.reassign))
	r.Get("/users/getReview", withTimeout(h.getUserPRs))
	r.Get("/stats", withTimeout(h.getStats))
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
}

func withTimeout(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		next(w, r.WithContext(ctx))
	}
}

func (h *Handler) createTeam(w http.ResponseWriter, r *http.Request) {
	var t model.Team
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, apiErrors.InternalError, "invalid body")
		return
	}
	if t.TeamName == "" {
		writeError(w, http.StatusBadRequest, apiErrors.InternalError, "team_name required")
		return
	}
	for _, m := range t.Members {
		if m.UserID == "" || m.Username == "" {
			writeError(w, http.StatusBadRequest, apiErrors.InternalError, "all members must have user_id and username")
			return
		}
	}
	team, err := h.svc.CreateTeam(r.Context(), t)
	if err != nil {
		handleSvcError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"team": team})
}

func (h *Handler) getTeam(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		writeError(w, http.StatusBadRequest, apiErrors.InternalError, "team_name required")
		return
	}
	team, err := h.svc.GetTeam(r.Context(), teamName)
	if err != nil {
		handleSvcError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, team)
}

func (h *Handler) setIsActive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   string `json:"user_id"`
		IsActive bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		writeError(w, http.StatusBadRequest, apiErrors.InternalError, "user_id required")
		return
	}
	user, err := h.svc.SetUserIsActive(r.Context(), req.UserID, req.IsActive)
	if err != nil {
		handleSvcError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (h *Handler) createPR(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PRID   string `json:"pull_request_id"`
		PRName string `json:"pull_request_name"`
		Author string `json:"author_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PRID == "" || req.PRName == "" || req.Author == "" {
		writeError(w, http.StatusBadRequest, apiErrors.InternalError, "pull_request_id, pull_request_name and author_id required")
		return
	}
	pr, err := h.svc.CreatePR(r.Context(), req.PRID, req.PRName, req.Author)
	if err != nil {
		handleSvcError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"pr": pr})
}

func (h *Handler) mergePR(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PRID string `json:"pull_request_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PRID == "" {
		writeError(w, http.StatusBadRequest, apiErrors.InternalError, "pull_request_id required")
		return
	}
	pr, err := h.svc.MergePR(r.Context(), req.PRID)
	if err != nil {
		handleSvcError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pr": pr})
}

func (h *Handler) reassign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PRID    string `json:"pull_request_id"`
		OldUser string `json:"old_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PRID == "" || req.OldUser == "" {
		writeError(w, http.StatusBadRequest, apiErrors.InternalError, "pull_request_id and old_user_id required")
		return
	}
	pr, replacedBy, err := h.svc.ReassignReviewer(r.Context(), req.PRID, req.OldUser)
	if err != nil {
		handleSvcError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pr": pr, "replaced_by": replacedBy})
}

func (h *Handler) getUserPRs(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, apiErrors.InternalError, "user_id required")
		return
	}
	prs, err := h.svc.GetPRsForReviewer(r.Context(), userID)
	if err != nil {
		handleSvcError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_id": userID, "pull_requests": prs})
}

func (h *Handler) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetStats(r.Context())
	if err != nil {
		handleSvcError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, code int, errCode apiErrors.ErrorCode, message string) {
	writeJSON(w, code, map[string]any{
		"error": map[string]any{"code": errCode, "message": message},
	})
}

func handleSvcError(w http.ResponseWriter, err error) {
	var e apiErrors.APIError
	switch {
	case errors.As(err, &e):
		switch e.Code {
		case apiErrors.TeamExists:
			writeError(w, http.StatusConflict, e.Code, e.Message)
		case apiErrors.PRExists:
			writeError(w, http.StatusConflict, e.Code, e.Message)
		case apiErrors.PRAlreadyMerged:
			writeError(w, http.StatusConflict, e.Code, e.Message)
		case apiErrors.NotAssigned:
			writeError(w, http.StatusConflict, e.Code, e.Message)
		case apiErrors.NoCandidate:
			writeError(w, http.StatusConflict, e.Code, e.Message)
		case apiErrors.NotFound:
			writeError(w, http.StatusNotFound, e.Code, e.Message)
		default:
			writeError(w, http.StatusInternalServerError, apiErrors.InternalError, e.Message)
		}
	default:
		writeError(w, http.StatusInternalServerError, apiErrors.InternalError, err.Error())
	}
}
