package wallet

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeService struct {
	getBalanceCalled bool
	balance          int64

	depositCalled bool
	gotDepositReq DepositRequest

	withdrawCalled bool
	gotWithdrawReq WithdrawRequest

	transferCalled    bool
	gotTransferReq    TransferRequest
	gotIdempotencyKey string
	transfer          Transfer

	historyCalled    bool
	gotHistoryLimit  int
	gotHistoryOffset int
	historyEntries   []*LedgerEntry

	err error
}

func (s *fakeService) CreateAccount(ctx context.Context, userId int64, currency string) (*Account, error) {
	return nil, nil
}

func (s *fakeService) GetBalance(ctx context.Context, accountId, requestingUserId int64) (int64, error) {
	s.getBalanceCalled = true
	return s.balance, s.err
}

func (s *fakeService) Deposit(ctx context.Context, req DepositRequest) error {
	s.depositCalled = true
	s.gotDepositReq = req
	s.balance += req.AmountMinor
	return s.err
}

func (s *fakeService) Withdraw(ctx context.Context, req WithdrawRequest) error {
	s.withdrawCalled = true
	s.gotWithdrawReq = req
	return s.err
}

func (s *fakeService) Transfer(ctx context.Context, req TransferRequest, idempotencyKey string) (*Transfer, error) {
	s.transferCalled = true
	s.gotTransferReq = req
	s.gotIdempotencyKey = idempotencyKey
	return &s.transfer, s.err
}

func (s *fakeService) GetHistory(ctx context.Context, accountId, requestingUserId int64, limit, offset int) ([]*LedgerEntry, error) {
	s.historyCalled = true
	s.gotHistoryLimit = limit
	s.gotHistoryOffset = offset
	return s.historyEntries, s.err
}

func newJSONRequest(method, url string, body any) *http.Request {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestHandlerGetBalanceValidRequest(t *testing.T) {
	fakeSvc := &fakeService{
		balance: 5000,
	}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = httptest.NewRequest(http.MethodPost, "/accounts/1/balance", nil)
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(100))

	h.GetBalance(c)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	assert.True(t, fakeSvc.getBalanceCalled)

	var resp map[string]any
	err := json.Unmarshal(responseRecorder.Body.Bytes(), &resp)

	require.NoError(t, err)
	assert.Equal(t, float64(5000), resp["balance"])
}

func TestHandlerGetBalanceForbidden(t *testing.T) {
	fakeSvc := &fakeService{err: ErrForbidden}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = httptest.NewRequest(http.MethodGet, "/accounts/1/balance", nil)
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(999))

	h.GetBalance(c)

	assert.Equal(t, http.StatusNotFound, responseRecorder.Code)
}

func TestHandlerGetBalanceMissingContent(t *testing.T) {
	fakeSvc := &fakeService{}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = httptest.NewRequest(http.MethodGet, "/accounts/1/balance", nil)
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}

	h.GetBalance(c)

	assert.Equal(t, http.StatusUnauthorized, responseRecorder.Code)
	assert.False(t, fakeSvc.getBalanceCalled)
}

func TestHandlerGetBalanceInvalidAccountId(t *testing.T) {
	fakeSvc := &fakeService{}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = httptest.NewRequest(http.MethodGet, "/accounts/1/balance", nil)
	c.Params = gin.Params{{Key: "accountId", Value: "not-a-number"}}
	c.Set("user_id", int64(100))

	h.GetBalance(c)

	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	assert.False(t, fakeSvc.getBalanceCalled)
}

func TestHandlerDepositValidRequest(t *testing.T) {
	fakeSvc := &fakeService{}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = newJSONRequest(http.MethodPost, "/accounts/1/deposit", map[string]any{
		"amount_minor": 1500,
	})
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(100))

	h.Deposit(c)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	require.True(t, fakeSvc.depositCalled)

	assert.Equal(t, int64(1500), fakeSvc.gotDepositReq.AmountMinor)
	assert.Equal(t, int64(1), fakeSvc.gotDepositReq.AccountID)
	assert.Equal(t, int64(100), fakeSvc.gotDepositReq.RequestingUserID)
}

func TestHandlerDepositMissingAuth(t *testing.T) {
	fakeSvc := &fakeService{}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = newJSONRequest(http.MethodPost, "/accounts/1/deposit", map[string]any{
		"amount_minor": 1500,
	})
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}

	h.Deposit(c)

	assert.Equal(t, http.StatusUnauthorized, responseRecorder.Code)
	assert.False(t, fakeSvc.depositCalled)
}

