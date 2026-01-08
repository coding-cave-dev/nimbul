# Nimbul ☁️🌪️  
A self-hosted, Kubernetes-native platform for deploying apps on your own infrastructure.

Nimbul is a learning + portfolio project focused on **platform engineering**: provisioning infrastructure, running a Kubernetes cluster, and building a small “PaaS control plane” that can deploy apps, expose them via HTTPS, and support day-2 operations like logs, rollouts, and rollbacks.

> **Scope (Phase 1):** one self-hosted cluster on Hetzner — **1 control plane + 2 workers** — deployed via Terraform + Ansible, running **k3s**.

---

## Goals

### Primary
- Learn Kubernetes by building a real platform-style system
- Become job-ready for Platform/Infra roles by shipping an end-to-end project:
  - Provision infra
  - Bootstrap k3s
  - Install platform addons (ingress, TLS, GitOps)
  - Build a control plane + CLI to deploy apps to the cluster

### Non-goals (for now)
- Multi-cloud support
- Multi-cluster control plane
- Buildpacks / source-based builds
- Billing / teams / auth (beyond basics)
- “Vercel-level” edge features

---

## High-level architecture (Phase 1)

- **Hetzner Cloud**: hosts the cluster nodes
- **k3s**: Kubernetes distribution
- **Ingress**: Traefik (initially) or NGINX later
- **cert-manager**: automatic TLS via Let’s Encrypt
- **Argo CD (optional early, recommended)**: GitOps for cluster addons and platform services
- **Nimbul API**: control plane that creates K8s resources for apps
- **Nimbul CLI**: developer interface for deploying and managing apps

A typical deploy will create/update:
- Namespace (per app or shared `apps` namespace)
- Deployment
- Service
- Ingress (host-based routing)
- TLS via cert-manager

