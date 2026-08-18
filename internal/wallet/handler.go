package wallet

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AccountHandler interface {
	CreateAccount(ctx *gin.Context)
	GetBalance(ctx *gin.Context)
}

type handler struct {
	service AccountService
}

func NewHandler(service AccountService) *handler {
	return &handler{service: service}
}

type createAccountRequest struct {
	Currency string `json:"currency" binding:"required"`
}

func (h *handler) CreateAccount(ctx *gin.Context) {
	var req createAccountRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, ok := ctx.Get("user_id")
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "missing user context"})
		return
	}

	userIDInt, ok := userID.(int64)
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	account, err := h.service.CreateAccount(ctx, userIDInt, req.Currency)
	if err != nil {
		if errors.Is(err, ErrInvalidCurrency) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if errors.Is(err, ErrUserNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message":    "Account was successfully created",
		"account_id": account.Id,
		"currency":   account.Currency,
		"balance":    account.BalanceMinor,
	})
}

func (h *handler) GetBalance(ctx *gin.Context) {
	accountID, err := strconv.ParseInt(ctx.Param("accountId"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	balance, err := h.service.GetBalance(ctx, accountID)
	if err != nil {
		if errors.Is(err, ErrNoRows) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
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
