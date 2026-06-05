package buzzhive

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var errRuntimeCacheMiss = errors.New("runtime cache miss")

type RuntimeCache struct {
	client *redis.Client
}

type adminSessionCacheValue struct {
	User      AppUser `json:"user"`
	ExpiresAt string  `json:"expires_at"`
}

type routeSessionCacheValue struct {
	ModelRouteID int64  `json:"model_route_id"`
	ExpiresAt    string `json:"expires_at"`
}

type keyCooldownCacheValue struct {
	ExpiresAt string `json:"expires_at"`
	RPDLike   bool   `json:"rpd_like"`
	Hits      int    `json:"hits"`
}

func OpenRuntimeCache(cfg RedisConfig) (*RuntimeCache, error) {
	if cfg.URL == "" && cfg.Addr == "" {
		return nil, nil
	}

	var (
		opts *redis.Options
		err  error
	)
	if cfg.URL != "" {
		opts, err = redis.ParseURL(cfg.URL)
		if err != nil {
			return nil, err
		}
	} else {
		opts = &redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		}
	}

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &RuntimeCache{client: client}, nil
}

func (c *RuntimeCache) Enabled() bool {
	return c != nil && c.client != nil
}

func (c *RuntimeCache) Close() error {
	if !c.Enabled() {
		return nil
	}
	return c.client.Close()
}

