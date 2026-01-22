package auth

import (
	dto "casino_backend/internal/api/dto/auth"
	"casino_backend/internal/converter"
	"casino_backend/internal/service"
	"casino_backend/pkg/req"
	"casino_backend/pkg/resp"
	"log"
	"net/http"
)

type HandlerDeps struct {
	serv service.AuthService
}

type Handler struct {
	serv service.AuthService
}

func NewHandler(deps HandlerDeps) *Handler {
	return &Handler{serv: deps.serv}
}

// Register создаёт пользователя, открывает сессию
// и возвращает access_token и session_id через cookies
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	requestBody, err := req.Decode[dto.RegisterRequest](r.Body)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	data, err := h.serv.Register(
		r.Context(),
		converter.RegisterRequestToUserModel(&requestBody),
	)
	if err != nil {
		log.Println("Register error:", err)
		http.Error(w, "register failed", http.StatusConflict)
		return
	}

	setSessionIDCookie(w, data.SessionID)

	setRefreshTokenCookie(w, data.RefreshToken)

	resp.WriteJSONResponse(w, http.StatusCreated, map[string]interface{}{
		"access_token": data.AccessToken,
	})
}

// Login создаёт сессию и возвращает access_token и session_id через cookies
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	requestBody, err := req.Decode[dto.LoginRequest](r.Body)
	if err != nil {
		return
	}

	accessToken, sessionID, err := h.serv.Login(
		r.Context(),
		requestBody.Login,
		requestBody.Password,
	)
	if err != nil {
		log.Println("Login error:", err)
		http.Error(w, "login failed", http.StatusUnauthorized)
		return
	}

	setSessionIDCookie(w, sessionID)

	setRefreshTokenCookie(w, accessToken)

	w.WriteHeader(http.StatusOK)
}

// Refresh обновляет access_token по session_id
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("session_id")
	if err != nil {
		http.Error(w, "no session_id cookie", http.StatusUnauthorized)
		return
	}

	sessionID := c.Value

	accessToken, err := h.serv.Refresh(r.Context(), sessionID)
	if err != nil {
		log.Println("Refresh error:", err)
		http.Error(w, "refresh failed", http.StatusUnauthorized)
		return
	}

	setRefreshTokenCookie(w, accessToken)

	w.WriteHeader(http.StatusOK)
}

// Logout закрывает сессию по session_id
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("session_id")
	if err != nil {
		http.Error(w, "no session_id cookie", http.StatusUnauthorized)
		return
	}

	sessionID := c.Value

	err = h.serv.Logout(r.Context(), sessionID)
	if err != nil {
		log.Println("Logout error:", err)
		http.Error(w, "logout failed", http.StatusInternalServerError)
		return
	}

	deleteSessionIDCookie(w)
	deleteRefreshTokenCookie(w)

	w.WriteHeader(http.StatusNoContent)
}

// setRefreshTokenCookie устанавливает cookie с refresh_token
func setRefreshTokenCookie(w http.ResponseWriter, refreshToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/refresh",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 30, // 30 дней
	})
}

// deleteRefreshTokenCookie удаляет cookie с session_id
func deleteRefreshTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// setSessionIDCookie устанавливает cookie с session_id
func setSessionIDCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   30 * 24 * 60 * 60, // 30 дней
	})
}

// deleteSessionIDCookie удаляет cookie с session_id
func deleteSessionIDCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}
