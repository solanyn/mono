---
title: Adding Cloud GPUs to My Home Kubernetes Cluster
description: How I added GPU burst computing to my home lab using Liqo and GKE spot instances
date: 2026-02-26
draft: true
tags: [kubernetes, gpu, liqo, gke, home-lab]
---

I run a small Kubernetes cluster at home — three low-power Talos nodes, enough for most things. But when I want to train models or run GPU workloads, I am stuck. No GPUs, no budget for a dedicated card that sits idle 99% of the time.

The dream: burst to cloud GPUs when needed, scale to zero when not, keep everything GitOps-friendly. Here is what I tried.

## The Detours

### Tensor Processing Group Operator

First attempt was tgp-operator — seemed perfect for multi-cloud GPU access. Hit walls fast: had to build network fabric from scratch, Talos added complexity (no standard kubelet paths), provider API fragmentation meant lots of glue code, prepaid credits model — did not want to front cash for capacity I might not use.

Parked it.

### Modal

Pivoted to Modal for specific services — LLM inference, immich-ml, that kind of thing. Actually works great for those use cases. But it is not general-purpose Kubernetes. Cannot just schedule arbitrary pods there.

Good tool, wrong shape for what I wanted.

## Liqo + GKE: The Win

Liqo does cluster federation — your home cluster peers with a remote cluster, and workloads can be scheduled across both transparently. The remote nodes show up as virtual nodes in your cluster.

Paired it with GKE using spot instances:
- Home cluster: always on, control plane, lightweight workloads
- GKE: GPU node pool with spot T4s, scales to zero when idle

### Cost Model

- Home cluster: already paying for it
- GKE control plane: ~$70/mo (autopilot would be cheaper but less control)
- GPU nodes: spot T4 ~$0.35/hr, only when running

The key insight: GKE spot instances are actually pay-as-you-go with real scale-to-zero. No prepaid credits, no minimum commitment. That is the win over smaller GPU cloud providers.

## What is Next

Still setting up the Terraform/OpenTofu for the GKE side. Once that is solid, I will add Kubeflow pipelines that transparently burst GPU training jobs to cloud. The home cluster stays the brain, cloud is just muscle when needed.

---

Repo: github.com/solanyn/home-ops
