package app

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/reggae-krk/payflow/internal/users"
)

type RegistrationHandler interface {
	Register(c *gin.Context)
}

type registrationHandler struct {
	service RegistrationService
}

func NewHandler(service RegistrationService) *registrationHandler {
	return &registrationHandler{
		service: service,
	}
}

type registrationRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

func (h *registrationHandler) Register(c *gin.Context) {
	var req registrationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, account, err := h.service.RegisterWithDefaultAccount(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, users.ErrEmailTaken) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "User was successfully created",
		"user_id":    user.Id,
		"email":      user.Email,
		"account_id": account.Id,
		"currency":   account.Currency,
	})
}
