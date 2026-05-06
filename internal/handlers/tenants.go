package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"

	"git.assilvestrar.club/lourenco/k8s-mtp/internal/store"
	v1 "git.assilvestrar.club/lourenco/k8s-mtp/pkg/api/v1"
)

type TenantHandler struct {
	store  *store.Database
	logger *slog.Logger
}

func NewTenantHandler(store *store.Database, logger *slog.Logger) *TenantHandler {
	return &TenantHandler{store: store, logger: logger}
}

func (t *TenantHandler) ListTenants() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		list, err := t.store.ListTenants(r.Context())
		if err != nil {
			t.logger.Error("error listing the tenants", "err", err)
			http.Error(w, `{"error": "error listing the tenants"}`, http.StatusInternalServerError)
			return
		}
		var resp []TenantResponse
		for _, tenant := range list {
			resp = append(resp, TenantResponse{
				ID:         string(tenant.UID),
				Name:       tenant.Spec.Name,
				Namespace:  tenant.Status.Namespace,
				Tier:       string(tenant.Spec.Tier),
				OwnerEmail: tenant.Spec.OwnerEmail,
			})
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

func (t *TenantHandler) CreateTenant() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req CreateTenantRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.logger.Error("error decoding the request body", "err", err)
			http.Error(w, `{"error": "error decoding the request body"}`, http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			t.logger.Error("tenant name is required")
			http.Error(w, `{"error": "tenant name is required"}`, http.StatusBadRequest)
			return
		}
		if req.Tier != "free" && req.Tier != "pro" && req.Tier != "enterprise" {
			t.logger.Error("invalid tier")
			http.Error(w, `{"error": "invalid tier"}`, http.StatusBadRequest)
			return
		}
		if req.OwnerEmail == "" {
			t.logger.Error("owner email is required")
			http.Error(w, `{"error": "owner email is required"}`, http.StatusBadRequest)
			return
		}

		tenant := &v1.Tenant{
			Spec: v1.TenantSpec{
				Name:       req.Name,
				Tier:       v1.Tier(req.Tier),
				OwnerEmail: req.OwnerEmail,
			},
			Status: v1.TenantStatus{
				Namespace: "tenant-" + req.Name,
			},
		}
		tenant.UID = types.UID(uuid.New().String())

		if err := t.store.CreateTenant(r.Context(), tenant); err != nil {
			t.logger.Error("error creating tenant", "err", err)
			http.Error(w, `{"error": "error creating tenant"}`, http.StatusInternalServerError)
			return
		}

		resp := &TenantResponse{
			ID:         string(tenant.UID),
			Name:       tenant.Spec.Name,
			Namespace:  tenant.Status.Namespace,
			Tier:       string(tenant.Spec.Tier),
			OwnerEmail: tenant.Spec.OwnerEmail,
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}
}

func (t *TenantHandler) GetTenant() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id := r.PathValue("id")
		get, err := t.store.GetTenantByID(r.Context(), id)
		if err != nil {
			t.logger.Error("error getting the tenant", "err", err)
			http.Error(w, `{"error": "error getting the tenant"}`, http.StatusInternalServerError)
			return
		} else if get == nil {
			t.logger.Error("tenant is not found")
			http.Error(w, `{"error": "tenant is not found"}`, http.StatusNotFound)
			return
		}
		resp := &TenantResponse{
			ID:         string(get.UID),
			Name:       get.Spec.Name,
			Namespace:  get.Status.Namespace,
			Tier:       string(get.Spec.Tier),
			OwnerEmail: get.Spec.OwnerEmail,
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

func (t *TenantHandler) UpdateTenant() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id := r.PathValue("id")
		get, err := t.store.GetTenantByID(r.Context(), id)
		if err != nil {
			t.logger.Error("error getting the tenant", "err", err)
			http.Error(w, `{"error": "error getting the tenant"}`, http.StatusInternalServerError)
			return
		} else if get == nil {
			t.logger.Error("tenant is not found")
			http.Error(w, `{"error": "tenant is not found"}`, http.StatusNotFound)
			return
		}
		req := UpdateTenantRequest{}
		if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.logger.Error("error decoding the updating request", "err", err)
			http.Error(w, `{"error": "error decoding the updating request"}`, http.StatusBadRequest)
			return
		}
		if req.Tier != nil {
			get.Spec.Tier = v1.Tier(*req.Tier)
		}
		if req.OwnerEmail != nil {
			get.Spec.OwnerEmail = *req.OwnerEmail
		}
		err = t.store.UpdateTenant(r.Context(), get)
		if err != nil {
			t.logger.Error("error updating the tenant", "err", err)
			http.Error(w, `{"error": "error updating the tenant"}`, http.StatusInternalServerError)
			return
		}
		resp := &TenantResponse{
			ID:         string(get.UID),
			Name:       get.Spec.Name,
			Namespace:  get.Status.Namespace,
			Tier:       string(get.Spec.Tier),
			OwnerEmail: get.Spec.OwnerEmail,
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

func (t *TenantHandler) DeleteTenant() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id := r.PathValue("id")
		err := t.store.DeleteTenant(r.Context(), id)
		if err != nil {
			t.logger.Error("error deleting the tenant", "err", err)
			http.Error(w, `{"error": "error deleting the tenant"}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
