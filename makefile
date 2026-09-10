# Makefile — каталог операций проекта.
#
# Зачем: команды перестают жить в голове и в истории терминала.
# Новый человек в команде читает Makefile и видит всё, что можно делать.
# CI вызывает те же цели — одна точка правды.
#
# ВАЖНО: отступы — ТАБУЛЯЦИЯ, не пробелы. Иначе "missing separator".

# ── Версии инструментов ────────────────────────────────────────────────
# Пиннинг намеренный: с @latest у тебя, у коллеги и в CI получится
# разный сгенерированный код, и в git полетят диффы, которых никто не писал.
#
# PROTOC_GEN_GO_VERSION должна совпадать с версией google.golang.org/protobuf
# в go.mod — сгенерированный .pb.go содержит проверку совместимости версий.
# Посмотреть текущую:  go list -m google.golang.org/protobuf

PROTOC_GEN_GO_VERSION      := v1.36.11
PROTOC_GEN_GO_GRPC_VERSION := v1.5.1
MOCKGEN_VERSION            := v0.6.0

# ── Пути ───────────────────────────────────────────────────────────────
AUTH_SERVICE      := ./services/app/authservice/cmd/authservice
AUTH_MIGRATOR     := ./services/app/authservice/cmd/migrator
FIXATION_SERVICE  := ./services/integration/fixationservice/cmd/fixationservice
FIXATION_MIGRATOR := ./services/integration/fixationservice/cmd/migrator
PARTNER_API       := ./services/integration/partnerapi/cmd/partnerapi
DB_BOOTSTRAP      := ./tools/dbbootstrap

# ── Адреса ─────────────────────────────────────────────────────────────
# Имя базы одно во всех местах: compose, configs/*.yaml, здесь, в dbtest.
DB_NAME    := broker
DB_USER    := postgres
GRPC_ADDR  := localhost:50052
HTTP_ADDR  := http://localhost:8080

BOOTSTRAP_DSN := postgres://postgres:postgres@localhost:55432/$(DB_NAME)?sslmode=disable

# Соль телефона. Значение то же, что в configs/local.yaml и в compose:
# сидер и сервис обязаны считать один и тот же хэш, иначе частичный
# уникальный индекс на стенде не срабатывает и стенд врёт.
HASH_SECRET := local-phone-hash-secret-change-me

# .PHONY говорит make: это команды, а не имена файлов.
# Без него make увидит файл с таким именем и решит, что делать нечего.
.PHONY: help tools generate mocks lint fmt vet test test-integration race-fixation cover \
        build run run-auth run-fixation run-partnerapi \
        migrate migrate-bootstrap migrate-app migrate-integration seed seed-check seed-bulk seed-bulk-check show-data db-check postman-env clean-stray-db \
        up up-full down down-v logs psql redis-cli rabbit-ui minio-ui \
        grpc-list grpc-fix check-db reconcile token vuln breaking

# ── Справка ────────────────────────────────────────────────────────────
help:
	@echo "Инфраструктура:"
	@echo "  make up                поднять базу, redis, rabbit, minio, jaeger, prometheus, grafana, моки"
	@echo "  make up-full           то же + монолит + все три Go-сервиса + миграции"
	@echo "  make down              остановить"
	@echo "  make down-v            остановить и стереть данные"
	@echo "  make logs              смотреть логи"
	@echo "  make psql              консоль postgres"
	@echo "  make psql-a / psql-b   два именованных сеанса psql для ручного воспроизведения"
	@echo "  make locks             кто кого ждёт: pg_stat_activity + pg_blocking_pids"
	@echo "  make redis-cli         консоль redis"
	@echo "  make rabbit-ui         открыть RabbitMQ management"
	@echo "  make minio-ui          открыть консоль MinIO"
	@echo ""
	@echo "База:"
	@echo "  make migrate           все три каталога миграций по порядку"
	@echo "  make migrate-bootstrap схемы, роли, гранты (суперюзером)"
	@echo "  make migrate-app       migrations/app под app_user"
	@echo "  make migrate-integration  migrations/integration под go_user"
	@echo "  make seed              демо-данные: агентства, лоты, фиксации"
	@echo "  make seed-check        проверить состав сценарных данных без вывода количества"
	@echo "  make seed-bulk         объём поверх демо-данных (ROWS=3000000 по умолчанию)"
	@echo "  make show-data         кто и что лежит в базе: агентства, сотрудники, проекты"
	@echo "  make db-check          на какой сервер смотрят хост и compose, что там есть"
	@echo "  make clean-stray-db DSN=..  убрать схемы стенда из чужого постгреса"
	@echo "  make postman-env       собрать окружение Postman из текущих данных базы"
	@echo ""
	@echo "Разработка:"
	@echo "  make run-partnerapi    запустить partnerapi"
	@echo "  make run-auth          запустить authservice"
	@echo "  make run-fixation      запустить fixationservice"
	@echo "  make generate          proto -> Go"
	@echo "  make mocks             gomock по интерфейсам юзкейсов"
	@echo ""
	@echo "Проверки:"
	@echo "  make lint              golangci-lint + buf lint"
	@echo "  make test              быстрые тесты (-short)"
	@echo "  make test-integration  все тесты, поднимает Postgres в docker"
	@echo "  make race-fixation     50 горутин на один телефон, распределение кодов"
	@echo "  make vuln              скан уязвимостей"
	@echo ""
	@echo "Проверка руками:"
	@echo "  make grpc-list         какие методы есть на fixationservice"
	@echo "  make token EMAIL=..    получить access-токен сотрудника"
	@echo "  make grpc-fix          дёрнуть NewFixation напрямую по gRPC"
	@echo "  make check-db          что записалось в фиксации"
	@echo "  make reconcile         сверить наши фиксации с моком amoCRM"

