package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/yourorg/pim-demo/handlers"
	"github.com/yourorg/pim-demo/internal/config"
	"github.com/yourorg/pim-demo/internal/middleware"
	"github.com/yourorg/pim-demo/services"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Check if command is provided
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			runMigrations(cfg)
			return
		case "serve":
			// Continue to start server
		default:
			fmt.Printf("Unknown command: %s\n", os.Args[1])
			fmt.Println("Available commands: migrate, serve")
			os.Exit(1)
		}
	}

	// Setup database
	ctx := context.Background()
	db, err := config.SetupDatabase(ctx, cfg.DatabaseURL, cfg.LogLevel)
	if err != nil {
		log.Fatalf("Failed to setup database: %v", err)
	}
	defer config.CloseDatabase(db)

	log.Println("Database connected successfully")

	// Setup OpenTracing (NoopTracer for now)
	tracer := opentracing.GlobalTracer()
	opentracing.SetGlobalTracer(tracer)

	// Create HTTP router
	mux := http.NewServeMux()

	// Register health check endpoint
	healthHandler := handlers.NewHealthHandler(db)
	mux.HandleFunc("/health", healthHandler.Health)

	// Create services
	productService := services.NewProductService(db)
	categoryService := services.NewCategoryService(db)
	variantService := services.NewVariantService(db)
	assetService := services.NewAssetService(db, cfg.AssetStoragePath)
	searchService := services.NewSearchService(db)
	importService := services.NewImportService(db, productService)
	exportService := services.NewExportService(db)

	// Create handlers
	productHandler := handlers.NewProductHandler(productService)
	categoryHandler := handlers.NewCategoryHandler(categoryService)
	variantHandler := handlers.NewVariantHandler(variantService, productService)
	assetHandler := handlers.NewAssetHandler(assetService, productService)
	searchHandler := handlers.NewSearchHandler(searchService)
	importHandler := handlers.NewImportHandler(importService)
	exportHandler := handlers.NewExportHandler(exportService)

	// Register import/export endpoints
	mux.HandleFunc("/api/v1/products/import", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			importHandler.Import(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/products/import/validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			importHandler.Validate(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/products/import/template", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			importHandler.GetTemplate(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/products/export", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			exportHandler.Export(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Register search endpoint
	mux.HandleFunc("/api/v1/products/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			searchHandler.Search(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Register product endpoints
	mux.HandleFunc("/api/v1/products", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			productHandler.Create(w, r)
		case http.MethodGet:
			productHandler.List(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Register category endpoints
	mux.HandleFunc("/api/v1/categories", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			categoryHandler.Create(w, r)
		case http.MethodGet:
			categoryHandler.List(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/categories/tree", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			categoryHandler.GetTree(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/categories/", func(w http.ResponseWriter, r *http.Request) {
		// Handle /api/v1/categories/{id}
		switch r.Method {
		case http.MethodGet:
			categoryHandler.Get(w, r)
		case http.MethodPut:
			categoryHandler.Update(w, r)
		case http.MethodDelete:
			categoryHandler.Delete(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Register variant endpoints
	mux.HandleFunc("/api/v1/variants", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			variantHandler.Create(w, r)
		case http.MethodGet:
			variantHandler.List(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/variants/generate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			variantHandler.Generate(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/variants/", func(w http.ResponseWriter, r *http.Request) {
		// Handle /api/v1/variants/{id}
		switch r.Method {
		case http.MethodGet:
			variantHandler.Get(w, r)
		case http.MethodPut:
			variantHandler.Update(w, r)
		case http.MethodDelete:
			variantHandler.Delete(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Register asset endpoints
	mux.HandleFunc("/api/v1/assets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			assetHandler.Upload(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/assets/", func(w http.ResponseWriter, r *http.Request) {
		// Check for specific sub-paths
		if strings.HasSuffix(r.URL.Path, "/download") && r.Method == http.MethodGet {
			assetHandler.Download(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/set-primary") && r.Method == http.MethodPost {
			assetHandler.SetPrimary(w, r)
			return
		}

		// Handle /api/v1/assets/{id}
		switch r.Method {
		case http.MethodGet:
			assetHandler.Get(w, r)
		case http.MethodPut:
			assetHandler.Update(w, r)
		case http.MethodDelete:
			assetHandler.Delete(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/products/", func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a variants sub-resource request
		if strings.HasSuffix(r.URL.Path, "/variants") && r.Method == http.MethodGet {
			variantHandler.ListByProduct(w, r)
			return
		}

		// Check if this is an assets sub-resource request
		if strings.HasSuffix(r.URL.Path, "/assets") && r.Method == http.MethodGet {
			assetHandler.ListByProduct(w, r)
			return
		}

		// Handle /api/v1/products/{id}
		switch r.Method {
		case http.MethodGet:
			productHandler.Get(w, r)
		case http.MethodPut:
			productHandler.Update(w, r)
		case http.MethodDelete:
			productHandler.Delete(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Apply middleware chain
	handler := middleware.RecoveryMiddleware(
		middleware.LoggingMiddleware(
			middleware.TracingMiddleware(
				middleware.TenantMiddleware(mux),
			),
		),
	)

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting PIM API server on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited properly")
}

func runMigrations(cfg *config.Config) {
	ctx := context.Background()

	log.Println("Running database migrations...")

	// Setup database
	db, err := config.SetupDatabase(ctx, cfg.DatabaseURL, cfg.LogLevel)
	if err != nil {
		log.Fatalf("Failed to setup database: %v", err)
	}
	defer config.CloseDatabase(db)

	// Run migrations
	if err := services.AutoMigrate(ctx, db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Println("Migrations completed successfully")
}
