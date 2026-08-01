package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/richxcame/ride-hailing/pkg/cache"
	"github.com/richxcame/ride-hailing/pkg/logger"
	"github.com/richxcame/ride-hailing/pkg/models"
	"go.uber.org/zap"
)

// CachedService wraps Service with Redis caching for hot-path queries.
type CachedService struct {
	*Service
	cache *cache.Manager
}

// NewCachedService creates a new CachedService.
func NewCachedService(svc *Service, cacheManager *cache.Manager) *CachedService {
	return &CachedService{Service: svc, cache: cacheManager}
}

// GetProfile retrieves user profile with cache-aside pattern.
func (cs *CachedService) GetProfile(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	cacheKey := cache.Keys.User(userID.String())

	var user models.User
	if err := cs.cache.Get(ctx, cacheKey, &user); err == nil {
		return &user, nil // Cache hit
	}

	// Cache miss - fetch from database
	result, err := cs.Service.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Cache the result (non-blocking)
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := cs.cache.Set(cacheCtx, cacheKey, result, cache.TTL.Medium()); err != nil {
			logger.Warn("Failed to cache user profile", zap.Error(err), zap.String("user_id", userID.String()))
		}
	}()

	return result, nil
}

// UpdateProfile updates user profile and invalidates cache.
func (cs *CachedService) UpdateProfile(ctx context.Context, userID uuid.UUID, updates *models.User) (*models.User, error) {
	result, err := cs.Service.UpdateProfile(ctx, userID, updates)
	if err != nil {
		return nil, err
	}

	// Invalidate cache
	cacheKey := cache.Keys.User(userID.String())
	if err := cs.cache.Delete(ctx, cacheKey); err != nil {
		logger.Warn("Failed to invalidate user cache", zap.Error(err), zap.String("user_id", userID.String()))
	}

	return result, nil
}