func TestHandlerDepositInvalidAccountId(t *testing.T) {
	fakeSvc := &fakeService{}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = newJSONRequest(http.MethodPost, "/accounts/x/deposit", map[string]any{
		"amount_minor": 1500,
	})
	c.Params = gin.Params{{Key: "accountId", Value: "not-a-number"}}
	c.Set("user_id", int64(100))

	h.Deposit(c)

	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	assert.False(t, fakeSvc.depositCalled)
}

func TestHandlerDepositInvalidBody(t *testing.T) {
	fakeSvc := &fakeService{}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = newJSONRequest(http.MethodPost, "/accounts/1/deposit", map[string]any{
		"amount_minor": 0,
	})
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(100))

	h.Deposit(c)

	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	assert.False(t, fakeSvc.depositCalled)
}

func TestHandlerDepositForbidden(t *testing.T) {
	fakeSvc := &fakeService{err: ErrForbidden}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = newJSONRequest(http.MethodPost, "/accounts/1/deposit", map[string]any{
		"amount_minor": 1500,
	})
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(999))

	h.Deposit(c)

	assert.Equal(t, http.StatusNotFound, responseRecorder.Code)
}

func TestHandlerWithdrawValidRequest(t *testing.T) {
	fakeSvc := &fakeService{}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = newJSONRequest(http.MethodPost, "/accounts/1/withdraw", map[string]any{
		"amount_minor": 500,
	})
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(100))

	h.Withdraw(c)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	require.True(t, fakeSvc.withdrawCalled)
	assert.Equal(t, int64(500), fakeSvc.gotWithdrawReq.AmountMinor)
	assert.Equal(t, int64(1), fakeSvc.gotWithdrawReq.AccountID)
	assert.Equal(t, int64(100), fakeSvc.gotWithdrawReq.RequestingUserID)
}

func TestHandlerWithdrawInsufficientFunds(t *testing.T) {
	fakeSvc := &fakeService{err: ErrInsufficientFunds}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = newJSONRequest(http.MethodPost, "/accounts/1/withdraw", map[string]any{
		"amount_minor": 999999,
	})
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(100))

	h.Withdraw(c)

	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
}

func TestHandlerWithdrawForbidden(t *testing.T) {
	fakeSvc := &fakeService{err: ErrForbidden}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = newJSONRequest(http.MethodPost, "/accounts/1/withdraw", map[string]any{
		"amount_minor": 500,
	})
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(999))

	h.Withdraw(c)

	assert.Equal(t, http.StatusNotFound, responseRecorder.Code)
}

func TestHandlerWithdrawInvalidBody(t *testing.T) {
	fakeSvc := &fakeService{}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = newJSONRequest(http.MethodPost, "/accounts/1/withdraw", map[string]any{})
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(100))

	h.Withdraw(c)

	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	assert.False(t, fakeSvc.withdrawCalled)
}

func TestHandlerTransferValidRequest(t *testing.T) {
	fakeSvc := &fakeService{
		transfer: Transfer{
			Id:                   10,
			SourceAccountID:      1,
			DestinationAccountID: 2,
			AmountMinor:          700,
			Status:               "completed",
		},
	}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = newJSONRequest(http.MethodPost, "/accounts/1/transfer", map[string]any{
		"amount_minor":           700,
		"destination_account_id": 2,
	})
	c.Request.Header.Set("X-Idempotency-Key", "key-123")
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(100))

	h.Transfer(c)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	require.True(t, fakeSvc.transferCalled)
	assert.Equal(t, "key-123", fakeSvc.gotIdempotencyKey)
	assert.Equal(t, int64(1), fakeSvc.gotTransferReq.SourceAccountID)
	assert.Equal(t, int64(2), fakeSvc.gotTransferReq.DestinationAccountID)
	assert.Equal(t, int64(700), fakeSvc.gotTransferReq.AmountMinor)
	assert.Equal(t, int64(100), fakeSvc.gotTransferReq.RequestingUserID)
}

