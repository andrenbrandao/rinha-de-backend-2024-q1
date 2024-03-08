build:
	go build -o bin/api main.go

clean:
	rm -rf bin/

test:
	go test -v

run:
	go run main.go

up:
	docker compose up

down:
	docker compose down --volumes

build-image:
	docker build -t andrenbrandao/rinha-de-backend-2024-q1-api -f Dockerfile .

push-image:
	docker push andrenbrandao/rinha-de-backend-2024-q1-api

