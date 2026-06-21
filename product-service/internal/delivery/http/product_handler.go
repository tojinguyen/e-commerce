package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/toainguyen/ecommerce/product-service/internal/model"
	"github.com/toainguyen/ecommerce/product-service/internal/repository"
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

// Create godoc
// @Summary      Create a product
// @Description  Persists a new product to PostgreSQL and queues it for Elasticsearch indexing
// @Tags         products
// @Accept       json
// @Produce      json
// @Param        product  body      model.Product  true  "Product payload"
// @Success      201      {object}  model.Product
// @Failure      400      {object}  map[string]string
// @Failure      500      {object}  map[string]string
// @Router       /api/v1/products [post]
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

// Update godoc
// @Summary      Update a product
// @Description  Replaces an existing product in PostgreSQL; the change is projected to Elasticsearch via CDC
// @Tags         products
// @Accept       json
// @Produce      json
// @Param        id       path      string         true  "Product ID"
// @Param        product  body      model.Product  true  "Product payload"
// @Success      200      {object}  model.Product
// @Failure      400      {object}  map[string]string
// @Failure      404      {object}  map[string]string
// @Failure      500      {object}  map[string]string
// @Router       /api/v1/products/{id} [put]
func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing product id")
		return
	}
	var p model.Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p.ID = id
	updated, err := h.uc.UpdateProduct(r.Context(), &p)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "product not found")
			return
		}
		h.log.Error("update product failed", "error", err)
		writeError(w, http.StatusInternalServerError, "could not update product")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// Delete godoc
// @Summary      Delete a product
// @Description  Removes a product from PostgreSQL; the deletion is propagated to Elasticsearch via CDC
// @Tags         products
// @Produce      json
// @Param        id   path  string  true  "Product ID"
// @Success      204  "No Content"
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/v1/products/{id} [delete]
func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing product id")
		return
	}
	if err := h.uc.DeleteProduct(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "product not found")
			return
		}
		h.log.Error("delete product failed", "error", err)
		writeError(w, http.StatusInternalServerError, "could not delete product")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Search godoc
// @Summary      Search products
// @Description  Full-text search against Elasticsearch; returns matching products with relevance scores
// @Tags         products
// @Produce      json
// @Param        q     query  string  true   "Search query"
// @Param        size  query  int     false  "Max results to return (default 10)"
// @Success      200   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]string
// @Router       /api/v1/products/search [get]
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

// Suggest godoc
// @Summary      Autocomplete suggestions
// @Description  Returns product name suggestions for a given prefix using Elasticsearch completion
// @Tags         products
// @Produce      json
// @Param        q     query  string  true   "Prefix to autocomplete"
// @Param        size  query  int     false  "Max suggestions (default 5)"
// @Success      200   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]string
// @Router       /api/v1/products/suggest [get]
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
