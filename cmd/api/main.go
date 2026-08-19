package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reggae-krk/payflow/internal/app"
	"github.com/reggae-krk/payflow/internal/config"
	"github.com/reggae-krk/payflow/internal/users"
	"github.com/reggae-krk/payflow/internal/wallet"
)

func main() {
	cfg := config.Load()
	pool, err := pgxpool.New(context.Background(), cfg.ConnString())

	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}

	defer pool.Close()

	usersRepo := users.NewUserRepository(pool)
	usersService := users.NewService(usersRepo)
	usersHandler := users.NewHandler(usersService)

	accountRepo := wallet.NewAccountRepository(pool)
	accountService := wallet.NewService(accountRepo)
	accountHandler := wallet.NewHandler(accountService)

	registrationService := app.NewService(pool)
	registrationHandler := app.NewHandler(registrationService)

	router := SetupRouter(usersHandler, accountHandler, registrationHandler, func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	log.Println("PayFlow API listening on :8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
