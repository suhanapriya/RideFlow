# Contributing to Ride Hailing Platform

First off, thank you for considering contributing to the platform! We aim for high engineering standards, and this guide will help you align with our processes.

## Development Setup

### Prerequisites
- Go 1.24 or later
- Docker and Docker Compose
- `make` (GNU Make)
- `golangci-lint`

### Environment Bootstrap
1. Clone the repository:
   ```bash
   git clone https://github.com/richxcame/ride-hailing.git
   cd ride-hailing
   ```
2. Prepare configuration:
   ```bash
   cp .env.example .env
   ```
3. Boot infrastructure (Postgres, Redis, NATS) and run migrations:
   ```bash
   make setup
   # Under the hood, this runs `make dev-infra` and `make migrate-up`
   ```
4. Run your target service in dev mode:
   ```bash
   make dev SERVICE=auth
   ```

## Code Style & Linting

We enforce strict Go idioms.
- Run `gofmt` and `goimports` on all code before committing.
- CI will fail if `golangci-lint` detects issues. Run it locally:
  ```bash
  make lint
  ```
- Keep cyclomatic complexity low. Favour early returns.

## Project Structure Conventions

We utilize a structured domain-driven layout:
- **`cmd/<service>`**: Main package, wiring, and configuration initialization.
- **`internal/<domain>`**: 
  - Follows the **Handler -> Service -> Repository** pattern.
  - Dependencies are injected via interfaces (Interface-based DI).
  - Business logic lives strictly in the Service layer.
- **`pkg/`**: Reusable libraries (e.g., `pkg/resilience`, `pkg/logger`) containing no business logic.

## Testing Requirements

We mandate high test coverage and robust testing methodologies:
1. **Unit Tests**: Required for all business logic (`internal/`) and utility functions (`pkg/`).
2. **Table-Driven Tests**: Use table-driven tests for comprehensive input/output validation.
3. **Mocks**: Generate mocks using `mockery` for interfaces. Do not hit real DB/Redis instances in unit tests.
4. **Integration Tests**: Place these in `test/integration/`. These *do* connect to test infrastructure.

```bash
# Run all tests
go test ./...

# Check coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Commit Message Conventions

We adhere to [Conventional Commits](https://www.conventionalcommits.org/). This allows for automated changelog generation and semantic versioning.

Examples:
- `feat(auth): add JWT key rotation`
- `fix(geo): resolve redis timeout during high load`
- `docs: update architecture diagram`
- `chore(deps): bump pgx to v5.4.3`

## Pull Request Process

1. Create a feature branch off `main` (`feature/name` or `fix/name`).
2. Ensure your code passes `make lint` and `make test`.
3. Open a Pull Request with a clear description, linking to relevant issues.
4. CI will run (Lint, Test, Codecov). Ensure all checks pass.
5. Request a review from at least one core maintainer.
6. Once approved, squash and merge into `main`.

## Adding a New Service Checklist

When scaffolding a new microservice in this platform, ensure the following checklist is completed:

- [ ] Created `cmd/<new-service>/main.go` with graceful shutdown logic.
- [ ] Added `internal/<new-service>` with layered architecture (Handler/Service/Repo).
- [ ] Registered the service in `docker-compose.yml`.
- [ ] Configured environment variables in `.env.example` and config loaders.
- [ ] Set up OpenTelemetry tracing and Prometheus metrics endpoints.
- [ ] Added standard `/health/liveness` and `/health/readiness` routes.
- [ ] Created necessary `db/migrations/` and generated DB models.
- [ ] Registered routing upstream in Kong API Gateway (`kong/kong.yml`).
- [ ] Added a `make run-<new-service>` command in the `Makefile`.
- [ ] Updated `README.md` and `docs/ARCHITECTURE.md` tables/diagrams.
