package wallet

import (
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
	err              error
}

func (s *fakeService) CreateAccount(ctx context.Context, userId int64, currency string) (*Account, error) {
	return nil, nil
}

func (s *fakeService) GetBalance(ctx context.Context, accountId, requestingUserId int64) (int64, error) {
	s.getBalanceCalled = true
	return s.balance, s.err
}

func (s *fakeService) Deposit(ctx context.Context, req DepositRequest) error {
	return nil
}

func (s *fakeService) Withdraw(ctx context.Context, req WithdrawRequest) error {
	return nil
}

func (s *fakeService) Transfer(ctx context.Context, req TransferRequest, idempotencyKey string) (*Transfer, error) {
	return nil, nil
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
