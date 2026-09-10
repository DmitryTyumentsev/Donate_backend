package usecase

import (
	"Broker_backend/services/integration/fixationservice/internal/config"
	"Broker_backend/services/integration/fixationservice/internal/domain/entity"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	randomPhone      = "8(999)999-99-99"
	fixationDuration = 24 * time.Hour * 60
	mockHashSecret   = "mock-hash-secret"
	fixFor           = "22222222-2222-2222-2222-222222222222"
	projectID        = "33333333-3333-3333-3333-333333333333"
	fixBy            = "22222222-2222-2222-2222-222222222222"
	agencyID         = "11111111-1111-1111-1111-111111111111"
)

var _ FixationRepository = (*mockRepo)(nil)
var _ TxManager = (*mockTxManager)(nil)
var _ Clock = (*mockClock)(nil)

type mockRepo struct {
	statusByProjectID           func(ctx context.Context, projectID uuid.UUID) (string, error)
	isUserIDInAgencyID          func(ctx context.Context, agencyID, userID uuid.UUID) (bool, error)
	fixationCurrent             func(ctx context.Context, phoneHash string, projectID uuid.UUID) (*entity.Fixation, error)
	insertNewFixation           func(ctx context.Context, f entity.Fixation) error
	insertAudit                 func(ctx context.Context, f entity.Fixation) error
	insertOutbox                func(ctx context.Context, f entity.Fixation) error
	updateFixationStatusExpired func(ctx context.Context, statusExpired entity.Status, id uuid.UUID) error
}

type mockTxManager struct {
	do func(ctx context.Context, fn func(ctx context.Context) error) error
}

type mockClock struct {
	clock func() time.Time
}

func newMockConfig() *config.Config {
	return &config.Config{
		Business: config.BusinessConfig{
			FixationDuration: fixationDuration,
			HashSecret:       mockHashSecret,
		},
	}
}

func (m *mockClock) Now() time.Time {
	return m.clock()
}

func (m *mockTxManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.do(ctx, fn)
}

func (m *mockRepo) StatusByProjectID(ctx context.Context, projectID uuid.UUID) (string, error) {
	return m.statusByProjectID(ctx, projectID)
}

func (m *mockRepo) IsUserIDInAgencyID(ctx context.Context, agencyID, userID uuid.UUID) (bool, error) {
	return m.isUserIDInAgencyID(ctx, agencyID, userID)
}

func (m *mockRepo) FixationCurrent(ctx context.Context, phoneHash string, projectID uuid.UUID) (*entity.Fixation, error) {
	return m.fixationCurrent(ctx, phoneHash, projectID)
}

func (m *mockRepo) InsertNewFixation(ctx context.Context, f entity.Fixation) error {
	return m.insertNewFixation(ctx, f)
}

func (m *mockRepo) InsertAudit(ctx context.Context, f entity.Fixation) error {
	return m.insertAudit(ctx, f)
}

func (m *mockRepo) InsertOutbox(ctx context.Context, f entity.Fixation) error {
	return m.insertOutbox(ctx, f)
}

func (m *mockRepo) UpdateFixationStatusExpired(ctx context.Context, statusExpired entity.Status, id uuid.UUID) error {
	return m.updateFixationStatusExpired(ctx, statusExpired, id)
}

func TestNewFixation_NoActiveFixation_InsertFixationAuditOutbox(t *testing.T) {
	repo := &mockRepo{
		statusByProjectID: func(context.Context, uuid.UUID) (string, error) {
			fmt.Println("implement statusByProjectID")
			return string(entity.StatusExpired), nil
		},
		isUserIDInAgencyID: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
			fmt.Println("implement isUserIDInAgencyID")
			return true, nil
		},
		fixationCurrent: func(context.Context, string, uuid.UUID) (*entity.Fixation, error) {
			fmt.Println("implement fixationCurrent")
			return &entity.Fixation{
				Status: entity.StatusNoRows,
			}, nil
		},
		insertNewFixation: func(context.Context, entity.Fixation) error {
			fmt.Println("implement insertNewFixation")
			return nil
		},
		insertAudit: func(ctx context.Context, f entity.Fixation) error {
			fmt.Println("implement insertAudit")
			return nil
		},
		insertOutbox: func(ctx context.Context, f entity.Fixation) error {
			fmt.Println("implement insertOutbox")
			return nil
		},
	}
	now := time.Now().UTC()
	mockNow := &mockClock{
		clock: func() time.Time {
			return now
		},
	}
	tx := &mockTxManager{do: func(ctx context.Context, fn func(context.Context) error) error {
		return fn(ctx)
	}}
	cfg := newMockConfig()

	svc := NewService(cfg, zap.NewNop(), mockNow, repo, tx)

	req := &FixationRequest{
		AgencyID:  uuid.New(),
		FixFor:    uuid.New(),
		FixBy:     uuid.New(),
		Phone:     randomPhone,
		ProjectID: uuid.New(),
	}
	got, err := svc.NewFixation(context.Background(), req)
	if err != nil {
		t.Fatalf("NewFixation failed, %v", err)
	}
	expected := entity.Fixation{
		AgencyID:  req.AgencyID,
		FixFor:    req.FixFor,
		FixBy:     req.FixBy,
		Status:    entity.StatusActive,
		ProjectID: req.ProjectID,
	}
	if got.AgencyID != expected.AgencyID {
		t.Fatalf("got.agencyID: %v != expected.AgencyID: %v", got.AgencyID, expected.AgencyID)
	}
	if got.FixFor != expected.FixFor {
		t.Fatalf("got.FixFor: %v != expected.FixFor: %v", got.FixFor, expected.FixFor)
	}
	if got.FixBy != expected.FixBy {
		t.Fatalf("got.FixBy: %v != expected.FixBy: %v", got.FixBy, expected.FixBy)
	}
	if got.Status != expected.Status {
		t.Fatalf("got.Status: %v != expected.Status: %v", got.Status, expected.Status)
	}
	if got.ProjectID != expected.ProjectID {
		t.Fatalf("got.ProjectID: %v != expected.ProjectID: %v", got.ProjectID, expected.ProjectID)
	}
	if got.PhoneHash == req.Phone || got.PhoneHash == "" {
		t.Fatalf("phone was not hashed, got.PhoneHash: %v phone: %v", got.PhoneHash, req.Phone)
	}
	if got.FixationID == uuid.Nil {
		t.Fatalf("got.FixationID: %v == uuid.Nil", got.FixationID)
	}
	if !got.ExpiresAt.Equal(got.FixedAt.Add(cfg.Business.FixationDuration)) {
		t.Fatalf("inaccurate timing: got.ExpiresAt: %v != got.FixedAt: %v", got.ExpiresAt, got.FixedAt)
	}

	//пишем какие сценарии могут быть:
	//позитивный(успешная новая фиксация),
	//позитивный(успешная фиксация быстрее воркера помечает старую фиксацию expired, создает новую),
	//негативный (фиксация уже создана и expires_at не истек)
	//негативный(req невалиден: project_id, fix_for, fix_by не существуют)
	//негативный(в agency_id нет юзеров fix_for, fix_by)
	//негативный(проверка транзакции - FixationCurrent выполнился, после этого соединение закрылось)
	//негативный(50 горутин одновременно пробуют сделать новую активную фиксацию)
	//готовим тестовые данные:
	//создаем мок структуры под каждый тест, внутри req/entity/pgerrcode
	//создаем в каждом тесте экземпляры моков, заполняем, вызываем оригинальный метод NewFixation передавая мок в ресивере и на вход метода

}
