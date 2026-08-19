package app

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/reggae-krk/payflow/internal/users"
)

type RegistrationHandler interface {
	Register(ctx *gin.Context)
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

func (h *registrationHandler) Register(ctx *gin.Context) {
	var req registrationRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, account, err := h.service.RegisterWithDefaultAccount(ctx, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, users.ErrEmailTaken) {
			ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message":    "User was successfully created",
		"user_id":    user.Id,
		"email":      user.Email,
		"account_id": account.Id,
		"currency":   account.Currency,
	})
}
