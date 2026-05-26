# SOCKS5 Proxy Server

![Build and Push Docker image](https://github.com/astronaut808/socks5-proxy-server/actions/workflows/docker.yml/badge.svg)

[English documentation](README.en.md)

Небольшой SOCKS5 proxy server на `go-socks5` и Go 1.26.3. Поддерживает аутентификацию по логину и паролю, несколько аккаунтов, allowlist по исходным IP, политики доступа по FQDN назначения, лимиты соединений, rate limiting, обычный SOCKS5, SOCKS5 over TLS и опциональный mutual TLS.

## Быстрый старт

Запуск с одним пользователем и паролем:

```bash
docker run -d --name socks5 \
  -p 1080:1080 \
  -e PROXY_USER=<user> \
  -e PROXY_PASSWORD=<password> \
  ghcr.io/astronaut808/socks5-proxy-server:latest
```

Запуск на другом порту внутри контейнера:

```bash
docker run -d --name socks5 \
  -p 1090:9090 \
  -e PROXY_USER=<user> \
  -e PROXY_PASSWORD=<password> \
  -e PROXY_PORT=9090 \
  ghcr.io/astronaut808/socks5-proxy-server:latest
```

Проверка через proxy:

```bash
curl --socks5 <proxy-host>:1080 -U <user>:<password> https://ipinfo.io
```

## Конфигурация

| Переменная | Тип | По умолчанию | Обязательность | Описание |
| --- | --- | --- | --- | --- |
| `REQUIRE_AUTH` | Boolean | `true` | нет | Требовать SOCKS5-аутентификацию по логину и паролю. Отключать не рекомендуется, если proxy не защищён другим способом. |
| `PROXY_USER` | String | пусто | условно | Логин для одного пользователя. Нужен вместе с `PROXY_PASSWORD`, когда `REQUIRE_AUTH=true` и `PROXY_ACCOUNTS` пустой. |
| `PROXY_PASSWORD` | String | пусто | условно | Пароль для одного пользователя. Нужен вместе с `PROXY_USER`, когда `REQUIRE_AUTH=true` и `PROXY_ACCOUNTS` пустой. Plaintext-пароль хэшируется в памяти при запуске. |
| `PROXY_ACCOUNTS` | String | пусто | условно | Список аккаунтов через запятую в формате `user:password` или `user:bcrypt-hash`. Нужен, когда `REQUIRE_AUTH=true` и одиночные credentials не заданы. |
| `ENABLE_PLAIN` | Boolean | `false` | нет | Оставить обычный SOCKS5 listener включённым, когда включён TLS. |
| `PROXY_PORT` | String | `1080` | нет | Порт обычного SOCKS5 listener. Значение должно быть от 1 до 65535. |
| `PROXY_LISTEN_IP` | String | `0.0.0.0` | нет | Адрес прослушивания. Для доступа только с локальной машины используйте `127.0.0.1`. |
| `ALLOWED_DEST_FQDN` | String | пусто | нет | Регулярное выражение для FQDN назначения. Пустое значение разрешает все направления. Лучше использовать anchored regex, например `^api\.example\.com$`. |
| `ALLOWED_IPS` | String | пусто | нет | Allowlist исходных IP-адресов через запятую. |
| `MAX_CONNECTIONS` | Int | `100` | нет | Максимальное количество одновременных клиентских соединений. Должно быть больше нуля. |
| `TIMEOUT` | Int | `300` | нет | Read/write timeout в секундах. Должен быть больше нуля. |
| `MAX_AUTH_FAILURES` | Int | `5` | нет | Количество неудачных попыток аутентификации до временной блокировки. Должно быть больше нуля. |
| `AUTH_LOCKOUT` | Int | `300` | нет | Длительность блокировки в секундах. Должна быть больше нуля. |
| `RATE_LIMIT_PER_SEC` | Int | `10` | нет | Лимит новых соединений в секунду. Burst равен 2x от этого значения. Должен быть больше нуля. |
| `TLS_ENABLED` | Boolean | `false` | нет | Включить SOCKS5 over TLS. |
| `TLS_PORT` | String | `10443` | нет | Порт TLS listener. Значение должно быть от 1 до 65535. |
| `TLS_CERT_FILE` | String | пусто | условно | Путь к TLS-сертификату в PEM-формате. Нужен, когда `TLS_ENABLED=true`. |
| `TLS_KEY_FILE` | String | пусто | условно | Путь к приватному TLS-ключу в PEM-формате. Нужен, когда `TLS_ENABLED=true`. |
| `TLS_CLIENT_AUTH` | Boolean | `false` | нет | Требовать клиентский сертификат для TLS-подключений. |
| `TLS_CLIENT_CA_FILE` | String | пусто | условно | PEM-файл с CA для проверки клиентских сертификатов. Нужен, когда `TLS_CLIENT_AUTH=true`. |
| `TLS_MIN_VERSION` | String | `1.2` | нет | Минимальная версия TLS. Поддерживаются `1.2` и `1.3`. |
| `METRICS_ADDR` | String | пусто | нет | Опциональный адрес `host:port` для стандартного Go `expvar` endpoint на `/debug/vars`. Пустое значение отключает метрики. |

## Примеры

Ограничить направления конкретными доменами:

```bash
ALLOWED_DEST_FQDN='^.*\.(example\.com|internal\.local)$'
```

Разрешить доступ только с конкретных клиентских IP:

```bash
ALLOWED_IPS=192.168.1.10,10.0.0.5
```

Включить TLS:

```bash
TLS_ENABLED=true
TLS_PORT=10443
TLS_CERT_FILE=/etc/ssl/certs/proxy.crt
TLS_KEY_FILE=/etc/ssl/private/proxy.key
TLS_MIN_VERSION=1.3
```

Запуск с примонтированными TLS-сертификатами:

```bash
docker run -d --name socks5-tls \
  -p 10443:10443 \
  -e PROXY_USER=<user> \
  -e PROXY_PASSWORD=<password> \
  -e TLS_ENABLED=true \
  -e TLS_CERT_FILE=/certs/cert.pem \
  -e TLS_KEY_FILE=/certs/key.pem \
  -v /path/to/certs:/certs:ro \
  ghcr.io/astronaut808/socks5-proxy-server:latest
```

При SOCKS5 over TLS обычный SOCKS5-трафик передаётся внутри защищённого TLS-соединения. Если SOCKS5-клиент не умеет TLS напрямую, можно сделать локальную TLS-обёртку через `socat`:

```bash
socat TCP-LISTEN:1080,reuseaddr,fork OPENSSL:<server-ip>:10443,verify=0
curl --socks5 localhost:1080 -U <user>:<password> https://ipinfo.io
```

## Разработка

Структура проекта:

```text
cmd/socks5-proxy-server/    entrypoint приложения
internal/proxy/             config, auth, access policy, listeners, TLS, metrics, server orchestration
.github/workflows/          CI и публикация Docker image
```

Локальные проверки:

```bash
go test ./...
go test -race ./...
go vet ./...
golangci-lint run ./...
```

Сборка локального бинарника:

```bash
go build ./cmd/socks5-proxy-server
```

Включить локальные метрики при разработке:

```bash
METRICS_ADDR=127.0.0.1:9090 go run ./cmd/socks5-proxy-server
curl http://127.0.0.1:9090/debug/vars
```

Сборка и запуск через Docker Compose:

```bash
cp .env.example .env
docker compose -f docker-compose.build.yml up -d --build
```

## Production

Оставляйте аутентификацию включённой, используйте TLS или mTLS в недоверенных сетях, привязывайте proxy к `127.0.0.1`, если он доступен только через туннель, и дополняйте `ALLOWED_IPS` правилами firewall на хосте. Избегайте слишком широких regex для направлений, если вы не хотите получить открытый egress proxy.
