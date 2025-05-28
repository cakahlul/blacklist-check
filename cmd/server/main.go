package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"blacklist-check/internal/api"
	"blacklist-check/internal/service"
	"blacklist-check/internal/store"
	"blacklist-check/pkg/config"
	"blacklist-check/pkg/log"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/dig"
	"go.uber.org/zap"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	blacklistChecksTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "blacklist_checks_total",
			Help: "Total number of blacklist checks",
		},
		[]string{"match_type", "result"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(blacklistChecksTotal)
}

func main() {
	container := dig.New()

	// Provide configuration
	container.Provide(func() (*config.Config, error) {
		return config.Load()
	})

	// Provide logger
	container.Provide(func(cfg *config.Config) (*zap.Logger, error) {
		return log.NewLogger(cfg.Server.LogLevel)
	})

	// Provide database connection
	container.Provide(func(cfg *config.Config) (*sqlx.DB, error) {
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
			cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode)
		return sqlx.Connect("postgres", dsn)
	})

	// Provide Redis client
	container.Provide(func(cfg *config.Config) *redis.Client {
		return redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
	})

	// Provide store
	container.Provide(store.NewBlacklistStore)

	// Provide service
	container.Provide(service.NewBlacklistService)

	// Provide handler
	container.Provide(api.NewHandler)

	// Start server
	err := container.Invoke(func(
		cfg *config.Config,
		log *zap.Logger,
		handler *api.Handler,
	) error {
		r := chi.NewRouter()

		// Middleware
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)
		r.Use(middleware.RequestID)
		r.Use(middleware.RealIP)
		r.Use(middleware.Timeout(60 * time.Second))

		// Prometheus middleware
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				start := time.Now()
				ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
				next.ServeHTTP(ww, r)
				duration := time.Since(start).Seconds()

				httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", ww.Status())).Inc()
				httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
			})
		})

		// Routes
		r.Get("/healthz", handler.HealthCheck)
		r.Post("/api/v1/blacklist", handler.CheckBlacklist)
		r.Get("/metrics", promhttp.Handler())

		// Start server
		srv := &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
			Handler: r,
		}

		// Server run context
		serverCtx, serverStopCtx := context.WithCancel(context.Background())

		// Listen for syscall signals for process to interrupt/quit
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		go func() {
			<-sig

			// Shutdown signal with grace period of 30 seconds
			shutdownCtx, shutdownCancel := context.WithTimeout(serverCtx, 30*time.Second)
			defer shutdownCancel()

			go func() {
				<-shutdownCtx.Done()
				if shutdownCtx.Err() == context.DeadlineExceeded {
					log.Fatal("graceful shutdown timed out.. forcing exit.")
				}
			}()

			// Trigger graceful shutdown
			err := srv.Shutdown(shutdownCtx)
			if err != nil {
				log.Fatal(err.Error())
			}
			serverStopCtx()
		}()

		// Run the server
		log.Info("Starting server", zap.Int("port", cfg.Server.Port))
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err.Error())
		}

		// Wait for server context to be stopped
		<-serverCtx.Done()

		return nil
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
} 