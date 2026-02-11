---
date: "2025-06-17"
title: "Bazel Remote Build Execution"
tags: ["kubernetes", "home-ops", "bazel"]
---

Bazel is pretty cool, it:

- Is a build tool, with `rules` for language support e.g., [`rules_python`](https://github.com/bazel-contrib/rules_python)
- All about that "hermeticity". How they get there is too big brain for me.
- Made by Google after their internal Blaze build tool, so really made to go BRRR

Because of all this hermeticity stuff messages are ultra cryptic and who knows what they even mean. Tried using it at work but gave up because it's _pain_ to even decode what error messages are saying when you just want something to work and I was pretty much alone in using it.

Seems like the kind of thing which is worth if an org has big investment in engineering infra and talent.

I'm gonna give it another go. Might be more fun this time around and some rules have come a long way. For RBE platforms, there are not many self-hosted options but here's what I found:

- [buildbarn/buildbarn](https://github.com/buildbarn/bb-deployments)
  - Kinda nice but k8s skill issue didn't really know how to get it work a couple of years ago, I could probably figure it out now. Still not many examples to get the remote build part up and running. Written in go and configured in jsonnet.
- [buildfarm/buildfarm](https://github.com/buildfarm/buildfarm)
  - Has an OCI chart which would be easy to deploy but not many examples. Also hosting Java stuff kinda sucks because they love to eat RAM. Also not many examples. This is the original Bazel remote build but I guess they split off at some point.
- [TraceMachina/nativelink](https://github.com/TraceMachina/nativelink)
  - Think this used to be turbocache (a Rust-based CAS) but I guess they went and rebranded and now have a full RBE solution which is cool. Might try deploying this.

The play is to:

1. Full send Claude Code on bashing its head against a monorepo until it figures out how to build things with Bazel
2. Deploy a nativelink cluster, maybe fast-slow storage with memory as fast and Dragonfly/Redis as slow storage (lower) + NFS for slower storage (upper)
3. ???
4. Profit