func (c *RuntimeCache) AdminSession(ctx context.Context, token string) (SessionUser, error) {
	if !c.Enabled() {
		return SessionUser{}, errRuntimeCacheMiss
	}
	raw, err := c.client.Get(ctx, adminSessionKey(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return SessionUser{}, errRuntimeCacheMiss
	}
	if err != nil {
		return SessionUser{}, err
	}

	var value adminSessionCacheValue
	if err := json.Unmarshal(raw, &value); err != nil {
		return SessionUser{}, err
	}
	expiresAt, err := time.Parse(time.RFC3339, value.ExpiresAt)
	if err != nil {
		return SessionUser{}, err
	}
	if !value.User.Valid || time.Now().After(expiresAt) {
		_ = c.DeleteAdminSession(ctx, token)
		return SessionUser{}, errRuntimeCacheMiss
	}
	return SessionUser{User: value.User, ExpiresAt: expiresAt}, nil
}

func (c *RuntimeCache) SetAdminSession(ctx context.Context, token string, sessionUser SessionUser) error {
	if !c.Enabled() {
		return nil
	}
	ttl := time.Until(sessionUser.ExpiresAt)
	if ttl <= 0 {
		return c.DeleteAdminSession(ctx, token)
	}
	raw, err := json.Marshal(adminSessionCacheValue{
		User:      sessionUser.User,
		ExpiresAt: sessionUser.ExpiresAt.Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	return c.client.Set(ctx, adminSessionKey(token), raw, ttl).Err()
}

func (c *RuntimeCache) DeleteAdminSession(ctx context.Context, token string) error {
	if !c.Enabled() {
		return nil
	}
	return c.client.Del(ctx, adminSessionKey(token)).Err()
}

func adminSessionKey(token string) string {
	return "bh:admin-session:" + sessionHash(token)
}

func (c *RuntimeCache) RouteSession(ctx context.Context, key string) (RouteSession, error) {
	if !c.Enabled() {
		return RouteSession{}, errRuntimeCacheMiss
	}
	raw, err := c.client.Get(ctx, routeSessionKey(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return RouteSession{}, errRuntimeCacheMiss
	}
	if err != nil {
		return RouteSession{}, err
	}

	var value routeSessionCacheValue
	if err := json.Unmarshal(raw, &value); err != nil {
		return RouteSession{}, err
	}
	expiresAt, err := time.Parse(time.RFC3339, value.ExpiresAt)
	if err != nil {
		return RouteSession{}, err
	}
	if time.Now().After(expiresAt) {
		_ = c.DeleteRouteSession(ctx, key)
		return RouteSession{}, errRuntimeCacheMiss
	}
	return RouteSession{
		ModelRouteID: value.ModelRouteID,
		ExpiresAt:    expiresAt,
	}, nil
}

func (c *RuntimeCache) SetRouteSession(ctx context.Context, key string, session RouteSession) error {
	if !c.Enabled() {
		return nil
	}
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return c.DeleteRouteSession(ctx, key)
	}
	raw, err := json.Marshal(routeSessionCacheValue{
		ModelRouteID: session.ModelRouteID,
		ExpiresAt:    session.ExpiresAt.Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	return c.client.Set(ctx, routeSessionKey(key), raw, ttl).Err()
}

func (c *RuntimeCache) DeleteRouteSession(ctx context.Context, key string) error {
	if !c.Enabled() {
		return nil
	}
	return c.client.Del(ctx, routeSessionKey(key)).Err()
}

func routeSessionKey(key string) string {
	return "bh:route-session:" + sessionHash(key)
}

func (c *RuntimeCache) KeyCooldown(ctx context.Context, key string) (time.Time, bool, error) {
	if !c.Enabled() {
		return time.Time{}, false, errRuntimeCacheMiss
	}
	raw, err := c.client.Get(ctx, keyCooldownKey(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return time.Time{}, false, errRuntimeCacheMiss
	}
	if err != nil {
		return time.Time{}, false, err
	}

	var value keyCooldownCacheValue
	if err := json.Unmarshal(raw, &value); err != nil {
		return time.Time{}, false, err
	}
	expiresAt, err := time.Parse(time.RFC3339, value.ExpiresAt)
	if err != nil {
		return time.Time{}, false, err
	}
	if time.Now().After(expiresAt) {
		return time.Time{}, false, errRuntimeCacheMiss
	}
	return expiresAt, value.RPDLike, nil
}

func (c *RuntimeCache) MarkKeyExhausted(ctx context.Context, key string, cooldown, rpdCooldown time.Duration) (time.Time, bool, error) {
	if !c.Enabled() {
		return time.Time{}, false, errRuntimeCacheMiss
	}
	if cooldown <= 0 {
		cooldown = time.Minute
	}
	if rpdCooldown <= 0 {
		rpdCooldown = time.Hour
	}

	var (
		hits    int
		rpdLike bool
	)
	if raw, err := c.client.Get(ctx, keyCooldownKey(key)).Bytes(); err == nil {
		var existing keyCooldownCacheValue
		if json.Unmarshal(raw, &existing) == nil {
			hits = existing.Hits
			rpdLike = existing.RPDLike
		}
	} else if err != nil && !errors.Is(err, redis.Nil) {
		return time.Time{}, false, err
	}

	hits++
	rpdLike = rpdLike || hits >= 2
	nextCooldown := cooldown
	if rpdLike {
		nextCooldown = rpdCooldown
	}
	expiresAt := time.Now().Add(nextCooldown)
	raw, err := json.Marshal(keyCooldownCacheValue{
		ExpiresAt: expiresAt.Format(time.RFC3339),
		RPDLike:   rpdLike,
		Hits:      hits,
	})
	if err != nil {
		return time.Time{}, false, err
	}
	return expiresAt, rpdLike, c.client.Set(ctx, keyCooldownKey(key), raw, keyCooldownRetention(nextCooldown, rpdCooldown)).Err()
}

func (c *RuntimeCache) DeleteKeyCooldown(ctx context.Context, key string) error {
	if !c.Enabled() {
		return nil
	}
	return c.client.Del(ctx, keyCooldownKey(key)).Err()
}

func keyCooldownKey(key string) string {
	return "bh:key-cooldown:" + sessionHash(key)
}

func keyCooldownRetention(cooldown, rpdCooldown time.Duration) time.Duration {
	retention := rpdCooldown
	if retention < cooldown {
		retention = cooldown
	}
	return retention + 5*time.Minute
}
