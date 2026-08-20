package wallet

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AccountHandler interface {
	GetBalance(ctx *gin.Context)
	Deposit(ctx *gin.Context)
	Withdraw(ctx *gin.Context)
	Transfer(ctx *gin.Context)
}

type handler struct {
	service AccountService
}

type DepositRequest struct {
	AmountMinor      int64 `json:"amount_minor" binding:"required,min=1"`
	AccountID        int64 `json:"-"`
	RequestingUserID int64 `json:"-"`
}

type WithdrawRequest struct {
	AmountMinor      int64 `json:"amount_minor" binding:"required,min=1"`
	AccountID        int64 `json:"-"`
	RequestingUserID int64 `json:"-"`
}

type TransferRequest struct {
	AmountMinor          int64 `json:"amount_minor" binding:"required,min=1"`
	DestinationAccountID int64 `json:"destination_account_id" binding:"required"`
	SourceAccountID      int64 `json:"-"`
	RequestingUserID     int64 `json:"-"`
}

type HandlerError struct {
	Code    int
	Message string
}

func (e *HandlerError) Error() string {
	return e.Message
}

func NewHandler(service AccountService) *handler {
	return &handler{service: service}
}

func (h *handler) GetBalance(ctx *gin.Context) {
	accountID, requestingUserID, hErr := accountIDAndUserIDFromContext(ctx)
	if hErr != nil {
		ctx.JSON(hErr.Code, gin.H{"error": hErr.Error()})
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

func (h *handler) Deposit(ctx *gin.Context) {
	var req DepositRequest

	accountID, requestingUserID, hErr := accountIDAndUserIDFromContext(ctx)
	if hErr != nil {
		ctx.JSON(hErr.Code, gin.H{"error": hErr.Error()})
		return
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.AccountID = accountID
	req.RequestingUserID = requestingUserID

	if err := h.service.Deposit(ctx, req); err != nil {
		if errors.Is(err, ErrForbidden) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "deposit successful"})
}

func (h *handler) Withdraw(ctx *gin.Context) {
	var req WithdrawRequest

	accountID, requestingUserID, hErr := accountIDAndUserIDFromContext(ctx)
	if hErr != nil {
		ctx.JSON(hErr.Code, gin.H{"error": hErr.Error()})
		return
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.AccountID = accountID
	req.RequestingUserID = requestingUserID

	if err := h.service.Withdraw(ctx, req); err != nil {
		if errors.Is(err, ErrForbidden) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
			return
		}

		if errors.Is(err, ErrInsufficientFunds) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "insufficient funds"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "withdrawal successful"})
}

func (h *handler) Transfer(ctx *gin.Context) {
	var req TransferRequest

	idempotencyKey := ctx.GetHeader("X-Idempotency-Key")
	if idempotencyKey == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "X-Idempotency-Key header required"})
		return
	}

	accountID, requestingUserID, hErr := accountIDAndUserIDFromContext(ctx)
	if hErr != nil {
		ctx.JSON(hErr.Code, gin.H{"error": hErr.Error()})
		return
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.SourceAccountID = accountID
	req.RequestingUserID = requestingUserID

	transfer, err := h.service.Transfer(ctx, req, idempotencyKey)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
			return
		}

		if errors.Is(err, ErrInsufficientFunds) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "insufficient funds"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "transfer successful",
		"transfer_id":   transfer.Id,
		"source account_id":   transfer.SourceAccountID,
		"destination account_id":   transfer.DestinationAccountID,
		"transfer amount":   transfer.AmountMinor,
		"status":   transfer.Status,
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

func accountIDAndUserIDFromContext(ctx *gin.Context) (int64, int64, *HandlerError) {
	accountID, err := strconv.ParseInt(ctx.Param("accountId"), 10, 64)
	if err != nil {
		return 0, 0, &HandlerError{
			Code:    http.StatusBadRequest,
			Message: "invalid account id",
		}
	}

	userID, ok := userIDFromContext(ctx)
	if !ok {
		return 0, 0, &HandlerError{
			Code:    http.StatusUnauthorized,
			Message: "authentication required",
		}
	}

	return accountID, userID, nil
}
