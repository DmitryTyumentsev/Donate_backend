package grpc

import (
	fixationv1 "Broker_backend/gen/fixation/v1"
	"Broker_backend/services/integration/fixationservice/internal/domain/entity"
	"Broker_backend/services/integration/fixationservice/internal/usecase"
	"Broker_backend/shared/pkg/authz"
	"Broker_backend/shared/pkg/authz/roles"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	randomPhone      = "8(999)999-99-99"
	fixationDuration = 24 * time.Hour * 60
	mockHashSecret   = "mock-hash-secret"
	fixFor           = "22222222-2222-2222-2222-222222222222"
	projectID        = "33333333-3333-3333-3333-333333333333"
	fixBy            = "22222222-2222-2222-2222-222222222222"
	agencyID         = "11111111-1111-1111-1111-111111111111"
	deviceID         = "22222222-2222-2222-2222-222222222222"
	fixationID       = "11111111-1111-1111-1111-111111111111"
)

var _ FixationService = (*mockService)(nil)

type mockService struct {
	newFixation func(ctx context.Context, req *usecase.FixationRequest) (*entity.Fixation, error)
}

func newMockService(t *testing.T) *mockService {
	return &mockService{
		newFixation: func(ctx context.Context, req *usecase.FixationRequest) (*entity.Fixation, error) {
			t.Helper()
			t.Log("new fixation was called")
			now := time.Now().UTC()
			return &entity.Fixation{
				FixationID: uuid.MustParse(fixationID),
				FixedAt:    now,
				ExpiresAt:  now.Add(fixationDuration),
			}, nil
		},
	}
}

func (m *mockService) NewFixation(ctx context.Context, req *usecase.FixationRequest) (*entity.Fixation, error) {
	return m.newFixation(ctx, req)
}

func TestNewFixation_ContextWithEmptyPrincipal_CodeUnauthenticated(t *testing.T) {
	h := NewHandler(fixationv1.UnimplementedFixationServiceServer{}, newMockService(t), zap.NewNop())
	req := &fixationv1.NewFixationRequest{
		FixFor:    fixFor,
		Phone:     randomPhone,
		ProjectId: projectID,
	}
	resp, err := h.NewFixation(context.Background(), req)
	if err != nil {
		if status.Code(err) == codes.Unauthenticated {
			t.Log("successful, principal not found")
		} else {
			t.Fatalf("error is not unauthenticated, err: %v, resp: %s", err, resp)
		}
	}
	if err == nil || resp != nil {
		t.Fatalf("other error or success fixation, resp: %v, err: %v", resp, err)
	}
}

func TestNewFixation_PrincipalInContext_ServiceCalled(t *testing.T) {
	h := NewHandler(fixationv1.UnimplementedFixationServiceServer{}, newMockService(t), zap.NewNop())
	req := &fixationv1.NewFixationRequest{
		FixFor:    fixFor,
		Phone:     randomPhone,
		ProjectId: projectID,
	}
	ctx := authz.WithPrincipal(context.Background(), authz.Principal{
		AgencyID: uuid.MustParse(agencyID),
		UserID:   uuid.MustParse(fixBy),
		DeviceID: deviceID,
		Role:     roles.SalesManager,
	},
	)
	gotResp, err := h.NewFixation(ctx, req)
	if err != nil {
		t.Fatalf("err: %v, resp: %s", err, gotResp)
	}
	if gotResp.FixationId != fixationID {
		t.Fatalf("fixation was not called, gotResp: %v", gotResp)
	}
	t.Log("successful, principal in context")
}