# ── Инструменты ────────────────────────────────────────────────────────
tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	go install go.uber.org/mock/mockgen@$(MOCKGEN_VERSION)
	go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

# "generate: tools" = перед generate выполни tools.
# Не надо помнить порядок — make сам разрулит.
generate: tools
	buf generate

# ── Моки ───────────────────────────────────────────────────────────────
# Генерим по интерфейсам-портам, а не по конкретным типам: порт объявлен
# на стороне юзкейса, и мок должен следовать за ним. Руками написанный мок
# молча расходится с интерфейсом ровно в тот момент, когда в интерфейс
# добавили метод — и тест продолжает зеленеть, проверяя вчерашний код.
#
# Сгенерированное лежит в отдельных пакетах *mocks: так оно не мешает
# ручным заглушкам в тестах и не попадает в прод-сборку.
mocks:
	go install go.uber.org/mock/mockgen@$(MOCKGEN_VERSION)
	mockgen -source=services/integration/fixationservice/internal/usecase/deps.go \
	        -destination=services/integration/fixationservice/internal/usecase/mocks/usecase_mocks.go \
	        -package=usecasemocks
	mockgen -source=services/app/authservice/internal/domain/interfaces.go \
	        -destination=services/app/authservice/internal/domain/mocks/domain_mocks.go \
	        -package=domainmocks
	mockgen -source=services/integration/partnerapi/internal/transport/handlers/fixationhandlers/handler.go \
	        -destination=services/integration/partnerapi/internal/transport/handlers/fixationhandlers/mocks/fixation_client_mock.go \
	        -package=fixationhandlersmocks

# ── Инфраструктура ─────────────────────────────────────────────────────
up:
	docker compose up -d
	@echo ""
	@echo "Jaeger      http://localhost:16686"
	@echo "Prometheus  http://localhost:9090"
	@echo "Grafana     http://localhost:3000   admin/admin"
	@echo "RabbitMQ    http://localhost:15672  broker/broker17"
	@echo "MinIO       http://localhost:9001   minioadmin/minioadmin"
	@echo "mock-amocrm      http://localhost:9101/_control/state"
	@echo "mock-profitbase  http://localhost:9102/_control/state"

# Профиль full поднимает ещё монолит и три Go-сервиса.
# Миграции внутри профиля — отдельные one-shot контейнеры, и сервисы
# ждут их через service_completed_successfully.
up-full:
	docker compose --profile full up -d --build
	@echo ""
	@echo "partnerapi  $(HTTP_ADDR)/healthz"
	@echo "монолит     http://localhost:8000/internal/v1/health"
	@echo "Дальше:     make seed"

down:
	docker compose --profile full down

# Отдельная цель, а не флаг: -v стирает тома, и такое не должно
# случаться по опечатке в make down.
down-v:
	docker compose --profile full down -v

logs:
	docker compose --profile full logs -f

psql:
	docker compose exec postgres psql -U $(DB_USER) -d $(DB_NAME)

# Два именованных сеанса для ручного воспроизведения гонок.
#
#   терминал 1:  make psql-a
#   терминал 2:  make psql-b
#
# Отличаются только приглашением. В двух одинаковых окнах команды путаются
# местами, и потом непонятно, какая транзакция что видела; с «A>» и «B>»
# лог сеанса можно приложить к отчёту как есть.
#
# Транзакцией psql управляешь сам: begin/commit/rollback руками, иначе
# каждая команда уедет в свою автокоммитную транзакцию и наблюдать будет
# нечего.
psql-a:
	docker compose exec postgres psql -U $(DB_USER) -d $(DB_NAME) -v PROMPT1='A> ' -v PROMPT2='A- '

