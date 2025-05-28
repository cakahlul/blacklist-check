package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"blacklist-check/internal/service"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var (
	nikRegex = regexp.MustCompile(`^\d{16}$`)

	blacklistChecksTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "blacklist_checks_total",
			Help: "Total number of blacklist checks",
		},
		[]string{"match_type", "result"},
	)
)

func init() {
	prometheus.MustRegister(blacklistChecksTotal)
}

// Handler handles HTTP requests
type Handler struct {
	service *service.BlacklistService
	log     *zap.Logger
}

// NewHandler creates a new handler
func NewHandler(service *service.BlacklistService, log *zap.Logger) *Handler {
	return &Handler{
		service: service,
		log:     log,
	}
}

// checkRequest represents the request body for blacklist check
type checkRequest struct {
	Name       string     `json:"name"`
	NIK        *string    `json:"nik,omitempty"`
	BirthPlace *string    `json:"birth_place,omitempty"`
	BirthDate  *time.Time `json:"birth_date,omitempty"`
}

// checkResponse represents the response body for blacklist check
type checkResponse struct {
	Blacklisted bool   `json:"blacklisted"`
	Details     string `json:"details,omitempty"`
	MatchType   string `json:"match_type"`
}

// CheckBlacklist handles blacklist check requests
func (h *Handler) CheckBlacklist(w http.ResponseWriter, r *http.Request) {
	var req checkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Error decoding request body", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate name
	if len(req.Name) < 3 {
		h.log.Error("Name too short", zap.String("name", req.Name))
		http.Error(w, "Name must be at least 3 characters long", http.StatusBadRequest)
		return
	}

	// Validate NIK if provided
	if req.NIK != nil && !nikRegex.MatchString(*req.NIK) {
		h.log.Error("Invalid NIK format", zap.String("nik", *req.NIK))
		http.Error(w, "NIK must be a 16-digit number", http.StatusBadRequest)
		return
	}

	// Create service request
	serviceReq := service.CheckRequest{
		Name: req.Name,
	}
	if req.NIK != nil {
		serviceReq.NIK = *req.NIK
	}
	if req.BirthPlace != nil {
		serviceReq.BirthPlace = *req.BirthPlace
	}
	if req.BirthDate != nil {
		serviceReq.BirthDate = *req.BirthDate
	}

	// Check blacklist
	result, err := h.service.CheckBlacklist(r.Context(), serviceReq)
	if err != nil {
		h.log.Error("Error checking blacklist", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Record metrics
	blacklistChecksTotal.WithLabelValues(result.MatchType, fmt.Sprintf("%v", result.Blacklisted)).Inc()

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(checkResponse{
		Blacklisted: result.Blacklisted,
		Details:     result.Details,
		MatchType:   result.MatchType,
	})
}

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
} 