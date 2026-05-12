"""Toolchain definitions for Helm."""

HelmInfo = provider(
    doc = "Information about how to invoke helm",
    fields = ["helm_bin"],
)

def _helm_toolchain_impl(ctx):
    toolchain_info = platform_common.ToolchainInfo(
        helm_info = HelmInfo(
            helm_bin = ctx.file.helm_bin,
        ),
    )
    return [toolchain_info]

helm_toolchain = rule(
    implementation = _helm_toolchain_impl,
    attrs = {
        "helm_bin": attr.label(allow_single_file = True),
    },
)