psql-b:
	docker compose exec postgres psql -U $(DB_USER) -d $(DB_NAME) -v PROMPT1='B> ' -v PROMPT2='B- '

# Кто кого ждёт прямо сейчас. Третий терминал к паре выше: пока A и B
# сидят в открытых транзакциях, здесь видно, висит ли кто-то на блокировке
# и на ком именно.
locks:
	@docker compose exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -X -q -c "\
	  select a.pid, \
	         a.state, \
	         a.wait_event_type || ':' || coalesce(a.wait_event, '-') as waiting, \
	         pg_blocking_pids(a.pid)                                 as blocked_by, \
	         now() - a.xact_start                                    as xact_age, \
	         left(regexp_replace(a.query, '\\s+', ' ', 'g'), 60)      as query \
	    from pg_stat_activity a \
	   where a.datname = current_database() \
	     and a.pid <> pg_backend_pid() \
	     and a.backend_type = 'client backend' \
	   order by a.pid"

redis-cli:
	docker compose exec redis redis-cli

rabbit-ui:
	@echo "RabbitMQ management: http://localhost:15672  (broker/broker17)"
	@echo "Обменники: admin.events (монолит -> Go), fixation.events (Go -> монолит)"
	@python -m webbrowser http://localhost:15672 2>/dev/null || \
	 start http://localhost:15672 2>/dev/null || \
	 xdg-open http://localhost:15672 2>/dev/null || true

minio-ui:
	@echo "MinIO console: http://localhost:9001  (minioadmin/minioadmin)"
	@python -m webbrowser http://localhost:9001 2>/dev/null || \
	 start http://localhost:9001 2>/dev/null || \
	 xdg-open http://localhost:9001 2>/dev/null || true

# ── Миграции ───────────────────────────────────────────────────────────
# Три шага, три владельца, три роли. Порядок обязателен: bootstrap создаёт
# схемы и роли, под которыми работают остальные два.
#
# После миграций — постусловие. Запись в таблице версий goose означает
# «мигратор считает, что накатил», а не «объекты есть на том сервере,
# куда пойдут сервисы». Разница вылезает ровно один раз и стоит часа.
migrate: migrate-bootstrap migrate-app migrate-integration
	@go run ./tools/dbcheck -dsn "$(BOOTSTRAP_DSN)" \
	  -require app.users,app.agencies,app.projects,app.lots,integration.fixations,integration.outbox

migrate-bootstrap:
	BOOTSTRAP_DSN="$(BOOTSTRAP_DSN)" go run $(DB_BOOTSTRAP)

migrate-app:
	go run $(AUTH_MIGRATOR)

migrate-integration:
	go run $(FIXATION_MIGRATOR)

# ── Демо-данные ────────────────────────────────────────────────────────
# 3 агентства, 10 сотрудников, 2 проекта, 200 лотов, 300 фиксаций.
# Печатает идентификаторы, которые дальше нужны для make race-fixation.
#
# Сидер ходит суперпользователем: фиксации лежат в схеме integration,
# и у app_user там только select. Это осознанное нарушение границы
# контуров ради стенда.
#
# --no-deps: поднимаем ТОЛЬКО контейнер монолита. Без него docker compose
# потянет за собой всю цепочку depends_on — rabbitmq и три контейнера
# с миграциями. Миграции к этому моменту уже накачены (`make migrate`),
# а шина сидеру не нужна вообще.
# --build: seed.php уезжает в образ на этапе сборки (COPY monolith/bin).
# Без пересборки правка сидера не доезжает до контейнера, сидер отчитывается
# об успехе, и полчаса уходит на вопрос «почему данных нет». Сборка кэшируется
# и стоит секунды.
seed:
	docker compose --profile full run --rm --no-deps --build \
	  -e SEED_DB_DSN="pgsql:host=postgres;port=5432;dbname=$(DB_NAME)" \
	  -e SEED_DB_USER=postgres \
	  -e SEED_DB_PASSWORD=postgres \
	  -e SEED_PHONE_HASH_SECRET="$(HASH_SECRET)" \
	  --entrypoint php monolith /app/bin/seed.php
	@$(MAKE) --no-print-directory seed-check
	@$(MAKE) --no-print-directory postman-env

