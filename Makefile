
default: run

run:
	@go run pkg/cmd/main.go

build:
	@go build pkg/cmd/main.go

