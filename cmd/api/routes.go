package main

import (
	"github.com/gin-gonic/gin"
	"github.com/reggae-krk/payflow/internal/app"
	"github.com/reggae-krk/payflow/internal/auth"
	"github.com/reggae-krk/payflow/internal/users"
	"github.com/reggae-krk/payflow/internal/wallet"
)

func SetupRouter(userHandler users.UserHandler, accountHandler wallet.AccountHandler, registrationHandler app.RegistrationHandler, healthHandler gin.HandlerFunc) *gin.Engine {
	server := gin.Default()

	server.GET("/health", healthHandler)

	registerRoutes(server, userHandler, accountHandler, registrationHandler)

	return server
}

func registerRoutes(server *gin.Engine, usersHandler users.UserHandler, accountHandler wallet.AccountHandler, registrationHandler app.RegistrationHandler) {
	server.POST("/register", registrationHandler.Register)
	server.POST("/login", usersHandler.Login)

	authGroup := server.Group("/")
	authGroup.Use(auth.RequireAuth)

	authGroup.GET("/accounts/:accountId/balance", accountHandler.GetBalance)
	authGroup.POST("/accounts/:accountId/deposit", accountHandler.Deposit)
	authGroup.POST("/accounts/:accountId/withdraw", accountHandler.Withdraw)
	authGroup.POST("/accounts/:accountId/transfer", accountHandler.Transfer)
}
