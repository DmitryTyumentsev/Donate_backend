// Package dbtest поднимает настоящий PostgreSQL в docker, катит на него
// те же миграции, что в проде, и отдаёт пул.
//
// Зачем настоящая база, а не мок репозитория. Всё, ради чего этот проект
// вообще существует, живёт в базе: частичный уникальный индекс, уровни
// изоляции, поведение при конкурентной вставке. Мок репозитория
// подтверждает только то, что код вызвал метод, который сам же и описал.
//
// Использование:
//
//	func TestMain(m *testing.M) { ... }   // см. пример в integrationtest
//	pool := dbtest.Start(t)
//
// Контейнер живёт на весь пакет тестов, а не на каждый тест: старт
// PostgreSQL занимает секунды, и поднимать его на каждый Test — способ
// сделать прогон невыносимо долгим и получить «да ну эти интеграционные».
package dbtest

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // драйвер "pgx" для database/sql, нужен goose
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/pressly/goose/v3"
)

const (
	// Версия та же, что в docker-compose.yml. Тест на 16-й версии против
	// прода на 13-й — способ не заметить регрессию и найти несуществующую.
	postgresImage = "postgres"
	postgresTag   = "13-alpine"

	// Имя базы одно во всём проекте.
	databaseName = "broker"

	// Страховка: если процесс тестов убьют, контейнер умрёт сам.
	containerTTLSeconds = 300

	startupTimeout = 60 * time.Second
)

// Instance — поднятая база. Close() обязателен, иначе контейнер останется
// висеть до срабатывания TTL.
type Instance struct {
	Pool *pgxpool.Pool

	// DSN отдаёт строку подключения с нужным search_path. Нужен тестам,
	// которые хотят своё соединение под postgres.
	DSN func(searchPath string) string

	// DSNForRole нужен проверкам реальных грантов приложения.
	DSNForRole func(username, password, searchPath string) string

	purge func()
}

func (i *Instance) Close() {
	if i == nil {
		return
	}
	if i.Pool != nil {
		i.Pool.Close()
	}
	if i.purge != nil {
		i.purge()
	}
}

// Start поднимает контейнер, катит миграции и возвращает готовый пул.
//
// Возвращаемый Pool работает под postgres: тесту часто нужно готовить данные
// в обеих схемах. Сами миграции при этом катятся реальными владельцами,
// а DSNForRole позволяет отдельно проверить гранты приложения.
func Start(tb testing.TB) *Instance {
	tb.Helper()

	if testing.Short() {
		tb.Skip("integration test: пропускаем при -short")
	}

	dockerPool, err := dockertest.NewPool("")
	if err != nil {
		tb.Fatalf("connect to docker: %v", err)
	}

	if err := dockerPool.Client.Ping(); err != nil {
		tb.Fatalf("docker is not running: %v", err)
	}

	// AutoRemove — чтобы не копить мёртвые контейнеры, если тест упадёт
	// до Purge.
	resource, err := dockerPool.RunWithOptions(&dockertest.RunOptions{
		Repository: postgresImage,
		Tag:        postgresTag,
		Env: []string{
			"POSTGRES_USER=postgres",
			"POSTGRES_PASSWORD=postgres",
			"POSTGRES_DB=" + databaseName,
			"listen_addresses='*'",
		},
	}, func(hc *docker.HostConfig) {
		hc.AutoRemove = true
		hc.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		tb.Fatalf("start postgres container: %v", err)
	}

	purge := func() { _ = dockerPool.Purge(resource) }
	_ = resource.Expire(containerTTLSeconds)

	hostPort := resource.GetPort("5432/tcp")
	dsnForRole := func(username, password, searchPath string) string {
		return fmt.Sprintf(
			"postgres://%s:%s@localhost:%s/%s?sslmode=disable&search_path=%s",
			username, password, hostPort, databaseName, searchPath,
		)
	}
	dsn := func(searchPath string) string {
		return dsnForRole("postgres", "postgres", searchPath)
	}

	// Контейнер поднялся, но база внутри стартует не мгновенно.
	// Retry долбится с backoff, пока не ответит.
	dockerPool.MaxWait = startupTimeout
	if err := dockerPool.Retry(func() error {
		db, err := sql.Open("pgx", dsn("public"))
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()

		// PingContext с потолком: без него одна зависшая попытка съедает
		// весь MaxWait, и вместо «база не поднялась за минуту» тест висит.
		pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		return db.PingContext(pingCtx)
	}); err != nil {
		purge()
		tb.Fatalf("wait for postgres: %v", err)
	}

	if err := applyMigrations(dsnForRole); err != nil {
		purge()
		tb.Fatalf("apply migrations: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), dsn("integration,app,public"))
	if err != nil {
		purge()
		tb.Fatalf("create pool: %v", err)
	}

	return &Instance{Pool: pool, DSN: dsn, DSNForRole: dsnForRole, purge: purge}
}

// applyMigrations катит те же три каталога, что и прод, в том же порядке.
//
// Важно: у каждого каталога СВОЙ search_path и СВОЯ таблица версий.
// Первое — потому что часть объектов создаётся без префикса схемы.
// Второе — потому что с общей таблицей goose решит, что версия 00001
// уже накачена, когда дойдёт до второго каталога, и молча пропустит его.
func applyMigrations(dsn func(username, password, searchPath string) string) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	root, err := repoRoot()
	if err != nil {
		return err
	}

	steps := []struct {
		dir        string
		searchPath string
		table      string
		username   string
		password   string
	}{
		{"bootstrap", "public", "public.goose_bootstrap_version", "postgres", "postgres"},
		{"app", "app", "app.goose_db_version", "app_user", "app_user"},
		{"integration", "integration", "integration.goose_db_version", "go_user", "go_user"},
	}

	for _, step := range steps {
		db, err := sql.Open("pgx", dsn(step.username, step.password, step.searchPath))
		if err != nil {
			return fmt.Errorf("open db for %s: %w", step.dir, err)
		}

		goose.SetTableName(step.table)
		err = goose.Up(db, filepath.Join(root, "migrations", step.dir))
		_ = db.Close()

		if err != nil {
			return fmt.Errorf("goose up %s: %w", step.dir, err)
		}
	}

	return nil
}

// repoRoot идёт вверх от текущего каталога, пока не найдёт go.mod.
// Так путь к миграциям не зависит от того, из какого пакета запущен тест.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}

		dir = parent
	}
}
