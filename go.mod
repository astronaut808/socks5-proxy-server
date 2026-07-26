module github.com/astronaut808/socks5-proxy-server

go 1.26.3

require github.com/armon/go-socks5 v0.0.0-20160902184237-e75332964ef5

require (
	github.com/ilyakaznacheev/cleanenv v1.5.0
	golang.org/x/crypto v0.54.0
	golang.org/x/time v0.15.0
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	golang.org/x/net v0.56.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	olympos.io/encoding/edn v0.0.0-20201019073823-d3554ca0b0a3 // indirect
)

replace github.com/armon/go-socks5 => github.com/serjs/go-socks5 v0.0.0-20250923183437-3920b97ee0d2
