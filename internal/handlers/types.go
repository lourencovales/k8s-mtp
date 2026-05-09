package handlers

// so that we don't leak k8s internals
type CreateTenantRequest struct {
	Name       string `json:"name"`
	Tier       string `json:"tier"`
	OwnerEmail string `json:"owner_email"`
}

type UpdateTenantRequest struct {
	Tier       *string `json:"tier"`
	OwnerEmail *string `json:"owner_email"`
}

type TenantResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Tier       string `json:"tier"`
	OwnerEmail string `json:"owner_email"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type AddMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

type MemberResponse struct {
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}
