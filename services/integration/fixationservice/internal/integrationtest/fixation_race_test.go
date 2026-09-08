package integrationtest

import (
	"Broker_backend/services/integration/fixationservice/internal/config"
	"Broker_backend/services/integration/fixationservice/internal/domain"
	"Broker_backend/services/integration/fixationservice/internal/repository/postgres"
	"Broker_backend/services/integration/fixationservice/internal/usecase"
	"Broker_backend/shared/pkg/authz"
	"Broker_backend/shared/pkg/authz/roles"
	"Broker_backend/shared/pkg/clock"
	"Broker_backend/shared/pkg/dbtest"
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

//const (
//	goroutines       = 50
//	testPhone        = "8(999)111-22-33"
//	fixationDuration = 60 * 24 * time.Hour
//	hashSecret       = "integration-test-secret"
//)
//
//// realClock — настоящие часы. В интеграционном тесте подменять время
//// незачем: проверяется поведение базы, а не арифметика сроков.
//type realClock struct{}
//
//func (realClock) Now() time.Time { return time.Now().UTC() }
//
//func TestNewFixation_50Goroutines_ExactlyOneSucceeds(t *testing.T) {
//	agencyID, projectID, userID := seedAgencyProjectUser(t)
//
//	// ── Настоящий стек целиком. Ни одного мока ──────────────────────
//	txManager := postgres.NewTxManager(testPool)
//	repo := postgres.NewRepository(txManager)
//
//	cfg := &config.Config{}
//	cfg.Business.FixationDuration = fixationDuration
//	cfg.Business.HashSecret = hashSecret
//
//	svc := usecase.NewService(cfg, zap.NewNop(), realClock{}, repo, txManager)
//
//	req := &usecase.FixationRequest{
//		AgencyID:  agencyID,
//		FixFor:    userID,
//		FixBy:     userID,
//		ProjectID: projectID,
//		Phone:     testPhone,
//	}
//
//	// ── Запуск ──────────────────────────────────────────────────────
//	//
//	// start — стартовый барьер. Все горутины блокируются на чтении из
//	// незакрытого канала. close(start) отпускает их ОДНОВРЕМЕННО:
//	// чтение из закрытого канала возвращается сразу и у всех.
//	//
//	// Без барьера первые горутины успеют отработать раньше, чем
//	// запустятся последние. Гонки не случится, и тест позеленеет
//	// даже на сломанном индексе.
//	start := make(chan struct{})
//
//	// Буфер на goroutines — иначе горутины залипнут на отправке,
//	// пока основная не начнёт читать, и Wait никогда не вернётся.
//	results := make(chan error, goroutines)
//
//	var wg sync.WaitGroup
//	for i := 0; i < goroutines; i++ {
//		wg.Add(1)
//		go func() {
//			defer wg.Done()
//			<-start // ждём здесь
//			_, err := svc.NewFixation(context.Background(), req)
//			results <- err
//		}()
//	}
//
//	close(start)
//	wg.Wait() // без него тест завершится раньше горутин
//	close(results)
//
//	// ── Разбор ──────────────────────────────────────────────────────
//	//
//	// t.Errorf зовём только здесь, в основной горутине. Из горутин
//	// после завершения теста это даёт панику всего прогона.
//	var success, conflict int
//	var unexpected []error
//
//	for err := range results {
//		switch {
//		case err == nil:
//			success++
//		case errors.Is(err, domain.ErrNotUnique),
//			errors.Is(err, domain.ErrFixationAlreadyExist):
//			conflict++
//		default:
//			unexpected = append(unexpected, err)
//		}
//	}
//
//	// Проверяем КОЛИЧЕСТВО, а не «победила нулевая горутина»:
//	// порядок недетерминирован, выиграть может любая.
//	if success != 1 {
//		t.Errorf("успехов: %d, ожидали ровно 1 (конфликтов %d)", success, conflict)
//	}
//	if conflict != goroutines-1 {
//		t.Errorf("конфликтов: %d, ожидали %d", conflict, goroutines-1)
//	}
//	for _, err := range unexpected {
//		t.Errorf("неожиданная ошибка вместо конфликта: %v", err)
//	}
//
//	// Контрольная проверка в самой базе. Главная в этом тесте:
//	// выше проверялось, что вернул код, здесь — что реально записано.
//	phoneHash := usecase.HashPhone(testPhone, hashSecret) // ← подставь свою функцию
//	if n := countActiveFixations(t, phoneHash, projectID); n != 1 {
//		t.Fatalf("активных фиксаций в базе: %d, ожидали 1", n)
//	}
//}
//
//// Второй сценарий, который тоже проверяется только на живой базе:
//// протухшая фиксация не должна мешать создать новую.
//func TestNewFixation_ExpiredFixation_DoesNotBlockNew(t *testing.T) {
//	agencyID, projectID, userID := seedAgencyProjectUser(t)
//
//	txManager := postgres.NewTxManager(testPool)
//	repo := postgres.NewRepository(txManager)
//
//	cfg := &config.Config{}
//	cfg.Business.FixationDuration = fixationDuration
//	cfg.Business.HashSecret = hashSecret
//
//	svc := usecase.NewService(cfg, zap.NewNop(), realClock{}, repo, txManager)
//
//	req := &usecase.FixationRequest{
//		AgencyID:  agencyID,
//		FixFor:    userID,
//		FixBy:     userID,
//		ProjectID: projectID,
//		Phone:     "8(999)444-55-66",
//	}
//
//	if _, err := svc.NewFixation(context.Background(), req); err != nil {
//		t.Fatalf("первая фиксация: %v", err)
//	}
//
//	// Сдвигаем срок в прошлое НАПРЯМУЮ в базе, не запуская никакой воркер.
//	// Смысл: горячий путь обязан работать, даже если фоновый процесс лежит.
//	_, err := testPool.Exec(context.Background(),
//		`UPDATE integration.fixations SET expires_at = now() - interval '1 day'
//		 WHERE project_id = $1`, projectID)
//	if err != nil {
//		t.Fatalf("сдвиг срока: %v", err)
//	}
//
//	if _, err := svc.NewFixation(context.Background(), req); err != nil {
//		t.Fatalf("вторая фиксация после протухания: %v", err)
//	}
//}

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
	//var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 50; i++ {
		//wg.Add(1)
		go func() {
			//defer wg.Done()
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
	testPool = poolWithMaxConns(t, goroutines, dbtest.MakeDSN("integration"))
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
