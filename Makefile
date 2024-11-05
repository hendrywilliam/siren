run:
	go run -race main.go

build:
	go build -o main main.go

build_container:
	docker buildx build -f build/Dockerfile -t siren:latest .

run_container:
	docker run -it -p 8080:8080 siren:latest

.PHONY: run build build_container run_container