# Сидер обязан воспроизводить граничные случаи и считать phone_hash тем же
# способом, что fixationservice. Проверка не печатает объёмы специальных
# наборов: их разработчик устанавливает запросом во время расследования.
seed-check:
	@docker compose exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -X -q \
	  -v key="$(HASH_SECRET)" -f - < deploy/seed/verify_fixations.sql

# Объём поверх демо-данных. Отдельной целью, а не частью `make seed`:
# три миллиона строк нужны, когда меряешь план запроса, и мешают, когда
# просто смотришь список глазами.
#
#   make seed-bulk                # 3 000 000 строк
#   make seed-bulk ROWS=500000
#
# Требование «уложиться в 50 мс на 3 млн строк» на трёхстах строках не
# проверяется никак: планировщик берёт seq scan, оказывается прав, и
# отсутствие индекса выглядит как быстрый запрос.
#
# Убрать объём: `make seed` — он чистит фиксации целиком.
seed-bulk:
	@docker compose exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -X \
	  -v rows=$(or $(ROWS),3000000) -v key="$(HASH_SECRET)" \
	  -f - < deploy/seed/bulk_fixations.sql
	@$(MAKE) --no-print-directory seed-bulk-check ROWS=$(or $(ROWS),3000000)

seed-bulk-check:
	@docker compose exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -X -q \
	  -v rows=$(or $(ROWS),3000000) -v key="$(HASH_SECRET)" \
	  -f - < deploy/seed/verify_bulk_fixations.sql

# Сравнение двух точек зрения на базу: с хоста (так ходят мигратор и
# сервисы, запущенные через make run-*) и из сети compose (так ходят
# монолит и сидер).
#
# Если system_identifier в двух блоках разный — это ДВА разных сервера,
# и все странности вида «миграции накачены, а таблицы нет» объясняются им.
db-check:
	@go run ./tools/dbcheck -label "с хоста: $(BOOTSTRAP_DSN)" -dsn "$(BOOTSTRAP_DSN)"
	@echo ""
	@echo "── из сети compose: postgres:5432/$(DB_NAME) ──"
	@docker compose run --rm --no-deps -e PGPASSWORD=postgres --entrypoint psql postgres \
	  -h postgres -U postgres -d $(DB_NAME) -X -q -c \
	  "select current_database() as db, inet_server_addr()::text as server, \
	          (select system_identifier::text from pg_control_system()) as system_identifier;" \
	  -c "select schemaname || '.' || tablename as tables from pg_tables \
	       where schemaname in ('app','integration') order by 1;"

# Убрать схемы стенда из ЧУЖОГО постгреса.
#
# Пока порт стенда был 5432, миграции уезжали в локальный PostgreSQL
# разработчика и создавали там app/integration и роли app_user/go_user.
# Стенду они больше не нужны, а путаться будут.
#
# DSN обязателен и передаётся руками: цель дропает схемы каскадом,
# и подставлять его по умолчанию нельзя. Сначала показывает, что удалит,
# и просит подтвердить.
#
#   make clean-stray-db DSN="postgres://postgres:ПАРОЛЬ@localhost:5432/broker?sslmode=disable"
#   make clean-stray-db DSN="..." CONFIRM=yes
clean-stray-db:
	@test -n "$(DSN)" || (echo "нужен DSN=<строка подключения к чужой базе>"; exit 1)
	@go run ./tools/dbcheck -label "что сейчас в этой базе" -dsn "$(DSN)"
	@test "$(CONFIRM)" = "yes" || (echo ""; echo "Схемы app и integration будут удалены КАСКАДОМ."; echo "Если это та база — повтори команду с CONFIRM=yes"; exit 1)
	@docker compose run --rm --no-deps --entrypoint psql postgres "$(DSN)" -X -q -c 	  "drop schema if exists integration cascade; 	   drop schema if exists app cascade; 	   drop role if exists go_user; 	   drop role if exists app_user;"
	@echo "схемы и роли стенда убраны"

# Окружение Postman из того, что реально лежит в базе.
# Идентификаторы сидер генерирует заново на каждом прогоне, переносить их
# руками — шесть copy-paste и один шанс из шести ошибиться.
#
# В Postman: Import -> deploy/postman/broker.postman_environment.json,
# дальше выбрать окружение в списке справа сверху.
postman-env:
	@go run ./tools/standenv -dsn "$(BOOTSTRAP_DSN)"

