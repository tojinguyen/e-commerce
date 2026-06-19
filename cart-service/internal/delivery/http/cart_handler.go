package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/toainguyen/ecommerce/cart-service/internal/model"
	"github.com/toainguyen/ecommerce/cart-service/internal/usecase"
)

// CartHandler adapts HTTP to the cart usecase.
type CartHandler struct {
	uc  *usecase.CartUsecase
	log *slog.Logger
}

func NewCartHandler(uc *usecase.CartUsecase, log *slog.Logger) *CartHandler {
	return &CartHandler{uc: uc, log: log}
}

// userID resolves the cart owner from the X-User-ID header (auth is out of scope).
func userID(r *http.Request) string {
	if id := r.Header.Get("X-User-ID"); id != "" {
		return id
	}
	return "anonymous"
}

// Upsert godoc
// @Summary      Upsert cart
// @Description  Creates or replaces the cart for the user identified by the X-User-ID header
// @Tags         carts
// @Accept       json
// @Produce      json
// @Param        X-User-ID  header    string      false  "User identifier (falls back to 'anonymous')"
// @Param        cart       body      model.Cart  true   "Cart payload"
// @Success      200        {object}  model.Cart
// @Failure      400        {object}  map[string]string
// @Failure      500        {object}  map[string]string
// @Router       /api/v1/carts [put]
func (h *CartHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	var cart model.Cart
	if err := json.NewDecoder(r.Body).Decode(&cart); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cart.UserID = userID(r)
	if err := h.uc.SaveCart(r.Context(), &cart); err != nil {
		h.log.Error("save cart failed", "error", err)
		writeError(w, http.StatusInternalServerError, "could not save cart")
		return
	}
	writeJSON(w, http.StatusOK, cart)
}

// Get godoc
// @Summary      Get cart
// @Description  Returns the current cart for the user identified by the X-User-ID header
// @Tags         carts
// @Produce      json
// @Param        X-User-ID  header    string  false  "User identifier (falls back to 'anonymous')"
// @Success      200        {object}  model.Cart
// @Failure      500        {object}  map[string]string
// @Router       /api/v1/carts [get]
func (h *CartHandler) Get(w http.ResponseWriter, r *http.Request) {
	cart, err := h.uc.GetCart(r.Context(), userID(r))
	if err != nil {
		h.log.Error("get cart failed", "error", err)
		writeError(w, http.StatusInternalServerError, "could not fetch cart")
		return
	}
	writeJSON(w, http.StatusOK, cart)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
