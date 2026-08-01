package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/richxcame/ride-hailing/pkg/common"
	"github.com/richxcame/ride-hailing/pkg/config"
	"github.com/richxcame/ride-hailing/pkg/database"
	apperrors "github.com/richxcame/ride-hailing/pkg/errors"
	"github.com/richxcame/ride-hailing/pkg/jwtkeys"
	"github.com/richxcame/ride-hailing/pkg/logger"
	"github.com/richxcame/ride-hailing/pkg/middleware"
	"github.com/richxcame/ride-hailing/pkg/swagger"
	"github.com/richxcame/ride-hailing/pkg/tracing"
	"go.uber.org/zap"
)

// Option configures the server
type Option func(*Server)

// Server encapsulates common service startup
type Server struct {
	Name           string
	Version        string
	Config         *config.Config
	Router         *gin.Engine
	DB             *pgxpool.Pool
	JWTProvider    jwtkeys.KeyProvider
	httpServer     *http.Server
	
	port           string
	useDB          bool
	useJWT         bool
	cancelRotation context.CancelFunc
}

// WithPort sets a custom port
func WithPort(port string) Option {
	return func(s *Server) {
		s.port = port
	}
}

// WithoutDatabase disables database initialization
func WithoutDatabase() Option {
	return func(s *Server) {
		s.useDB = false
	}
}

// WithoutJWT disables JWT provider initialization
func WithoutJWT() Option {
	return func(s *Server) {
		s.useJWT = false
	}
}

// New creates a new server
func New(name, version string, opts ...Option) (*Server, error) {
	s := &Server{
		Name:    name,
		Version: version,
		useDB:   true,
		useJWT:  true,
	}

	for _, opt := range opts {
		opt(s)
	}

	cfg, err := config.Load(name)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %v", err)
	}
	s.Config = cfg

	if s.port == "" {
		s.port = cfg.Server.Port
	}

	// Initialize logger
	if err := logger.Init(cfg.Server.Environment); err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %v", err)
	}

	logger.Info("Starting service",
		zap.String("service", name),
		zap.String("version", version),
		zap.String("environment", cfg.Server.Environment),
	)

	// Initialize Sentry
	sentryConfig := apperrors.DefaultSentryConfig()
	sentryConfig.ServerName = name
	sentryConfig.Release = version
	if err := apperrors.InitSentry(sentryConfig); err != nil {
		logger.Warn("Failed to initialize Sentry", zap.Error(err))
	} else {
		logger.Info("Sentry error tracking initialized successfully")
	}

	// Initialize OpenTelemetry tracer
	tracerEnabled := os.Getenv("OTEL_ENABLED") == "true"
	if tracerEnabled {
		tracerCfg := tracing.Config{
			ServiceName:    name,
			ServiceVersion: version,
			Environment:    cfg.Server.Environment,
			OTLPEndpoint:   os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
			Enabled:        true,
		}

		if os.Getenv("OTEL_SERVICE_NAME") != "" {
			tracerCfg.ServiceName = os.Getenv("OTEL_SERVICE_NAME")
		}

		_, err := tracing.InitTracer(tracerCfg, logger.Get())
		if err != nil {
			logger.Warn("Failed to initialize tracer", zap.Error(err))
		} else {
			logger.Info("OpenTelemetry tracing initialized successfully")
		}
	}

	// Initialize Database
	if s.useDB {
		pool, err := database.NewPostgresPool(&cfg.Database, cfg.Timeout.DatabaseQueryTimeout)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to database: %v", err)
		}
		s.DB = pool
		logger.Info("Connected to database")
	}

	// Initialize JWT Key Manager
	rootCtx, cancelRotation := context.WithCancel(context.Background())
	s.cancelRotation = cancelRotation

	if s.useJWT {
		keyManager, err := jwtkeys.NewManagerFromConfig(rootCtx, cfg.JWT, false)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize JWT key manager: %v", err)
		}
		keyManager.StartAutoRotation(rootCtx)
		s.JWTProvider = keyManager
	}

	// Setup Gin router
	if cfg.Server.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.NoRoute(common.NoRouteHandler())
	router.NoMethod(common.NoMethodHandler())
	router.Use(middleware.RecoveryWithSentry())
	router.Use(middleware.SentryMiddleware())
	router.Use(middleware.CorrelationID())
	router.Use(middleware.RequestTimeout(&cfg.Timeout))
	router.Use(middleware.RequestLogger(name))
	router.Use(middleware.CORS())
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.MaxBodySize(10 << 20))
	router.Use(middleware.SanitizeRequest())
	router.Use(middleware.Metrics(name))

	if tracerEnabled {
		router.Use(middleware.TracingMiddleware(name))
	}

	router.Use(middleware.ErrorHandler())
	
	s.Router = router

	return s, nil
}

// RegisterRoutes lets services register their routes and adds default health/metric routes
func (s *Server) RegisterRoutes(fn func(*gin.Engine, jwtkeys.KeyProvider)) {
	s.Router.GET("/healthz", common.HealthCheck(s.Name, s.Version))
	s.Router.GET("/health/live", common.LivenessProbe(s.Name, s.Version))
	
	healthChecks := make(map[string]func() error)
	if s.DB != nil {
		healthChecks["database"] = func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return s.DB.Ping(ctx)
		}
	}
	
	s.Router.GET("/health/ready", common.ReadinessProbe(s.Name, s.Version, healthChecks))
	s.Router.GET("/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": s.Name,
			"version": s.Version,
		})
	})
	s.Router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	
	swagger.RegisterRoutes(s.Router)
	
	if fn != nil {
		fn(s.Router, s.JWTProvider)
	}
}

// Run starts the HTTP server with graceful shutdown
func (s *Server) Run() error {
	s.httpServer = &http.Server{
		Addr:         ":" + s.port,
		Handler:      s.Router,
		ReadTimeout:  time.Duration(s.Config.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.Config.Server.WriteTimeout) * time.Second,
	}

	go func() {
		logger.Info("Server starting", zap.String("port", s.port))
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	
	if s.cancelRotation != nil {
		s.cancelRotation()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
		return err
	}

	if s.DB != nil {
		database.Close(s.DB)
	}
	
	apperrors.Flush(2 * time.Second)
	logger.Sync()

	logger.Info("Server stopped")
	return nil
}
