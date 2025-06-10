.PHONY: build
build:
	go build ./...

.PHONY: install
install:
	go install ./...

.PHONY: test
test:
	go test -v ./...

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: clean
clean:
	go clean
	rm -f bin/* 