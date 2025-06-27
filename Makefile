.PHONY: test
test :
	go test -v -timeout=10m ./...


.PHONY: lint
lint :
	go run github.com/golangci/golangci-lint/cmd/golangci-lint --timeout=5m run
	gopls check $$(find . -name "*.go" -type f)
