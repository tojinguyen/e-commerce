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
		h.log.Error("create product failed", "error", err, "sku", p.SKU)
		writeError(w, http.StatusInternalServerError, "could not create product")
		return
	}
	h.log.Info("product created", "id", p.ID, "sku", p.SKU, "name", p.Name)
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
		h.log.Error("update product failed", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "could not update product")
		return
	}
	h.log.Info("product updated", "id", id, "sku", updated.SKU)
	writeJSON(w, http.StatusOK, updated)
}

// Get godoc
// @Summary      Get a product
// @Description  Returns a single product by id from PostgreSQL (source of truth)
// @Tags         products
// @Produce      json
// @Param        id   path      string  true  "Product ID"
// @Success      200  {object}  model.Product
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/v1/products/{id} [get]
func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing product id")
		return
	}
	p, err := h.uc.GetProduct(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "product not found")
			return
		}
		h.log.Error("get product failed", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "could not get product")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// BatchGetRequest is the payload for fetching multiple products by id.
type BatchGetRequest struct {
	IDs []string `json:"ids"`
}

// BatchGet godoc
// @Summary      Get products by ids
// @Description  Returns the products matching the given ids in one call; missing ids are omitted
// @Tags         products
// @Accept       json
// @Produce      json
// @Param        body  body      BatchGetRequest  true  "Product ids"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /api/v1/products/batch [post]
func (h *ProductHandler) BatchGet(w http.ResponseWriter, r *http.Request) {
	var req BatchGetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids must not be empty")
		return
	}
	products, err := h.uc.GetProducts(r.Context(), req.IDs)
	if err != nil {
		h.log.Error("batch get products failed", "error", err, "count", len(req.IDs))
		writeError(w, http.StatusInternalServerError, "could not get products")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"products": products})
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
		h.log.Error("delete product failed", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "could not delete product")
		return
	}
	h.log.Info("product deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// AdjustStockRequest is the payload for atomically changing a product's stock.
type AdjustStockRequest struct {
	Delta int `json:"delta"`
}

// AdjustStock godoc
// @Summary      Adjust product stock
// @Description  Atomically adds delta to the product stock. Negative delta reserves units; positive releases them. Returns 409 when stock would go below zero.
// @Tags         products
// @Accept       json
// @Produce      json
// @Param        id    path      string             true  "Product ID"
// @Param        body  body      AdjustStockRequest true  "Stock delta"
// @Success      204   "No Content"
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Failure      409   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /api/v1/products/{id}/stock [patch]
func (h *ProductHandler) AdjustStock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing product id")
		return
	}
	var req AdjustStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Delta == 0 {
		writeError(w, http.StatusBadRequest, "delta must not be zero")
		return
	}
	if err := h.uc.AdjustStock(r.Context(), id, req.Delta); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "product not found")
			return
		}
		if errors.Is(err, repository.ErrInsufficientStock) {
			writeError(w, http.StatusConflict, "insufficient stock")
			return
		}
		h.log.Error("adjust stock failed", "error", err, "id", id, "delta", req.Delta)
		writeError(w, http.StatusInternalServerError, "could not adjust stock")
		return
	}
	h.log.Info("stock adjusted", "id", id, "delta", req.Delta)
	w.WriteHeader(http.StatusNoContent)
}

// Search godoc
// @Summary      Search products
// @Description  Full-text search against Elasticsearch; returns matching products with relevance scores
// @Tags         products
// @Produce      json
// @Param        q          query  string  false  "Search query (omit for a pure range search)"
// @Param        min_price  query  int     false  "Minimum price in cents (inclusive)"
// @Param        max_price  query  int     false  "Maximum price in cents (inclusive)"
// @Param        size       query  int     false  "Max results to return (default 10)"
// @Success      200   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]string
// @Router       /api/v1/products/search [get]
func (h *ProductHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	params := model.SearchParams{Query: q, Size: size}
	if v := r.URL.Query().Get("min_price"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			params.MinPriceCents = &n
		}
	}
	if v := r.URL.Query().Get("max_price"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			params.MaxPriceCents = &n
		}
	}
	results, err := h.uc.Search(r.Context(), params)
	if err != nil {
		h.log.Error("search failed", "error", err, "query", q)
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}
	h.log.Info("search executed", "query", q, "min_price", r.URL.Query().Get("min_price"), "max_price", r.URL.Query().Get("max_price"), "size", size, "hits", len(results))
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
		h.log.Error("suggest failed", "error", err, "prefix", prefix)
		writeError(w, http.StatusInternalServerError, "suggest failed")
		return
	}
	h.log.Info("suggest executed", "prefix", prefix, "size", size, "count", len(suggestions))
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
