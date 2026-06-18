package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/toainguyen/ecommerce/product-service/internal/model"
	"github.com/toainguyen/ecommerce/product-service/internal/usecase"
)

// ProductHandler adapts HTTP requests to the usecase layer.
type ProductHandler struct {
	uc  *usecase.ProductUsecase
	log *slog.Logger
}

func NewProductHandler(uc *usecase.ProductUsecase, log *slog.Logger) *ProductHandler {
	return &ProductHandler{uc: uc, log: log}
}

// Create handles POST /api/v1/products.
func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var p model.Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.uc.CreateProduct(r.Context(), &p); err != nil {
		h.log.Error("create product failed", "error", err)
		writeError(w, http.StatusInternalServerError, "could not create product")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// Search handles GET /api/v1/products/search?q=...&size=...
func (h *ProductHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	results, err := h.uc.Search(r.Context(), q, size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"query": q, "results": results})
}

// Suggest handles GET /api/v1/products/suggest?q=...
func (h *ProductHandler) Suggest(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("q")
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	suggestions, err := h.uc.Suggest(r.Context(), prefix, size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "suggest failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"prefix": prefix, "suggestions": suggestions})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
