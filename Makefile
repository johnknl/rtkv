COVERAGE_THRESHOLD=80

.PHONY: nice
nice: lint test-cover check-coverage
	@echo "Nice."

.PHONY: lint
lint:
	@golangci-lint run --config .golangci.yaml

.PHONY: test
test:
	@go test -v ./... -race

test-cover:
	@go test ./... -cover -race -coverprofile=coverage.out -covermode=atomic

check-coverage:
	@COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	if [ "$$(echo $$COVERAGE | cut -d. -f 1)" -lt $(COVERAGE_THRESHOLD) ]; then \
		echo "Coverage ($$COVERAGE%) is below threshold ($(COVERAGE_THRESHOLD)%)."; \
		uncover coverage.out; \
		exit 1; \
	else \
		echo "Coverage OK."; \
	fi

