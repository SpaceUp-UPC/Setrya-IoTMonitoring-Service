package rest

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/space-management/internal/application"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/space-management/internal/domain"
)

type Handler struct { App application.Service }

type spaceRequest struct {
	OwnerID string `json:"ownerId"`
	Name string `json:"name"`
	Description string `json:"description"`
	Location domain.Location `json:"location"`
	Dimensions domain.Dimensions `json:"dimensions"`
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/spaces"), "/")
	if path == "" { h.create(w, r); return }
	parts := strings.Split(path, "/")
	if len(parts) == 2 && parts[1] == "availability" { h.setAvailability(w, r, parts[0]); return }
	if len(parts) != 1 { http.NotFound(w, r); return }
	switch r.Method {
	case http.MethodGet: h.get(w, r, parts[0])
	case http.MethodPut: h.update(w, r, parts[0])
	default: http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h Handler) create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	var request spaceRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil { http.Error(w, "invalid JSON", http.StatusBadRequest); return }
	id, err := newID()
	if err != nil { http.Error(w, "could not create space id", http.StatusInternalServerError); return }
	space, err := domain.NewSpace(id, request.OwnerID, request.Name, request.Description, request.Location, request.Dimensions)
	if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
	if err := h.App.Create(r.Context(), space); err != nil { http.Error(w, "could not save space", http.StatusInternalServerError); return }
	w.Header().Set("Location", "/api/v1/spaces/"+space.ID)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(space)
}

func (h Handler) get(w http.ResponseWriter, r *http.Request, id string) {
	space, err := h.App.Get(r.Context(), id)
	if err == application.ErrNotFound { http.Error(w, err.Error(), http.StatusNotFound); return }
	if err != nil { http.Error(w, "could not retrieve space", http.StatusInternalServerError); return }
	_ = json.NewEncoder(w).Encode(space)
}

func (h Handler) update(w http.ResponseWriter, r *http.Request, id string) {
	var request spaceRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil { http.Error(w, "invalid JSON", http.StatusBadRequest); return }
	if err := h.App.Update(r.Context(), id, request.Name, request.Description, request.Location, request.Dimensions); err != nil {
		if err == application.ErrNotFound { http.Error(w, err.Error(), http.StatusNotFound) } else { http.Error(w, err.Error(), http.StatusBadRequest) }
		return
	}
	h.get(w, r, id)
}

func (h Handler) setAvailability(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPatch { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	var request struct { Active *bool `json:"active"` }
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Active == nil { http.Error(w, "active is required and must be boolean", http.StatusBadRequest); return }
	if err := h.App.SetStatus(r.Context(), id, *request.Active); err != nil {
		if err == application.ErrNotFound { http.Error(w, err.Error(), http.StatusNotFound) } else { http.Error(w, "could not update availability", http.StatusInternalServerError) }
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
