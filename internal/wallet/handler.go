package wallet

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AccountHandler interface {
	GetBalance(ctx *gin.Context)
}

type handler struct {
	service AccountService
}

func NewHandler(service AccountService) *handler {
	return &handler{service: service}
}

func (h *handler) GetBalance(ctx *gin.Context) {
	accountID, err := strconv.ParseInt(ctx.Param("accountId"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	
	requestingUserID, ok := userIDFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "missing or invalid user context"})
		return
	}

	balance, err := h.service.GetBalance(ctx, accountID, requestingUserID)

	if err != nil {
		if errors.Is(err, ErrNoRows) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		if errors.Is(err, ErrForbidden) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Balance was successfully obtained",
		"balance": balance,
	})
}

func userIDFromContext(ctx *gin.Context) (int64, bool) {
	userID, ok := ctx.Get("user_id")
	if !ok {
		return 0, false
	}

	userIDInt, ok := userID.(int64)
	return userIDInt, ok
}