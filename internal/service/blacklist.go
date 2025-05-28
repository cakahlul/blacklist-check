package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"blacklist-check/internal/store"

	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// BlacklistService handles blacklist checking business logic
type BlacklistService struct {
	db    *sqlx.DB
	redis *redis.Client
	store store.BlacklistStore
	log   *zap.Logger
}

// NewBlacklistService creates a new blacklist service
func NewBlacklistService(db *sqlx.DB, redis *redis.Client, store store.BlacklistStore, log *zap.Logger) *BlacklistService {
	return &BlacklistService{
		db:    db,
		redis: redis,
		store: store,
		log:   log,
	}
}

// CheckRequest represents a blacklist check request
type CheckRequest struct {
	Name       string
	NIK        string
	BirthPlace string
	BirthDate  time.Time
}

// CheckResult represents the result of a blacklist check
type CheckResult struct {
	Blacklisted bool
	Details     string
	MatchType   string
}

// CheckBlacklist checks if a person is blacklisted
func (s *BlacklistService) CheckBlacklist(ctx context.Context, req CheckRequest) (*CheckResult, error) {
	// Generate cache key based on request type
	var cacheKey string
	if req.NIK != "" {
		cacheKey = fmt.Sprintf("blacklist:nik:%s", req.NIK)
	} else {
		cacheKey = fmt.Sprintf("blacklist:name:%s:%s:%s", 
			req.Name, 
			req.BirthPlace, 
			req.BirthDate.Format("2006-01-02"))
	}

	// Try to get from cache first
	cachedResult, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var result CheckResult
		if err := json.Unmarshal([]byte(cachedResult), &result); err == nil {
			s.log.Info("Cache hit for blacklist check",
				zap.String("cache_key", cacheKey),
				zap.String("match_type", result.MatchType))
			return &result, nil
		}
	}

	// If not in cache, check database
	var result CheckResult

	// First try exact NIK match if provided
	if req.NIK != "" {
		record, err := s.store.GetByNIK(ctx, req.NIK)
		if err != nil {
			return nil, fmt.Errorf("error checking NIK: %w", err)
		}
		if record != nil {
			result = CheckResult{
				Blacklisted: true,
				Details:     record.Reason,
				MatchType:   "exact_nik",
			}
			s.log.Info("Found blacklist record by NIK",
				zap.String("nik", req.NIK),
				zap.String("match_type", result.MatchType))
		}
	}

	// If no NIK match, try fuzzy matching with birth place and birth date
	if !result.Blacklisted {
		records, err := s.store.GetByFuzzyMatch(ctx, req.Name, &req.BirthPlace, &req.BirthDate)
		if err != nil {
			return nil, fmt.Errorf("error searching by fuzzy match: %w", err)
		}

		if len(records) > 0 {
			// Check if any record matches both birth place and birth date
			for _, record := range records {
				if record.BirthPlace == req.BirthPlace && record.BirthDate.Equal(req.BirthDate) {
					result = CheckResult{
						Blacklisted: true,
						Details:     record.Reason,
						MatchType:   "fuzzy_full_match",
					}
					s.log.Info("Found blacklist record by fuzzy full match",
						zap.String("name", req.Name),
						zap.String("birth_place", req.BirthPlace),
						zap.Time("birth_date", req.BirthDate),
						zap.String("match_type", result.MatchType))
					break
				}
			}

			// If no full match found, try partial match with birth date only
			if !result.Blacklisted {
				for _, record := range records {
					if record.BirthDate.Equal(req.BirthDate) {
						result = CheckResult{
							Blacklisted: true,
							Details:     record.Reason,
							MatchType:   "fuzzy_date_match",
						}
						s.log.Info("Found blacklist record by fuzzy date match",
							zap.String("name", req.Name),
							zap.Time("birth_date", req.BirthDate),
							zap.String("match_type", result.MatchType))
						break
					}
				}
			}
		}

		// If still no match found
		if !result.Blacklisted {
			result = CheckResult{
				Blacklisted: false,
				MatchType:   "no_match",
			}
			s.log.Info("No blacklist record found",
				zap.String("name", req.Name),
				zap.String("match_type", result.MatchType))
		}
	}

	// Cache the result
	resultJSON, err := json.Marshal(result)
	if err != nil {
		s.log.Error("Error marshaling result for cache",
			zap.Error(err))
	} else {
		err = s.redis.Set(ctx, cacheKey, resultJSON, 24*time.Hour).Err()
		if err != nil {
			s.log.Error("Error caching result",
				zap.Error(err))
		}
	}

	return &result, nil
} 