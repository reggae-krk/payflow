package main

import (
	"github.com/gin-gonic/gin"
	"github.com/reggae-krk/payflow/internal/auth"
	"github.com/reggae-krk/payflow/internal/users"
)

func SetupRouter(userHandler users.UserHandler, healthHandler gin.HandlerFunc) *gin.Engine {
	server := gin.Default()

	server.GET("/health", healthHandler)

	registerRoutes(server, userHandler)

	return server
}

func registerRoutes(server *gin.Engine, usersHandler users.UserHandler) {
	server.POST("/register", usersHandler.Register)
	server.POST("/login", usersHandler.Login)

	authGroup := server.Group("/")
	authGroup.Use(auth.RequireAuth)
}