func TestHandlerTransferMissingIdempotencyKey(t *testing.T) {
	fakeSvc := &fakeService{}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = newJSONRequest(http.MethodPost, "/accounts/1/transfer", map[string]any{
		"amount_minor":           700,
		"destination_account_id": 2,
	})
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(100))

	h.Transfer(c)

	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	assert.False(t, fakeSvc.transferCalled)
}

func TestHandlerTransferForbidden(t *testing.T) {
	fakeSvc := &fakeService{err: ErrForbidden}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = newJSONRequest(http.MethodPost, "/accounts/1/transfer", map[string]any{
		"amount_minor":           700,
		"destination_account_id": 2,
	})
	c.Request.Header.Set("X-Idempotency-Key", "key-123")
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(999))

	h.Transfer(c)

	assert.Equal(t, http.StatusNotFound, responseRecorder.Code)
}

func TestHandlerTransferInsufficientFunds(t *testing.T) {
	fakeSvc := &fakeService{err: ErrInsufficientFunds}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = newJSONRequest(http.MethodPost, "/accounts/1/transfer", map[string]any{
		"amount_minor":           999999,
		"destination_account_id": 2,
	})
	c.Request.Header.Set("X-Idempotency-Key", "key-123")
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(100))

	h.Transfer(c)

	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
}

func TestHandlerTransferSameAccountReturnsBadRequest(t *testing.T) {
	fakeSvc := &fakeService{err: ErrInvalidTransfer}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = newJSONRequest(http.MethodPost, "/accounts/1/transfer", map[string]any{
		"amount_minor":           700,
		"destination_account_id": 1,
	})
	c.Request.Header.Set("X-Idempotency-Key", "key-123")
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(100))

	h.Transfer(c)

	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
}

func TestHandlerHistoryValidRequest(t *testing.T) {
	fakeSvc := &fakeService{
		historyEntries: []*LedgerEntry{
			{AccountID: 1, OperationType: OperationDeposit, EntryType: EntryCredit, AmountMinor: 500},
		},
	}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = httptest.NewRequest(http.MethodGet, "/accounts/1/history?limit=10&offset=0", nil)
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(100))

	h.GetHistory(c)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	require.True(t, fakeSvc.historyCalled)
	assert.Equal(t, 10, fakeSvc.gotHistoryLimit)
	assert.Equal(t, 0, fakeSvc.gotHistoryOffset)
}

func TestHandlerHistoryDefaultsWhenQueryParamsMissing(t *testing.T) {
	fakeSvc := &fakeService{}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = httptest.NewRequest(http.MethodGet, "/accounts/1/history", nil)
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(100))

	h.GetHistory(c)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	assert.Equal(t, 20, fakeSvc.gotHistoryLimit) // domyślny limit z handlera
	assert.Equal(t, 0, fakeSvc.gotHistoryOffset)
}

func TestHandlerHistoryInvalidLimitFallsBackToDefault(t *testing.T) {
	fakeSvc := &fakeService{}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = httptest.NewRequest(http.MethodGet, "/accounts/1/history?limit=abc&offset=-5", nil)
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(100))

	h.GetHistory(c)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	assert.Equal(t, 20, fakeSvc.gotHistoryLimit)
	assert.Equal(t, 0, fakeSvc.gotHistoryOffset)
}

func TestHandlerHistoryMissingAuth(t *testing.T) {
	fakeSvc := &fakeService{}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = httptest.NewRequest(http.MethodGet, "/accounts/1/history", nil)
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}

	h.GetHistory(c)

	assert.Equal(t, http.StatusUnauthorized, responseRecorder.Code)
	assert.False(t, fakeSvc.historyCalled)
}

func TestHandlerHistoryInvalidAccountId(t *testing.T) {
	fakeSvc := &fakeService{}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = httptest.NewRequest(http.MethodGet, "/accounts/x/history", nil)
	c.Params = gin.Params{{Key: "accountId", Value: "not-a-number"}}
	c.Set("user_id", int64(100))

	h.GetHistory(c)

	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	assert.False(t, fakeSvc.historyCalled)
}

func TestHandlerHistoryForbidden(t *testing.T) {
	fakeSvc := &fakeService{err: ErrForbidden}
	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	c.Request = httptest.NewRequest(http.MethodGet, "/accounts/1/history", nil)
	c.Params = gin.Params{{Key: "accountId", Value: "1"}}
	c.Set("user_id", int64(999))

	h.GetHistory(c)

	assert.Equal(t, http.StatusNotFound, responseRecorder.Code)
}
