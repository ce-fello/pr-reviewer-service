# PR Reviewer Assignment Service

### **Greetings**

#### Thank you for taking the time to review my project. I built this service using Go and PostgreSQL, technologies that I believe provide excellent performance, reliability, and scalability for microservices architecture.

#### This service automates the assignment of code reviewers for Pull Requests within teams, ensuring fair distribution and maintaining code quality standards.

## Project Overview

#### This is a microservice for automatic PR reviewer assignment that:
- #### Automatically assigns up to 2 active reviewers from the author's team
- #### Supports safe reviewer reassignment
- #### Prevents changes after PR merge
- #### Manages team members and their activity status
- #### Provides statistics on review assignments

## Technology Stack

- **Language**: Go 1.21+
- **Database**: PostgreSQL
- **API**: RESTful HTTP with OpenAPI 3.0 specification
- **Containerization**: Docker & Docker Compose
- **Testing**: Unit tests + Integration tests

## How to run?

### Prerequisites
- Docker and Docker Compose
- Go 1.21+ (for local development)

### Quick Start with Docker
```bash
# Clone and navigate to project
git clone <repository-url> name
cd name

# Start the service with dependencies
docker-compose up -d
```

#### The service will be available at:
`http://localhost:8080`

### Or use Makefile
```make run```

## API Documentation

The service provides OpenAPI 3.0 specification at openapi.yml. Key endpoints:

    POST /team/add - Create team with members

    GET /team/get - Get team information

    POST /pullRequest/create - Create PR with auto-assigned reviewers

    POST /pullRequest/reassign - Reassign reviewer

    POST /pullRequest/merge - Merge PR (idempotent)

    POST /users/setIsActive - Set user activity status

    GET /users/getReview - Get PRs assigned to user

    GET /health - Health check

    Get /stats - Get simple statistics data

## How to run tests?

### Unit tests
```
cd src
go test -v ./internal/service/... -cover
```
#### Or use Makefile
```
make test-unit
```
### Integration Tests
#### Make sure service is running first
```
docker-compose up -d
```

#### Run integration tests
```
go test -v ./tests/integration_test.go -tags=integration
```

#### Or use Makefile  
```
make test-integration
```

## All Tests
```
make test-all
```

## Load Testing

#### The service has been thoroughly load tested using k6 to ensure it meets performance requirements.

![load testing.png](resources/load%20testing.png)

#### The service demonstrates good performance

## Configuration

    Port: 8080 (configurable via PORT environment variable)

    Database: PostgreSQL with connection pooling

    Logging: Structured JSON logging with request ID tracking

    Timeouts: 5-second request timeout, 300ms SLI target

## Final Notes

#### Thank you for reviewing my implementation of the PR Reviewer Assignment Service. I've designed the system with a focus on clean architecture, reliability, and performance. The codebase follows Go best practices with proper separation of concerns, comprehensive testing, and production-ready error handling.

#### I welcome any feedback or questions about the architecture, implementation details, or design decisions.

#### Looking forward to discussing this project further!

## License

#### This project is provided for educational purposes. Adapt as needed for your own use.