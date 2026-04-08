# Quick Start

This guide will help you get Composia up and running in minutes.

## Prerequisites

Before starting, make sure you have the following tools installed:

- Go 1.25+
- Bun 1.3+
- Docker Engine + Docker Compose v2
- SQLite3
- Git

## Installation

### 1. Clone the repository

```bash
git clone https://forgejo.alexma.top/alexma233/composia.git
cd composia
```

### 2. Install frontend dependencies

```bash
bun install
```

### 3. Initialize the controller repository

```bash
mkdir -p ./repo-controller && git -C ./repo-controller init
```

## Start Services

### Start the Control Plane

```bash
go run ./cmd/composia controller -config ./configs/config.controller.dev.yaml
```

### Start the Agent

In another terminal:

```bash
go run ./cmd/composia agent -config ./configs/config.agent.dev.yaml
```

### Start the Frontend Development Server

```bash
bun run dev
```

## Access the Interface

Open your browser and visit http://localhost:5173 to view the web interface.

## Next Steps

- Learn about the [Architecture](./architecture)
- Check the API documentation
- Deploy your first service