# Что реально лежит в базе: агентства с их сотрудниками и проекты.
# Первое, что смотрят, когда «логин не проходит» — есть ли вообще
# пользователь с такой почтой. Вывод `make seed` теряется при закрытии
# терминала, а эта цель повторяется сколько угодно.
show-data:
	@docker compose exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -X -q -c \
	  "select a.status as agency, a.name, u.user_role, u.email, u.id \
	     from app.agencies a join app.users u on u.agency_id = a.id \
	    order by a.name, u.email;"
	@docker compose exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -X -q -c \
	  "select 'no agency' as agency, '' as name, user_role, email, id \
	     from app.users where agency_id is null order by email;"
	@docker compose exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -X -q -c \
	  "select status, name, id from app.projects order by status, name;"

# ── Разработка ─────────────────────────────────────────────────────────
build:
	go build ./...

run: run-partnerapi

run-auth:
	go run $(AUTH_SERVICE)

run-fixation:
	go run $(FIXATION_SERVICE)

run-partnerapi:
	go run $(PARTNER_API)

# ── Проверки ───────────────────────────────────────────────────────────
fmt:
	gofmt -w .

vet:
	go vet ./...

lint:
	golangci-lint run ./...
	buf lint

# -short пропускает интеграционные: в dbtest.Start стоит t.Skip
test:
	go test -race -short ./...

test-integration:
	go test -race ./...

# Гонка на горячем пути: 50 горутин одновременно фиксируют один телефон
# в один проект. Нужен запущенный partnerapi и токен брокера.
#
#   make race-fixation TOKEN=... PROJECT=... FIX_FOR=...
#
# Идентификаторы печатает `make seed`, токен даёт POST /api/v1/auth/login.
race-fixation:
	@test -n "$(TOKEN)"   || (echo "нужен TOKEN=<access token>"; exit 1)
	@test -n "$(PROJECT)" || (echo "нужен PROJECT=<uuid проекта>"; exit 1)
	@test -n "$(FIX_FOR)" || (echo "нужен FIX_FOR=<uuid брокера>"; exit 1)
	go run ./tools/racefix \
	  -url $(HTTP_ADDR) \
	  -token "$(TOKEN)" \
	  -project "$(PROJECT)" \
	  -fix-for "$(FIX_FOR)" \
	  -n $(or $(N),50)

cover:
	go test -short -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

vuln:
	govulncheck ./...

breaking:
	buf breaking --against '.git#branch=main'

# ── Проверка ручки руками ──────────────────────────────────────────────
# Работает благодаря reflection, включённой в app.go для локалки.
grpc-list:
	grpcurl -plaintext $(GRPC_ADDR) list
	grpcurl -plaintext $(GRPC_ADDR) list fixation.v1.FixationService

# Прямой вызов fixationservice, минуя partnerapi. Полезно, чтобы понять,
# на чьей стороне проблема: если здесь работает, а через HTTP нет —
# дело в partnerapi.
#
#   make grpc-fix PROJECT=... FIX_FOR=...
grpc-fix:
	grpcurl -plaintext  \
	-h "agency_id: $(or $(AGENCY_ID),11111111-1111-1111-1111-111111111111),  " \
	-h "user_id: $(or $(USER_ID),11111111-1111-1111-1111-111111111111),  " \
	-h "role: $(or $(ROLE),admin),  " \
	-d '{ \
	  "fix_for":    "$(or $(FIX_FOR),22222222-2222-2222-2222-222222222222)", \
	  "phone":      "$(or $(PHONE),+7 (999) 111-22-33)", \
	  "project_id": "$(or $(PROJECT),33333333-3333-3333-3333-333333333333)" \
	}' $(GRPC_ADDR) fixation.v1.FixationService/NewFixation

# Проверить, что записалось: времена не перепутаны, статус активный,
# срок больше момента фиксации.
check-db:
	docker compose exec postgres psql -U $(DB_USER) -d $(DB_NAME) -c \
	  "select id, status, fixed_at, expires_at, expires_at > fixed_at as period_ok \
	     from integration.fixations order by fixed_at desc limit 5;"

# Access-токен сотрудника. Печатает ТОЛЬКО токен — чтобы подставлялось
# в переменную:
#
#   TOKEN=$$(make -s token EMAIL=broker1@perviy-metr.test)
#
# Список сотрудников и их агентств печатает `make seed`.
token:
	@test -n "$(EMAIL)" || (echo "нужен EMAIL=<почта сотрудника>, список печатает make seed"; exit 1)
	@go run ./tools/token -url $(HTTP_ADDR) -email "$(EMAIL)" -password "$(or $(PASSWORD),password)"

# Сверка с моком amoCRM: что не доехало, что лишнее, где разошлись статусы.
reconcile:
	go run ./tools/reconcile -amocrm http://localhost:9101
