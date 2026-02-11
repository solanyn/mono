---
date: "2025-05-06"
title: "Deploying Kubeflow"
tags: ["machine-learning", "kubernetes"]
---

### Preamble

I recently started contributing to the Kubeflow open-source project and have been following some recent GitHub discussions, watched Working Group (WG) call recordings (they run past midnight in my timezone unfortunately) and community calls. Many contributors come from organizations that deploy Kubeflow internally or offer it as a managed service. One of the more popular requests from the community is to build Helm charts for deploying the Kubeflow platform.

### What on earth is Kubeflow?

The Kubeflow platform is a collection of machine learning tools designed to be deployed on Kubernetes. Kubeflow is deployed via a community-maintained repository, kubeflow/manifests, which provides a collection of Kustomize manifests for each component. These manifests include Istio configuration, pod security standards, network policies, and token-based authorization. The repository holds a collection of Kustomize manifests upstreamed from each Kubeflow component. It also implements an [Istio](https://istio.io) service mesh which secures traffic between components along with other security measures like pod security standards, network policies, and authorization tokens. It can also be configured to use an external OAuth2 provider. The scope of designing Kubeflow Helm charts is enormous and no easy effort, which makes sense why the Kubeflow organisation chooses not to maintain a central distribution of Kubeflow. Like many open-source projects, contributions often come from developers funded by their organizations or volunteers contributing in their free time. Kubeflow is already extremely complex to maintain to be security compliant for enterprises and has its own challenges designing and coordinating AI/ML tooling on Kubernetes. Vendors tend to provide enterprise support for deploying the platform.

I recently went through deploying a customised Kubeflow. I operate a [bare-metal Kubernetes cluster](https://github.com/solanyn/home-ops) using GitOps powered by [Flux](https://fluxcd.io). A Flux controller in my cluster reads the latest changes in the repository and synchronises changes to my cluster. I usually reach for Helm charts since I find that they are the simplest to configure but occasionally deploy operators and/or Custom Resource Definitions (CRDs) from manifests if this is not an option. I recently moved away from [Ingress NGINX Controller](https://kubernetes.github.io/ingress-nginx/) to the new [Gateway API](https://gateway-api.sigs.k8s.io) using the [Cilium Gateway implementation](https://docs.cilium.io/en/stable/network/servicemesh/gateway-api/gateway-api/). This posed challenges because Istio's Gateway implementation and the Gateway API's evolving spec introduced compatibility and configuration issues. This should be resolved with the next Kubeflow platform release.

Deploying the Kubeflow platform has a few external dependencies:

- S3-compatible storage
- MySQL/MySQL-compatible database
- Istio service mesh
- Cert Manager for certificates

I already deploy Cert Manager for certificates for my split-brain DNS setup to publicly expose services via Cloudflare DNS and Cloudflare tunnels and internal (only on my network) using External DNS and the external-dns-unifi-webhook and a MinIO deployment with an NFS backend to my NAS. I settled on MariaDB Operator for its convenient CRDs to create databases, users and grants and scheduled S3 backup and restore procedures.

### Kubeflow and Fluxtomizations

I largely used the [deployKF](https://github.com/deployKF/deployKF) repository as inspiration which is a more configurable setup using gomplate, ArgoCD and raw bash scripts to configure settings. It's also a couple of revisions old (my guess is the maintainer started vendoring the distribution for later Kubeflow versions). It uses the same mechanism Helm values are templated into charts to create customised deployments AND kustomizations. I ended up opting for configurable options through Kubernetes Secret and ConfigMap objects. One of the biggest challenges was identifying all the points where MySQL and MinIO credentials were needed and using Kustomize patches to use a centralised secret. I should mention that secrets can be populated manually, using GitOps with [sops](https://github.com/getsops/sops) encryption (natively supported in Flux) or use [External Secrets Operator](https://external-secrets.io/latest/) to populate secrets from an external secret store. I personally use 1Password to populate my secrets which allows me to conveniently autofill credentials.

Roughly speaking I went through the following steps:

1. Write a GitHub Actions workflow to commit the latest non-release candidate tag of the manifests to my repository
2. Add components to generate namespaces with the appropriate labels e.g., for Istio injection
3. Configure secrets and OAuth2 settings for `dex` and `oauth2-proxy` using ExternalSecret and ConfigMap resources
4. Add Kustomize patches to patch `dex` and `oauth2-proxy` deployments to mount the the configuration from secrets and/or configmaps
5. Add an ExternalSecret manifest with configurations for ALL Kubeflow components
6. Create Database objects for Kubeflow components that use MySQL/MySQL-compatible databases
7. Add Kustomize patches to patch Kubeflow components (mainly Katib, Pipelines and Model Registry) to connect to my MariaDB cluster and the appropriate tables
8. Add Kustomize patches to patch Kubeflow Pipelines to connect to my MinIO deployment
9. Add any additional Istio RequestAuthentication manifests to enable authorization from GitHub and Pocket ID

### Conclusion

While future changes to the manifests could require rework, using Kustomize patches makes changes relatively easy to reconfigure. Being a contributor helps here, I would be aware of any breaking changes that could be introduced. Looking forward to the ongoing improvements like Gateway API support and Istio Ambient mode and how things will break in my cluster! Thanks for reading.
