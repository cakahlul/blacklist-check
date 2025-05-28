# Blacklist Check Service

A Go service that checks if a given NIK (National Identity Number) is blacklisted.

## Features

- REST API endpoint for blacklist checking
- gRPC service for high-performance lookups
- PostgreSQL database with prepared statements
- Redis caching for hot lookups
- Structured logging with zap
- Prometheus metrics
- Docker and docker-compose support
- Database migrations with golang-migrate

## Prerequisites

- Go 1.21 or later
- Docker and docker-compose
- Make (optional, for using Makefile commands)

## Setup

1. Clone the repository:

```bash
git clone https://github.com/yourusername/blacklist-check.git
cd blacklist-check
```

2. Install dependencies:

```bash
make setup
```

3. Start the services using Docker:

```bash
make docker-up
```

4. Run database migrations:

```bash
make migrate-up
```

## Development

### Running Locally

1. Start the dependencies (PostgreSQL and Redis):

```bash
docker-compose up -d postgres redis
```

2. Run the application:

```bash
make run
```

### API Usage

#### Check Blacklist

```bash
curl -X POST http://localhost:8080/api/v1/blacklist \
  -H "Content-Type: application/json" \
  -d '{"nik": "1234567890123456"}'
```

Response:

```json
{
  "blacklisted": true,
  "details": {
    "name": "John Doe",
    "birth_place": "Jakarta",
    "birth_date": "1990-01-01",
    "description": "Some description"
  }
}
```

#### Health Check

```bash
curl http://localhost:8080/healthz
```

## Testing

Run the test suite:

```bash
make test
```

## Docker

Build and run using Docker:

```bash
make docker-build
make docker-up
```

## License

MIT
