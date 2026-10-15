package main

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/reggae-krk/payflow/internal/app"
	"github.com/reggae-krk/payflow/internal/auth"
	"github.com/reggae-krk/payflow/internal/config"
	"github.com/reggae-krk/payflow/internal/users"
	"github.com/reggae-krk/payflow/internal/wallet"
)

func SetupRouter(timeoutCfg config.TimeoutConfig, userHandler users.UserHandler, accountHandler wallet.AccountHandler, registrationHandler app.RegistrationHandler, healthHandler gin.HandlerFunc) *gin.Engine {
	server := gin.Default()

	server.GET("/health", timeoutMiddleware(timeoutCfg.Health), healthHandler)

	registerRoutes(server, timeoutCfg, userHandler, accountHandler, registrationHandler)

	return server
}

func registerRoutes(server *gin.Engine, timeoutCfg config.TimeoutConfig, usersHandler users.UserHandler, accountHandler wallet.AccountHandler, registrationHandler app.RegistrationHandler) {

	regLogGroup := server.Group("/")
	regLogGroup.Use(timeoutMiddleware(timeoutCfg.LogReg))

	regLogGroup.POST("/register", registrationHandler.Register)
	regLogGroup.POST("/login", usersHandler.Login)

	authGroup := server.Group("/")
	authGroup.Use(timeoutMiddleware(timeoutCfg.Request), auth.RequireAuth)

	authGroup.GET("/accounts/:accountId/balance", accountHandler.GetBalance)
	authGroup.POST("/accounts/:accountId/deposit", accountHandler.Deposit)
	authGroup.POST("/accounts/:accountId/withdraw", accountHandler.Withdraw)
	authGroup.POST("/accounts/:accountId/transfer", accountHandler.Transfer)
	authGroup.GET("/accounts/:accountId/history", accountHandler.GetHistory)
}

func timeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
