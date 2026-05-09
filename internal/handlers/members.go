package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"git.assilvestrar.club/lourenco/k8s-mtp/internal/store"
)

type MemberHandler struct {
	store  *store.Database
	logger *slog.Logger
}

func NewMemberHandler(store *store.Database, logger *slog.Logger) *MemberHandler {
	return &MemberHandler{store: store, logger: logger}
}

func (m *MemberHandler) AddMember() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id := r.PathValue("id")
		get, err := m.store.GetTenantByID(r.Context(), id)
		if err != nil {
			m.logger.Error("error getting the tenant", "err", err)
			http.Error(w, `{"error": "error getting the tenant"}`, http.StatusInternalServerError)
			return
		} else if get == nil {
			m.logger.Error("tenant is not found")
			http.Error(w, `{"error": "tenant is not found"}`, http.StatusNotFound)
			return
		}
		var req AddMemberRequest
		if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.logger.Error("error decoding the add member request", "err", err)
			http.Error(w, `{"error": "error decoding the add member request"}`, http.StatusBadRequest)
			return
		}
		if req.Role != "admin" && req.Role != "operator" {
			m.logger.Error("member has the wrong role")
			http.Error(w, `{"error": "member has the wrong role"}`, http.StatusBadRequest)
			return
		}
		if err := m.store.AddMember(r.Context(), id, req.UserID, req.Role); err != nil {
			m.logger.Error("error adding the member", "err", err)
			http.Error(w, `{"error": "error adding the member"}`, http.StatusInternalServerError)
			return
		}
		resp := &MemberResponse{
			UserID: req.UserID,
			Role:   req.Role,
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}
}

func (m *MemberHandler) RemoveMember() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id := r.PathValue("id")
		userID := r.PathValue("userId")
		if err := m.store.RemoveMember(r.Context(), id, userID); err != nil {
			m.logger.Error("error removing the member", "err", err)
			http.Error(w, `{"error": "error removing the member"}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (m *MemberHandler) ListMembers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id := r.PathValue("id")
		members, err := m.store.ListMembers(r.Context(), id)
		if err != nil {
			m.logger.Error("error getting the members", "err", err)
			http.Error(w, `{"error": "error getting the members"}`, http.StatusInternalServerError)
			return
		}
		var resp []MemberResponse
		for _, member := range members {
			resp = append(resp, MemberResponse{
				UserID:    member.UserID,
				Role:      member.Role,
				CreatedAt: member.CreatedAt.Format(time.RFC3339),
			})
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}
