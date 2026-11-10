package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"authservice.com/internal/service"
)

type Handler struct {
	Serv *service.Service
}
type UserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func NewHandler(serv *service.Service) *Handler {
	return &Handler{Serv: serv}
}

// RegisterHandler godoc
// @Summary Registration of a new user
// @Description Creates a new user in the system. The password is hashed using bcrypt.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body UserRequest true "Registration data (email, password)"
// @Success 201 {string} string "Created"
// @Failure 400 {string} string "Bad Request (invalid JSON or the user already exists)"
// @Failure 500 {string} string "Internal Server Error"
// @Router /register [post]
func (h *Handler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method  not allowed", http.StatusMethodNotAllowed)
		return
	}
	var u UserRequest
	err := json.NewDecoder(r.Body).Decode(&u)
	if err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		log.Printf("Decoding error: %v\n", err)
		return
	}
	err = h.Serv.Register(r.Context(), u.Email, u.Password)
	if err != nil {
		http.Error(w, "status bad request", http.StatusBadRequest)
		log.Printf("Registration error: %v\n", err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// LoginHandler godoc
// @Summary System login (obtaining JWT)
// @Description Verifies the email and password, returns a JWT token (valid for 24 hours).
// @Tags auth
// @Accept json
// @Produce text/plain
// @Param request body UserRequest true "Login credentials (email, password)"
// @Success 200 {string} string "JWT token"
// @Failure 400 {string} string "Bad Request (invalid JSON or invalid password)"
// @Router /login [post]
func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method  not allowed", http.StatusMethodNotAllowed)
		return
	}
	var u UserRequest
	err1 := json.NewDecoder(r.Body).Decode(&u)
	if err1 != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		log.Printf("Decoding error: %v\n", err1)
		return
	}
	tokenStr, err2 := h.Serv.Login(r.Context(), u.Email, u.Password)
	if err2 != nil {
		http.Error(w, "status bad request", http.StatusBadRequest)
		log.Printf("Token creation error: %v\n", err2)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err2 = json.NewEncoder(w).Encode(map[string]string{"token": tokenStr}); err2 != nil {
		log.Printf("Encoding error: %v\n", err2)
	}
}
