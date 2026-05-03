# Microservices IDS (Intrusion Detection System)

A minimalist, containerized microservices architecture built to practice DevOps, DevSecOps, and basic MLOps principles.

## Architecture

This project is orchestrated via `docker-compose.yml` and consists of two internal services:

*   **Go API Gateway (`go-api`)**: A lightweight REST API built with Go that acts as the single entry point, validating and routing external requests.
*   **Python ML Engine (`python-engine`)**: An isolated backend service running a machine learning inference script (`infer.py`). It does not expose ports to the public internet.

## Technical Highlights

This repository emphasizes infrastructure, automation, and security over complex algorithmic implementation:

*   **Optimized Containerization**: Uses multi-stage Dockerfiles for both services. The Go API is packaged in a minimal environment, and the Python ML Engine is configured to run as a secure, non-root user.
*   **DevSecOps CI/CD**: Automated through GitHub Actions (`.github/workflows/main.yml`). The pipeline builds the images and integrates **Aqua Trivy** for automated vulnerability scanning prior to deployment.

## Quick Start

**1. Build and start the services:**
```bash
docker compose up -d --build
```
2. Verify health status:
```bash
curl http://localhost:8080/health
```
3. Send an inference request:
```Bash
curl -X POST http://localhost:8080/predict \
  -H "Content-Type: application/json" \
  -d '{"features": [0.5, 0.4, 0.6, 0.5, 0.5, 0.5]}'
```
