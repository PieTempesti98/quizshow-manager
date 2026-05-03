package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/PieTempesti98/quizshow/internal/api"
	"github.com/PieTempesti98/quizshow/internal/auth"
	"github.com/PieTempesti98/quizshow/internal/category"
	"github.com/PieTempesti98/quizshow/internal/db"
	"github.com/PieTempesti98/quizshow/internal/question"
)

func main() {
	cfg, err := auth.LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	pool, err := db.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	adminRepo := auth.NewAdminRepository(pool)
	tokenRepo := auth.NewRefreshTokenRepository(pool)
	svc := auth.NewService(adminRepo, tokenRepo, cfg)
	h := auth.NewHandler(svc, cfg)

	categoryRepo := category.NewCategoryRepository(pool)
	categorySvc := category.NewService(categoryRepo)
	categoryHandler := category.NewHandler(categorySvc)

	questionRepo := question.NewRepository(pool)
	questionSvc := question.NewService(questionRepo)
	questionHandler := question.NewHandler(questionSvc)

	app := fiber.New(fiber.Config{
		BodyLimit: 6 * 1024 * 1024, // 6MB: allows 5MB CSV + multipart overhead
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{
					Code:    "INTERNAL_ERROR",
					Message: err.Error(),
				},
			})
		},
	})

	v1 := app.Group("/api/v1")

	// Public auth routes
	authGroup := v1.Group("/auth")
	authGroup.Post("/login", h.Login)
	authGroup.Post("/refresh", h.Refresh)

	// Public: no auth required (must be registered before the protected group)
	v1.Get("/questions/import/template", questionHandler.ImportTemplate)

	// Protected routes — require admin Bearer token
	protected := v1.Group("", auth.RequireAdmin(cfg))
	protected.Post("/auth/logout", h.Logout)

	protected.Get("/categories", categoryHandler.List)
	protected.Post("/categories", categoryHandler.Create)
	protected.Patch("/categories/:id", categoryHandler.Rename)
	protected.Delete("/categories/:id", categoryHandler.Delete)

	protected.Get("/questions", questionHandler.List)
	protected.Post("/questions", questionHandler.Create)
	protected.Patch("/questions/:id", questionHandler.Update)
	protected.Delete("/questions/:id", questionHandler.Delete)
	protected.Post("/questions/import", questionHandler.Import)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-quit
		log.Println("shutting down...")
		_ = app.Shutdown()
	}()

	log.Printf("listening on :%s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
