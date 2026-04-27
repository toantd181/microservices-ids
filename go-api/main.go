package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

type NetworkFeatures struct {
	Features []float64
}

type PredictionResult struct {
	Prediction   string  `json:"prediction"`
	AnomalyScore float64 `json:"anomaly_score"`
	ModelVersion string  `json:"model_version"`
}

type GatewayResponse struct {
	Status       string
	Prediction   string
	AnomalyScore float64
	ModelVersion string
	ProcessedBy  string
}

type MLEngineClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewMLEngineClient(baseURL string) *MLEngineClient {
	return &MLEngineClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *MLEngineClient) Predict(features []float64) (*PredictionResult, error) {
	payload := map[string][]float64{"features": features}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.New("failed to serialize payload")
	}

	req, err := http.NewRequest(
		http.MethodPost,
		c.baseURL+"/predict",
		bytes.NewBuffer(body),
	)

	if err != nil {
		return nil, errors.New("failed to build request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("ML Engine request failed", "error", err)
		return nil, errors.New("ML Engine unreachable")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New("Failed to read ML Engine response")
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("ML Engine returned error",
			"status", resp.StatusCode, "response", string(respBody))
		return nil, errors.New("ML engine inference failed")
	}

	var result PredictionResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, errors.New("failed to parse ML engine response")
	}
	return &result, nil
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}

func healthHandler(mlClient *MLEngineClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		resp, err := mlClient.httpClient.Get(mlClient.baseURL + "/health")
		if err != nil || resp.StatusCode != http.StatusOK {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"reason": "ML engine unreachable",
			})
			return
		}
		defer resp.Body.Close()

		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"ml_engine": "reachable",
			"gateway":   "ok",
		})
	}
}

func predictHandler(mlClient *MLEngineClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input NetworkFeatures
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid request: features array is required",
			})
			return
		}

		if len(input.Features) != 6 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "exactly 6 features required",
			})
			return
		}

		slog.Info("forwarding to ML engine",
			"features_count", len(input.Features))

		result, err := mlClient.Predict(input.Features)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{
				"error": err.Error(),
			})
			return
		}

		slog.Info("prediction complete",
			"prediction", result.Prediction,
			"score", result.AnomalyScore,
			"model_version", result.ModelVersion,
		)

		c.JSON(http.StatusOK, GatewayResponse{
			Status:       "success",
			Prediction:   result.Prediction,
			AnomalyScore: result.AnomalyScore,
			ModelVersion: result.ModelVersion,
			ProcessedBy:  "go-api-gateway",
		})
	}
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	mlEngineURL := getEnv("ML_ENGINE_URL", "http://localhost:8000")
	port := getEnv("PORT", "8080")
	environment := getEnv("ENVIRONMENT", "development")

	if environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	mlClient := NewMLEngineClient(mlEngineURL)

	router := gin.New()
	router.Use(requestLogger())
	router.Use(securityHeaders())
	router.Use(gin.Recovery())

	router.GET("/health", healthHandler(mlClient))
	router.POST("/predict", predictHandler(mlClient))

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("gateway starting", "port", port, "environment", environment)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
	}

	slog.Info("gateway stopped")
}
