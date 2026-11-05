**Русский** | [English](README.en.md)

# SOCKS5 Proxy Server

![Build and Push Docker image](https://github.com/astronaut808/socks5-proxy-server/actions/workflows/docker.yml/badge.svg)

Небольшой SOCKS5 proxy server на `go-socks5` и современном Go (версия — в `go.mod`). Поддерживает аутентификацию по логину и паролю, несколько аккаунтов, allowlist по исходным IP, политики доступа по FQDN назначения, лимиты соединений, rate limiting, обычный SOCKS5, SOCKS5 over TLS и опциональный mutual TLS.

## Быстрый старт

```bash
docker run -d --name socks5 \
  -p 1080:1080 \
  -e PROXY_USER=<user> \
  -e PROXY_PASSWORD=<password> \
  ghcr.io/astronaut808/socks5-proxy-server:latest
```

Проверка через proxy:

```bash
curl --socks5 <proxy-host>:1080 -U <user>:<password> https://ipinfo.io
```

Также можно использовать Docker Compose: скопируйте `.env.example` в `.env` и выполните `docker compose up -d` (см. `docker-compose.yml` и `docker-compose.build.yml`).

## Конфигурация

| Переменная | Тип | По умолчанию | Обязательность | Описание |
| --- | --- | --- | --- | --- |
| `REQUIRE_AUTH` | Boolean | `true` | нет | Требовать SOCKS5-аутентификацию по логину и паролю. Отключать не рекомендуется, если proxy не защищён другим способом. |
| `PROXY_USER` | String | пусто | условно | Логин для одного пользователя. Нужен вместе с `PROXY_PASSWORD`, когда `REQUIRE_AUTH=true` и `PROXY_ACCOUNTS` пустой. |
| `PROXY_PASSWORD` | String | пусто | условно | Пароль для одного пользователя. Нужен вместе с `PROXY_USER`, когда `REQUIRE_AUTH=true` и `PROXY_ACCOUNTS` пустой. Plaintext-пароль хэшируется в памяти при запуске. |
| `PROXY_ACCOUNTS` | String | пусто | условно | Список аккаунтов через запятую в формате `user:password` или `user:bcrypt-hash`. Нужен, когда `REQUIRE_AUTH=true` и одиночные credentials не заданы. |
| `ENABLE_PLAIN` | Boolean | `false` | нет | Обычный SOCKS5 listener по умолчанию включён. Когда включён TLS, plain listener отключается; выставите `true`, только если намеренно хотите оставить оба listener (plain и TLS). |
| `PROXY_PORT` | String | `1080` | нет | Порт обычного SOCKS5 listener. Значение должно быть от 1 до 65535. |
| `PROXY_LISTEN_IP` | String | `0.0.0.0` | нет | Адрес прослушивания. Для доступа только с локальной машины используйте `127.0.0.1`. |
| `ALLOWED_DEST_FQDN` | String | пусто | нет | Регулярное выражение для FQDN назначения. Пустое значение разрешает все направления. Лучше использовать anchored regex, например `^(.*\.)?(example\.com\|internal\.local)$`. Паттерн применяется только к FQDN: назначения, заданные IP-литералом, отклоняются, если паттерн задан. |
| `ALLOWED_IPS` | String | пусто | нет | Allowlist исходных IP-адресов через запятую. |
| `ENABLE_UDP_ASSOCIATE` | Boolean | `false` | нет | Разрешить команду SOCKS5 UDP ASSOCIATE. По умолчанию отклоняется на уровне политики: большинству развёртываний нужен только TCP CONNECT, а UDP расширяет поверхность атаки. Включайте только если клиентам действительно нужен UDP. |
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
| `METRICS_ADDR` | String | пусто | нет | Опциональный адрес `host:port` для стандартного Go `expvar` endpoint на `/debug/vars`. Пустое значение отключает метрики. Рекомендуется привязывать к loopback (`127.0.0.1:9090`) или защищать на сетевом уровне. |

## Документация

- [Примеры использования и эксплуатация](docs/usage.md) — TLS, socat-обёртка, метрики, production-заметки.
- [Разработка](docs/development.md) — структура проекта, тесты, сборка.
- [Безопасность](SECURITY.md) — политика и рекомендации по раскрытию уязвимостей.
