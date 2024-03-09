build:
	go build -o bin/api cmd/main.go

clean:
	rm -rf bin/

test:
	go test ./cmd/... -v

run:
	go run cmd/main.go

up:
	docker compose up

down:
	docker compose down --volumes

build-image:
	docker build -t andrenbrandao/rinha-de-backend-2024-q1-api -f Dockerfile .

push-image:
	docker push andrenbrandao/rinha-de-backend-2024-q1-api

