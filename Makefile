
default: run

.PHONY: run
run:
	@go run pkg/cmd/main.go

docker-clean:
	docker network prune -f && docker rm $$(docker ps -aq) -f
