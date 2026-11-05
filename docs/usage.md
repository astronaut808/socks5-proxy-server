# Примеры использования и эксплуатация

[English](usage.en.md)

## Ограничение направлений

Разрешить только конкретные домены (и их поддомены):

```bash
ALLOWED_DEST_FQDN='^(.*\.)?(example\.com|internal\.local)$'
```

Разрешить доступ только с конкретных исходных IP:

```bash
ALLOWED_IPS=192.168.1.10,10.0.0.5
```

## TLS

Включить SOCKS5 over TLS:

```bash
TLS_ENABLED=true
TLS_PORT=10443
TLS_CERT_FILE=/etc/ssl/certs/proxy.crt
TLS_KEY_FILE=/etc/ssl/private/proxy.key
TLS_MIN_VERSION=1.3
```

Запуск контейнера с примонтированными сертификатами:

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

## TLS-обёртка через socat

При SOCKS5 over TLS обычный SOCKS5-трафик передаётся внутри защищённого TLS-соединения. Если SOCKS5-клиент не умеет TLS напрямую, можно поднять локальную TLS-обёртку через `socat`.

Правильный вариант — с проверкой сертификата через CA:

```bash
socat TCP-LISTEN:1080,reuseaddr,fork OPENSSL:<server-ip>:10443,cafile=/path/to/ca.pem
curl --socks5 localhost:1080 -U <user>:<password> https://ipinfo.io
```

Вариант с `verify=0` отключает проверку сертификата и подходит только для локальных тестов, не для реальной эксплуатации:

```bash
# Только для локального тестирования — проверка сертификата отключена.
socat TCP-LISTEN:1080,reuseaddr,fork OPENSSL:<server-ip>:10443,verify=0
```

## Метрики

`METRICS_ADDR` включает стандартный Go `expvar` endpoint на `/debug/vars`. Рекомендуется привязывать его к loopback (`127.0.0.1:9090`) или защищать на сетевом уровне (firewall, приватный интерфейс), так как endpoint не требует аутентификации.

```bash
METRICS_ADDR=127.0.0.1:9090
curl http://127.0.0.1:9090/debug/vars
```

## Production

Оставляйте аутентификацию включённой, используйте TLS или mTLS в недоверенных сетях, привязывайте proxy к `127.0.0.1`, если он доступен только через туннель, и дополняйте `ALLOWED_IPS` правилами firewall на хосте. Избегайте слишком широких regex для направлений, если вы не хотите получить открытый egress proxy. См. [SECURITY.md](../SECURITY.md).
