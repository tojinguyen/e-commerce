package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/toainguyen/ecommerce/order-service/internal/model"
	"github.com/toainguyen/ecommerce/order-service/internal/usecase"
)

// OrderHandler adapts HTTP to the order usecase.
type OrderHandler struct {
	uc  *usecase.OrderUsecase
	log *slog.Logger
}

func NewOrderHandler(uc *usecase.OrderUsecase, log *slog.Logger) *OrderHandler {
	return &OrderHandler{uc: uc, log: log}
}

// Create godoc
// @Summary      Create an order
// @Description  Starts an OrderWorkflow saga via Temporal; returns 202 Accepted with the pending order
// @Tags         orders
// @Accept       json
// @Produce      json
// @Param        order  body      model.Order  true  "Order payload"
// @Success      202    {object}  model.Order
// @Failure      400    {object}  map[string]string
// @Failure      500    {object}  map[string]string
// @Router       /api/v1/orders [post]
func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var o model.Order
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	created, err := h.uc.CreateOrder(r.Context(), &o)
	if err != nil {
		h.log.Error("create order failed", "error", err)
		writeError(w, http.StatusInternalServerError, "could not create order")
		return
	}
	writeJSON(w, http.StatusAccepted, created)
}

// Get godoc
// @Summary      Get an order
// @Description  Returns the order with the given UUID; reflects the latest Temporal workflow status
// @Tags         orders
// @Produce      json
// @Param        id   path      string  true  "Order UUID"
// @Success      200  {object}  model.Order
// @Failure      404  {object}  map[string]string
// @Router       /api/v1/orders/{id} [get]
func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	order, err := h.uc.GetOrder(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
