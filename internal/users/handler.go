package users

import (
	// "errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/reggae-krk/payflow/internal/auth"
)

type UserHandler interface {
	Login(ctx *gin.Context)
}

type handler struct {
	service UserService
}

func NewHandler(service UserService) *handler {
	return &handler{service: service}
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

func (h *handler) Login(ctx *gin.Context) {
	var req loginRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.service.Login(ctx, req.Email, req.Password)

	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	token, err := auth.GenerateToken(user.Id)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "User was successfully logged in",
		"email":   user.Email,
		"token":   token,
	})
}
