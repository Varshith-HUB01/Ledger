package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"

	// PROMETHEUS IMPORTS
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const codeLength = 7
const cacheTTL = 24 * time.Hour

// PROMETHEUS METRICS
var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"method", "path"},
	)
)

type App struct {
	db    *pgxpool.Pool
	redis *redis.Client
	amqp  *amqp.Channel
}

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	ShortCode string `json:"short_code"`
	ShortURL  string `json:"short_url"`
}

type ClickEvent struct {
	ShortCode string    `json:"short_code"`
	IPAddress string    `json:"ip_address"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	ctx := context.Background()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/ledger?sslmode=disable"
	}
	dbPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer dbPool.Close()

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6379"
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisURL,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Unable to connect to Redis: %v\n", err)
	}

	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@localhost:5672/"
	}

	var conn *amqp.Connection
	for i := 0; i < 5; i++ {
		conn, err = amqp.Dial(rabbitURL)
		if err == nil {
			break
		}
		log.Printf("RabbitMQ not ready yet, retrying in 2 seconds...")
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer ch.Close()

	_, err = ch.QueueDeclare("click_events", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Failed to declare a queue: %v", err)
	}

	app := &App{
		db:    dbPool,
		redis: redisClient,
		amqp:  ch,
	}

	r := chi.NewRouter()

	// ADD CORS MIDDLEWARE HERE BELOW THE ROUTER INITIALIZATION
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	// PROMETHEUS MIDDLEWARE (Tracks latency and status codes)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			duration := time.Since(start).Seconds()
			status := strconv.Itoa(ww.Status())

			// Don't track the /metrics endpoint itself
			if r.URL.Path != "/metrics" {
				httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
				httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
			}
		})
	})

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// EXPOSE THE METRICS ENDPOINT
	r.Handle("/metrics", promhttp.Handler())

	r.Group(func(r chi.Router) {
		r.Use(app.rateLimitMiddleware)
		r.Post("/shorten", app.handleShorten)
		r.Get("/{code}", app.handleRedirect)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting Ledger API on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func generateShortCode() string {
	b := make([]byte, codeLength)
	for i := range b {
		b[i] = base62Chars[rand.Intn(len(base62Chars))]
	}
	return string(b)
}

func (a *App) handleShorten(w http.ResponseWriter, r *http.Request) {
	var req ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	shortCode := generateShortCode()
	ctx := r.Context()

	query := `INSERT INTO urls (short_code, original_url) VALUES ($1, $2) RETURNING short_code`
	err := a.db.QueryRow(ctx, query, shortCode, req.URL).Scan(&shortCode)
	if err != nil {
		log.Printf("DB Error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	err = a.redis.Set(ctx, shortCode, req.URL, cacheTTL).Err()
	if err != nil {
		log.Printf("Redis Set Error: %v", err)
	}

	resp := ShortenResponse{
		ShortCode: shortCode,
		ShortURL:  fmt.Sprintf("http://%s/%s", r.Host, shortCode),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (a *App) handleRedirect(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		http.Error(w, "Code is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	cachedURL, err := a.redis.Get(ctx, code).Result()
	var redirectURL string

	if err == nil {
		redirectURL = cachedURL
	} else {
		query := `SELECT original_url FROM urls WHERE short_code = $1`
		err = a.db.QueryRow(ctx, query, code).Scan(&redirectURL)
		if err != nil {
			if err.Error() == "no rows in result set" {
				http.Error(w, "URL not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		a.redis.Set(ctx, code, redirectURL, cacheTTL)
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	go func(c, i string) {
		event := ClickEvent{
			ShortCode: c,
			IPAddress: i,
			Timestamp: time.Now(),
		}
		body, _ := json.Marshal(event)

		pubErr := a.amqp.PublishWithContext(context.Background(),
			"",
			"click_events",
			false,
			false,
			amqp.Publishing{
				ContentType: "application/json",
				Body:        body,
			})
		if pubErr != nil {
			log.Printf("Failed to publish click event: %v", pubErr)
		}
	}(code, ip)

	http.Redirect(w, r, redirectURL, http.StatusMovedPermanently)
}

func (a *App) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		limit := int64(10)
		window := 60 * time.Second
		key := fmt.Sprintf("rate_limit:%s", ip)

		count, err := a.redis.Incr(ctx, key).Result()
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if count == 1 {
			a.redis.Expire(ctx, key, window)
		}

		if count > limit {
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "Rate limit exceeded"})
			return
		}

		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", limit-count))
		next.ServeHTTP(w, r)
	})
}
