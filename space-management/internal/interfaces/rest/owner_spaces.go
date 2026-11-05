package rest

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/space-management/internal/application"
)

type OwnerSpacesHandler struct {
	App application.Service
}

func (h OwnerSpacesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/owners/")
	if !strings.HasSuffix(path, "/spaces") {
		http.NotFound(w, r)
		return
	}
	ownerID := strings.TrimSuffix(path, "/spaces")
	ownerID = strings.Trim(ownerID, "/")
	if ownerID == "" || strings.Contains(ownerID, "/") {
		http.Error(w, "ownerId is required", http.StatusBadRequest)
		return
	}

	spaces, err := h.App.List(r.Context(), ownerID)
	if err != nil {
		http.Error(w, "could not retrieve owner spaces", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(spaces)
}
