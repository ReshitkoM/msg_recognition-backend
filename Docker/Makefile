all: build run
build:
	docker build -t ms -f $(PATH_TO_MODEL_SERVER)/Dockerfile $(PATH_TO_MODEL_SERVER)
	docker build -t backend ..
run:
	docker-compose up