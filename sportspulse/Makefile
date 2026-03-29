.PHONY: run stop build logs clean

# Start full stack locally
run:
	docker compose up --build

# Start without Kafka (Experiment 1 direct-write variant)
run-direct:
	docker compose -f docker-compose.direct.yml up --build

# Stop all containers
stop:
	docker compose down

# Build all service images
build:
	docker compose build

# Tail logs for a specific service: make logs svc=event-svc
logs:
	docker compose logs -f $(svc)

# Remove containers, volumes, and networks
clean:
	docker compose down -v --remove-orphans

# Run go mod tidy in all services
tidy:
	cd event-svc && go mod tidy
	cd stats-worker && go mod tidy
	cd query-svc && go mod tidy