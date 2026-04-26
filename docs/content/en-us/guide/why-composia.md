# Why Composia?

Composia is built around one idea: **your Compose files should always be the source of truth.** It adds multi-node coordination and operational visibility as an enhancement layer — your files stay yours, your CLI workflows keep working, and you can walk away from Composia at any time without losing your infrastructure.

In practice, this means:

- You should still be able to understand your system from the repo itself
- You should still be able to operate services with the appropriate CLI tools
- You should be able to move away from Composia without first reverse-engineering a private platform model
- Composia should coordinate, validate, execute, and report, not take ownership away from the operator

## What Composia Is

Composia is a self-hosted Docker Compose management platform built around:

- Git-backed service definitions
- Centralized coordination
- One or more execution agents
- Multi-node service deployment
- Task execution, repo writes, secrets, backup, restore, and operational visibility

It is not just a web editor for `compose.yaml`, and it is not a self-hosted clone of a cloud PaaS.

## Why Not Just Use a Compose Manager?

Projects like Dockge and Dockman are strong choices when you mainly want a better interface for managing Compose files.

They usually optimize for:

- Editing `compose.yaml` comfortably in the browser
- Starting, stopping, and updating stacks quickly
- Preserving direct access to Compose files
- Lightweight operation for a single host or a small number of hosts

That is valuable, but Composia aims at a different layer.

Composia is for cases where you need more than a stack UI:

- A controller-agent architecture instead of direct local stack management
- A service model that can target multiple nodes
- Structured task execution and logs
- Git-backed desired state changes with revision checks
- A system-wide view across services, instances, containers, and nodes

In short:

- Dockge and Dockman help you manage Compose more comfortably
- Composia helps you run Compose as managed infrastructure

## Why Not Use a Self-Hosted PaaS?

Projects like Dokploy and Coolify solve a different problem.

They position themselves as self-hosted alternatives to products like Heroku, Netlify, or Vercel. Their value is usually centered on:

- Deploying applications from Git repositories
- Managing apps, databases, domains, certificates, and templates from one platform
- Providing more platform automation and a more PaaS-like experience
- Letting the platform carry more of the operational model

That can be the right tradeoff when you want a deployment platform.

Composia is intentionally not built around that product philosophy.

Composia does not start from the assumption that the platform should become the primary interface to your application model. Instead, it starts from the assumption that:

- Docker Compose remains the operational foundation
- Files remain the source of truth for desired state
- Operators should retain direct control over the underlying system
- The management layer should stay removable

In short:

- Dokploy and Coolify are self-hosted PaaS platforms
- Composia is a Compose-native coordination layer for self-hosted infrastructure

## Where Composia Sits

Composia sits between two common categories:

- Simpler Compose managers
- Higher-level self-hosted PaaS platforms

It keeps the Compose-native, file-first, operator-controlled model that many people want, while adding the coordination capabilities that lightweight stack managers usually do not provide.

That balance is the point.

## Choose Composia If You Want

- A multi-node Docker Compose management platform
- Git-backed desired state instead of opaque platform state
- A system that respects normal CLI and file-based workflows
- Clear service, instance, container, and node boundaries
- Coordination, observability, and operational tooling without giving up low-level control

## Summary

If Dockge or Dockman are primarily better ways to operate Compose stacks, and Dokploy or Coolify are primarily self-hosted PaaS platforms, then Composia is trying to be something else:

**A multi-node Docker Compose coordination layer that works with your files, not instead of them.**
