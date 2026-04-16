package transport

import (
	"encoding/json"
	"net/http"
	"online-store/internal/application"
	"strconv"

	"github.com/gorilla/mux"
)

type Handler struct {
	orderService    *application.OrderService
	customerService *application.CustomerService
	productService  *application.ProductService
}

func NewHandler(os *application.OrderService, cs *application.CustomerService, ps *application.ProductService) *Handler {
	return &Handler{
		orderService:    os,
		customerService: cs,
		productService:  ps,
	}
}

func (h *Handler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/orders", h.placeOrder).Methods("POST")
	r.HandleFunc("/customers/{id}/email", h.updateEmail).Methods("PUT")
	r.HandleFunc("/products", h.createProduct).Methods("POST")
}

type placeOrderRequest struct {
	CustomerID uint `json:"customerId"`
	Items      []struct {
		ProductID uint `json:"productId"`
		Quantity  int  `json:"quantity"`
	} `json:"items"`
}

func (h *Handler) placeOrder(w http.ResponseWriter, r *http.Request) {
	var req placeOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	appReq := application.PlaceOrderRequest{
		CustomerID: req.CustomerID,
		Items: make([]struct {
			ProductID uint
			Quantity  int
		}, len(req.Items)),
	}
	for i, it := range req.Items {
		appReq.Items[i].ProductID = it.ProductID
		appReq.Items[i].Quantity = it.Quantity
	}

	order, err := h.orderService.PlaceOrder(r.Context(), appReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

type updateEmailRequest struct {
	Email string `json:"email"`
}

func (h *Handler) updateEmail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "invalid customer id", http.StatusBadRequest)
		return
	}

	var req updateEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	err = h.customerService.UpdateEmail(r.Context(), uint(id), req.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"email updated"}`))
}

type createProductRequest struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func (h *Handler) createProduct(w http.ResponseWriter, r *http.Request) {
	var req createProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	product, err := h.productService.CreateProduct(r.Context(), req.Name, req.Price)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(product)
}
