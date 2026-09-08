package middleware

import (
	"Broker_backend/services/integration/partnerapi/internal/config"
	"Broker_backend/services/integration/partnerapi/internal/transport/http/httperr"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"Broker_backend/shared/pkg/authz"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const processingIdempotencyValue = "processing"

type idempotencyResponse struct {
	StatusCode  int               `json:"status_code"`
	ContentType string            `json:"content_type"`
	Headers     map[string]string `json:"headers"`
	Body        []byte            `json:"body"`
}

func Idempotency(client *redis.Client, cfg config.IdempotencyConfig, logger *zap.Logger) fiber.Handler {
	if logger == nil {
		logger = zap.NewNop()
	}

	return func(c *fiber.Ctx) error {
		if !cfg.Enabled || !requiresIdempotency(c.Method()) {
			return c.Next()
		}

		if client == nil {
			return httperr.WriteServiceUnavailable(c, "idempotency store is not configured")
		}

		idempotencyKey := strings.TrimSpace(c.Get(cfg.Header))
		if idempotencyKey == "" {
			return httperr.WriteBadRequest(c, "idempotency key is required")
		}

		key := idempotencyStoreKey(cfg.LockPrefix, c, idempotencyKey)
		ctx := userContext(c)

		acquired, err := client.SetNX(ctx, key, processingIdempotencyValue, cfg.TTL).Result()
		if err != nil {
			logger.Warn("idempotency lock failed", zap.Error(err), zap.String("key", key))
			return httperr.WriteServiceUnavailable(c, "idempotency store unavailable")
		}

		if !acquired {
			return replayIdempotencyResponse(ctx, client, c, key)
		}

		err = c.Next()
		if err != nil {
			_ = client.Del(ctx, key).Err()
			return err
		}

		if c.Response().StatusCode() >= fiber.StatusInternalServerError {
			_ = client.Del(ctx, key).Err()
			return nil
		}

		body := c.Response().Body()
		if len(body) > cfg.MaxResponseBytes {
			_ = client.Del(ctx, key).Err()
			return nil
		}

		stored := idempotencyResponse{
			StatusCode:  c.Response().StatusCode(),
			ContentType: string(c.Response().Header.ContentType()),
			Headers: map[string]string{
				RequestIDHeader: CurrentRequestID(c),
			},
			Body: append([]byte(nil), body...),
		}

		raw, marshalErr := json.Marshal(stored)
		if marshalErr != nil {
			_ = client.Del(ctx, key).Err()
			return nil
		}

		_ = client.Set(ctx, key, raw, cfg.TTL).Err()
		return nil
	}
}

func replayIdempotencyResponse(ctx context.Context, client *redis.Client, c *fiber.Ctx, key string) error {
	raw, err := client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return httperr.WriteConflict(c, "idempotency request is already processing")
		}

		return httperr.WriteServiceUnavailable(c, "idempotency store unavailable")
	}

	if string(raw) == processingIdempotencyValue {
		return httperr.WriteConflict(c, "idempotency request is already processing")
	}

	var stored idempotencyResponse
	if err := json.Unmarshal(raw, &stored); err != nil {
		return httperr.WriteConflict(c, "idempotency request is already processing")
	}

	for key, value := range stored.Headers {
		c.Set(key, value)
	}

	if stored.ContentType != "" {
		c.Set(fiber.HeaderContentType, stored.ContentType)
	}

	return c.Status(stored.StatusCode).Send(stored.Body)
}

func requiresIdempotency(method string) bool {
	switch method {
	case fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete:
		return true
	default:
		return false
	}
}

func idempotencyStoreKey(prefix string, c *fiber.Ctx, idempotencyKey string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "idem"
	}

	owner := "anonymous"
	if principal, ok := authz.PrincipalFromContext(c.UserContext()); ok {
		owner = principal.UserID.String() + ":" + principal.DeviceID
	}

	return fmt.Sprintf("%s:%s:%s:%s:%s", prefix, owner, c.Method(), c.Path(), idempotencyKey)
}
