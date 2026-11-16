# ДЗ 4 — рефакторинг сервиса фильмов

## Структура

- `internal/domain` — модели и ошибки.
- `internal/usecase` — бизнес-логика и интерфейсы.
- `internal/adapter/stand` — клиент фильм-стенда.
- `internal/handler` — HTTP и gRPC.
- `cmd/server` — конфигурация, сборка зависимостей и запуск.

## Запуск

Из корня репозитория:

```sh
go run ./hw3/cmd/server
```

| Переменная | По умолчанию |
| --- | --- |
| `MOVIE_BASE_URL` | `https://homeworksite.site` |
| `HTTP_ADDR` | `:8080` |
| `GRPC_ADDR` | `:9090` |

HTTP: `/load-movies?from=1&to=3`, `/last-movie`.
Swagger: `/swagger/index.html`.
gRPC: `LoadMovies`, `GetLastMovie` — схема в `api/movies.proto`.

## Проверка

```sh
go test -race ./...
go vet ./...
```
