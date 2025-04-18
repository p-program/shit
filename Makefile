
run:
	docker compose up --build

build:
	docker compose build --no-cache

bbuild:
	go build