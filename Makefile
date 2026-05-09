.PHONY: build test lint install clean plugin-install

build:
	go build -o bin/kubectl-sick ./main.go

test:
	go test ./... -v

lint:
	golangci-lint run

install:
	go install ./...

plugin-install: build
	cp bin/kubectl-sick $(GOPATH)/bin/kubectl-sick

clean:
	rm -rf bin/
