# ДЗ 5 — PostgreSQL

При запуске сервис применяет миграции и скачивает недостающие фильмы со стенда.
HTTP и gRPC читают данные из БД.

## Запуск

Из корня репозитория:

```sh
docker compose -f hw5/compose.yaml up -d --wait
export DATABASE_URL='postgres://movies:movies@localhost:5432/movies?sslmode=disable'
go run ./hw5/cmd/server
```

| Переменная | По умолчанию |
| --- | --- |
| `DATABASE_URL` | обязательна |
| `MOVIE_BASE_URL` | `https://homeworksite.site` |
| `HTTP_ADDR` | `:8080` |
| `GRPC_ADDR` | `:9090` |

HTTP: `/load-movies?from=1&to=3`, `/last-movie`.
Swagger: `/swagger/index.html`.
gRPC: `LoadMovies`, `GetLastMovie` — схема в `api/movies.proto`.

## Структура

- `internal/domain` — модели и ошибки.
- `internal/usecase` — чтение фильмов, синхронизация, интерфейсы.
- `internal/adapter` — PostgreSQL, SQL-миграции и клиент стенда.
- `internal/handler` — HTTP и gRPC.
- `cmd/server` — конфигурация и запуск.

## Проверка

```sh
go test -race ./...
go vet ./...
```

Интеграционные тесты создают отдельные схемы в тестовой БД:

```sh
TEST_DATABASE_URL="$DATABASE_URL" go test -race ./...
```
