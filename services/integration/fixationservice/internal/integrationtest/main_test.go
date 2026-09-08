// Интеграционные тесты фиксации: настоящий PostgreSQL, настоящие миграции,
// настоящий репозиторий. Ни одного мока.
//
// Запуск:  make test-integration
// Пропуск: go test -short ./...   (нужен docker, поэтому в -short их нет)
//
// Подъём контейнера вынесен в shared/pkg/dbtest — он один на все сервисы.
// Здесь остаётся только то, что специфично для фиксаций: подготовка
// данных и контрольные проверки прямо в базе.

package integrationtest

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"

	"Broker_backend/shared/pkg/dbtest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool — живой пул к контейнерной базе. Заполняется в TestMain,
// используется всеми тестами пакета.
var testPool *pgxpool.Pool

// TestMain — точка входа пакета. Go вызывает её вместо того, чтобы сразу
// запускать тесты. Всё до m.Run() — подготовка, всё после — уборка.
func TestMain(m *testing.M) {
	os.Exit(run(m))
}

// run вынесен отдельно, чтобы работали defer: в TestMain их нельзя
// использовать из-за os.Exit, который завершает процесс мгновенно.
func run(m *testing.M) int {
	// В обычном прогоне флаги разбирает m.Run. Здесь значение -short нужно
	// раньше, чтобы вообще не поднимать контейнер, поэтому разбираем их сами.
	if !flag.Parsed() {
		flag.Parse()
	}

	// При -short выходим ДО m.Run(): без контейнера testPool остался бы nil,
	// и тесты упали бы не «пропущено», а паникой на nil-пуле.
	if testing.Short() {
		fmt.Fprintln(os.Stderr, "integrationtest: пропускаем при -short, нужен docker")

		return 0
	}

	instance := dbtest.Start(&mainT{})
	defer instance.Close()

	testPool = instance.Pool

	return m.Run()
}

// mainT — минимальная заглушка testing.TB для TestMain, где настоящего
// *testing.T ещё нет. Fatalf обязан завершить процесс: без базы
// продолжать нечего.
type mainT struct {
	testing.TB
}

func poolWithMaxConns(t *testing.T, n int32, dsn string) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	cfg.MaxConns = n
	p, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(p.Close)
	return p
}

func (t *mainT) Helper() {}

func (t *mainT) Skip(args ...any) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(0)
}

func (t *mainT) Fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "integration setup: "+format+"\n", args...)
	os.Exit(1)
}

// ─────────────────────────────────────────────────────────────────────
// Хелперы подготовки данных
// ─────────────────────────────────────────────────────────────────────

// seedAgencyProjectUser создаёт агентство, проект и сотрудника в нём.
// Без них NewFixation отвалится на проверках StatusByProjectID
// и IsUserIDInAgencyID.
//
// Каждый тест зовёт это сам и получает СВОИ идентификаторы — тогда тесты
// не мешают друг другу и таблицу между ними чистить не надо.
func seedAgencyProjectUser(t *testing.T) (agencyID, projectID, userID uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	agencyID = uuid.New()
	projectID = uuid.New()
	userID = uuid.New()

	_, err := testPool.Exec(ctx,
		`INSERT INTO app.agencies (id, name, status) VALUES ($1, $2, 'active')`,
		agencyID, "test agency "+agencyID.String()[:8])
	if err != nil {
		t.Fatalf("seed agency: %v", err)
	}

	_, err = testPool.Exec(ctx,
		`INSERT INTO app.projects (id, name, status) VALUES ($1, $2, 'active')`,
		projectID, "test project "+projectID.String()[:8])
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}

	_, err = testPool.Exec(ctx,
		`INSERT INTO app.users (id, email, user_role, password_hash, last_name, first_name, agency_id)
		 VALUES ($1, $2, 'broker_team_member', 'x', 'Тестов', 'Тест', $3)`,
		userID, userID.String()[:8]+"@test.local", agencyID)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	return agencyID, projectID, userID
}

// countActiveFixations — контрольная проверка ПРЯМО В БАЗЕ.
// Без неё тест доказывает лишь то, что код вернул одну ошибку,
// а не то, что база оказалась в одном состоянии.
func countActiveFixations(t *testing.T, phoneHash string, projectID uuid.UUID) int {
	t.Helper()

	var n int
	err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM integration.fixations
		 WHERE phone_hash = $1 AND project_id = $2 AND status = 'active'`,
		phoneHash, projectID).Scan(&n)
	if err != nil {
		t.Fatalf("count fixations: %v", err)
	}

	return n
}
