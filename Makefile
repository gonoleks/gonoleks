## help: Display available commands
.PHONY: help
help:
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## audit: Conduct quality checks
.PHONY: audit
audit:
	go mod verify
	go vet ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

## benchmark: Benchmark code performance
.PHONY: benchmark
benchmark:
	go test ./... -benchmem -bench=. -v -run=^Benchmark_$

## coverage: Generate coverage report
.PHONY: coverage
coverage:
	go run gotest.tools/gotestsum@latest -f testname -- ./... -race -count=1 -coverprofile=/tmp/coverage.out -covermode=atomic
	go tool cover -html=/tmp/coverage.out

## format: Fix code format issues
.PHONY: format
format:
	go run mvdan.cc/gofumpt@latest -w -l .

## markdown: Find markdown format issues (Requires markdownlint-cli2)
.PHONY: markdown
markdown:
	markdownlint-cli2 "**/*.md" "#vendor"

## lint: Run lint checks
.PHONY: lint
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run ./...

## test: Execute all tests
.PHONY: test
test:
	go run gotest.tools/gotestsum@latest -f testname -- ./... -race -count=1 -shuffle=on

## longtest: Execute all tests 10x
.PHONY: longtest
longtest:
	go run gotest.tools/gotestsum@latest -f testname -- ./... -race -count=10 -shuffle=on

## tidy: Clean and tidy dependencies
.PHONY: tidy
tidy:
	go mod tidy -v

## betteralign: Optimize alignment of fields in structs
.PHONY: betteralign
betteralign:
	go run github.com/dkorunic/betteralign/cmd/betteralign@latest -test_files -generated_files -apply ./...

## modernize: Run modernize analysis
.PHONY: modernize
modernize:
	go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest -fix -test ./...

## generate: Generate code
.PHONY: generate
generate:
	go generate ./...
