.PHONY: check check-internal fmt vet lint test benchmark run scan

GO ?= go

ifeq ($(shell command -v $(GO) 2>/dev/null),)
check:
	./scripts/in-docker check-internal
else
check: check-internal
endif

check-internal: fmt vet lint test benchmark scan

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './.git/*'))"

vet:
	$(GO) vet ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run ./...; else $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...; fi

test:
	@mkdir -p /tmp/go-shortener
	$(GO) test ./... -count=1 -coverprofile=/tmp/go-shortener/coverage.out
	@$(GO) test ./internal/domain -cover | tee /tmp/go-shortener/domain-coverage.txt
	@awk '{for (i=1;i<=NF;i++) if ($$i=="coverage:") {value=$$(i+1); gsub("%", "", value); if (value+0 < 70) {print "domain coverage below 70%"; exit 1}}}' /tmp/go-shortener/domain-coverage.txt

benchmark:
	$(GO) test ./internal/httpapi -run '^$$' -bench BenchmarkRedirectPath -benchtime=1s

scan:
	./scripts/scan-forbidden.sh

run:
	docker compose up --build
