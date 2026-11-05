# Разработка

Проект напрямую использует библиотеку `github.com/things-go/go-socks5`. Команда UDP ASSOCIATE поддерживается библиотекой, но отключена по умолчанию из соображений безопасности и включается опционально через `ENABLE_UDP_ASSOCIATE`. Команда BIND не поддерживается. Фильтрация `AllowedIPs` выполняется на уровне listener.

Структура проекта:

```text
cmd/socks5-proxy-server/    entrypoint приложения
internal/proxy/             config, auth, access policy, listeners, TLS, metrics, server orchestration
.github/workflows/          CI и публикация Docker image
```

## Локальные проверки

```bash
go test ./...
go test -race ./...
go vet ./...
golangci-lint run ./...
```

Те же проверки доступны через Makefile: `make test`, `make test-race`, `make vet`, `make lint`.

## Проверки безопасности

Полный security-контур (govulncheck, hadolint, actionlint, zizmor, gitleaks по истории и рабочему дереву, trivy, semgrep):

```bash
make security        # полный вариант (semgrep --config=auto)
make security-fast   # быстрый вариант, используется в CI
```

Требуются локально установленные `hadolint`, `gitleaks`, `zizmor`, `trivy`, `semgrep`; `govulncheck` и `actionlint` запускаются через `go run` по закреплённым версиям. Диапазон коммитов gitleaks задаётся переменной `GITLEAKS_LOG_OPTS` (по умолчанию `HEAD`). Для `make security` (semgrep `--config=auto`) дополнительно нужен `semgrep login` или `SEMGREP_APP_TOKEN`.

В CI эти проверки выполняет workflow `.github/workflows/security.yml` на push/PR в `master`: job падает при любых находках.

## Сборка бинарника

```bash
go build ./cmd/socks5-proxy-server
```

## Локальные метрики при разработке

```bash
METRICS_ADDR=127.0.0.1:9090 go run ./cmd/socks5-proxy-server
curl http://127.0.0.1:9090/debug/vars
```

## Сборка и запуск через Docker Compose

```bash
cp .env.example .env
docker compose -f docker-compose.build.yml up -d --build
```

[English README](../README.en.md) | [Русский README](../README.md)
