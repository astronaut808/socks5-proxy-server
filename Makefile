GITLEAKS_LOG_OPTS ?= HEAD
GOVULNCHECK_VERSION ?= v1.9.0
ACTIONLINT_VERSION ?= v1.7.12

.PHONY: test test-race vet build lint lint-docker lint-actions lint-actions-security vuln-check security security-fast

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

build:
	go build ./...

lint:
	golangci-lint run ./...

lint-docker:
	hadolint Dockerfile

lint-actions:
	go run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)

# Требует zizmor (pipx install zizmor). Аудит GitHub Actions на типовые
# проблемы безопасности: template injection, избыточные permissions и т.п.
lint-actions-security:
	zizmor --offline --pedantic .

vuln-check:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

# Полный контур: semgrep с авто-конфигом (медленнее, шире покрытие).
security:
	$(MAKE) vuln-check lint-docker lint-actions lint-actions-security
	gitleaks git --redact --no-banner --log-opts="$(GITLEAKS_LOG_OPTS)" .
	node scripts/check-worktree-secrets.mjs
	trivy fs --scanners vuln --include-dev-deps --severity HIGH,CRITICAL --ignore-unfixed --exit-code 1 --skip-dirs=.kilo,.opencode .
	semgrep --config=auto --disable-version-check --error .

# Быстрый контур для CI: тихий trivy, semgrep только по Go-правилам.
security-fast:
	$(MAKE) vuln-check lint-docker lint-actions lint-actions-security
	gitleaks git --redact --no-banner --log-opts="$(GITLEAKS_LOG_OPTS)" .
	node scripts/check-worktree-secrets.mjs
	trivy fs --scanners vuln --include-dev-deps --severity HIGH,CRITICAL --ignore-unfixed --exit-code 1 --quiet --skip-dirs=.kilo,.opencode .
	semgrep --config=p/golang --disable-version-check --error .
