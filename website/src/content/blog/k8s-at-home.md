---
date: "2025-02-28"
title: "Kubernetes at Home"
tags: ["kubernetes", "home-ops"]
---

I've been managing a bare-metal Kubernetes cluster at home for a while now to learn about Kubernetes and its ecosystem. You can check it out here: [solanyn/home-ops](https://github.com/solanyn/home-ops)

## The Cluster

My Kubernetes cluster is deployed with [Talos](https://talos.dev/) and is managed by [Flux](https://fluxcd.io/) via GitOps. I handle backups of the cluster to my NAS via NFS and S3 and offsite to Cloudflare R2.

## Core Components

- Flux: GitOps operator that reconciles cluster state from Git
- Renovate: Automatically updates dependencies and creates PRs via GitHub Actions
- cert-manager: Manages SSL certificates across services
- external-dns: Syncs DNS records for ingresses, services and Gateway API HTTP routes
- external-secrets: Integrates Kubernetes Secrets with 1Password
- cilium: High-performance networking for Kubernetes
- rook: Provides persistent storage for the cluster
- volsync: Provides volume replication and dissaster recovery

## DNS with Split Horizon

I run two instances of `external-dns`:

- One syncs private DNS records to my UniFi router using a webhook provider
- The other pushes public records to Cloudflare

Ingresses/HTTPRoutes are tagged with `internal` or `external` classes to control which DNS provider is used.

## Final Comments

This setup has been a great learning experience and has allowed me to explore the Kubernetes ecosystem in depth. I plan on building and expanding my cluster with compute, deploying more complex platforms and services (looking at you Kubeflow) and building AI/ML workloads on Kubernetes!
