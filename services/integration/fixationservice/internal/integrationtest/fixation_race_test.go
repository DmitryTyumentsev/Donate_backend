package integrationtest

import (
	"Broker_backend/services/integration/fixationservice/internal/config"
	"Broker_backend/services/integration/fixationservice/internal/domain"
	"Broker_backend/services/integration/fixationservice/internal/repository/postgres"
	"Broker_backend/services/integration/fixationservice/internal/usecase"
	"Broker_backend/shared/pkg/authz"
	"Broker_backend/shared/pkg/authz/roles"
	"Broker_backend/shared/pkg/clock"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	goroutines       = 50
	randomPhone      = "8(999)999-99-99"
	fixationDuration = 24 * time.Hour * 60
	mockHashSecret   = "mock-hash-secret"
	fixFor           = "22222222-2222-2222-2222-222222222222"
	deviceID         = "22222222-2222-2222-2222-222222222222"
	fixationID       = "11111111-1111-1111-1111-111111111111"
	DSN              = "integration,app,public"
)

func TestNewFixation_RaceFixations_OneOfFixationsSuccessful(t *testing.T) {
	agencyID, projectID, userID := seedAgencyProjectUser(t)

	req := &usecase.FixationRequest{
		AgencyID:  agencyID,
		FixFor:    userID,
		FixBy:     userID,
		Phone:     randomPhone,
		ProjectID: projectID,
	}
	svc := newTestService(t)
	results := make(chan error, goroutines)
	start := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			<-start
			_, err := svc.NewFixation(contextWithMockPrincipal(agencyID, userID), req)
			results <- err
		}()
	}
	close(start)

	var ok, conflict, other int
	for i := 0; i < goroutines; i++ {
		switch err := <-results; {
		case err == nil:
			ok++
			t.Logf("i: %d, ok = %d, conflict = %d, other = %d", i, ok, conflict, other)
		case errors.Is(err, domain.ErrNotUnique) || errors.Is(err, domain.ErrFixationAlreadyExist):
			conflict++
			t.Logf("i: %d, err: %v, ok = %d, conflict = %d, other = %d", i, err, ok, conflict, other)
		case !errors.Is(err, domain.ErrNotUnique), !errors.Is(err, domain.ErrFixationAlreadyExist):
			other++
			t.Logf("i: %d, err: %v, ok = %d, conflict = %d, other = %d", i, err, ok, conflict, other)
		}
	}
	if ok != 1 {
		t.Fatalf("ok = %d, expected 1, conflict = %d, other = %d", ok, conflict, other)
	}
	if conflict != 49 {
		t.Fatalf("conflict = %d, expected 49, ok = %d, other = %d", conflict, ok, other)
	}
	if other != 0 {
		t.Fatalf("other = %d, expected 0, ok = %d, conflict = %d", other, ok, conflict)
	}
	t.Log("success")
}

func newTestService(t *testing.T) *usecase.Service {
	t.Helper()
	cfg := &config.Config{
		Business: config.BusinessConfig{
			HashSecret:       mockHashSecret,
			FixationDuration: 24 * 30 * time.Hour,
		},
	}
	testPool = poolWithMaxConns(
		t,
		goroutines,
		testPool.Config().ConnString(),
	)
	tx := postgres.NewTxManager(testPool)
	repo := postgres.NewRepository(tx)
	cl := clock.NewRealClock()

	return usecase.NewService(cfg, zap.NewNop(), cl, repo, tx)
}

func contextWithMockPrincipal(agencyID, userID uuid.UUID) context.Context {
	return authz.WithPrincipal(context.Background(), authz.Principal{
		AgencyID: agencyID,
		UserID:   userID,
		DeviceID: deviceID,
		Role:     roles.SalesManager,
	},
	)
}
