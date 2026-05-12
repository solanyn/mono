"""Rules to package and push Helm charts as OCI artifacts."""

def _helm_chart_impl(ctx):
    chart_dir = ctx.actions.declare_directory(ctx.label.name + "_chart")
    output = ctx.actions.declare_file(ctx.attr.chart_name + "-" + ctx.attr.version + ".tgz")

    helm_info = ctx.toolchains["//tools/helm-oci:toolchain"].helm_info

    templates_cmds = []
    for f in ctx.files.templates:
        templates_cmds.append("cp {src} {dir}/templates/{basename}".format(
            src = f.path,
            dir = chart_dir.path,
            basename = f.basename,
        ))

    crds_cmds = []
    for f in ctx.files.crds:
        crds_cmds.append("cp {src} {dir}/crds/{basename}".format(
            src = f.path,
            dir = chart_dir.path,
            basename = f.basename,
        ))

    cmd = """
        mkdir -p {dir}/templates {dir}/crds
        cat > {dir}/Chart.yaml <<'CHART'
apiVersion: v2
name: {name}
version: {version}
description: {description}
type: application
CHART
        {templates_cmds}
        {crds_cmds}
        {helm} package {dir} -d $(dirname {output})
    """.format(
        dir = chart_dir.path,
        name = ctx.attr.chart_name,
        version = ctx.attr.version,
        description = ctx.attr.description,
        templates_cmds = "\n        ".join(templates_cmds),
        crds_cmds = "\n        ".join(crds_cmds),
        helm = helm_info.helm_bin.path,
        output = output.path,
    )

    ctx.actions.run_shell(
        mnemonic = "HelmPackage",
        outputs = [chart_dir, output],
        inputs = ctx.files.templates + ctx.files.crds,
        tools = [helm_info.helm_bin],
        command = cmd,
    )

    return DefaultInfo(files = depset([output]))

helm_chart = rule(
    implementation = _helm_chart_impl,
    attrs = {
        "chart_name": attr.string(mandatory = True),
        "version": attr.string(mandatory = True),
        "description": attr.string(default = ""),
        "templates": attr.label_list(allow_files = True),
        "crds": attr.label_list(allow_files = True),
    },
    toolchains = ["//tools/helm-oci:toolchain"],
)

def _helm_oci_push_impl(ctx):
    helm_info = ctx.toolchains["//tools/helm-oci:toolchain"].helm_info
    chart_file = ctx.file.chart

    script = ctx.actions.declare_file(ctx.label.name + "_push.sh")
    ctx.actions.write(
        output = script,
        content = """#!/bin/bash
set -euo pipefail
CHART="{chart}"
REPO="{repository}"
if [ -n "${{HELM_REGISTRY_USERNAME:-}}" ] && [ -n "${{HELM_REGISTRY_PASSWORD:-}}" ]; then
    REGISTRY=$(echo "$REPO" | cut -d/ -f1)
    {helm} registry login "$REGISTRY" --username "$HELM_REGISTRY_USERNAME" --password "$HELM_REGISTRY_PASSWORD"
fi
{helm} push "$CHART" "oci://$REPO"
""".format(
            chart = chart_file.short_path,
            repository = ctx.attr.repository,
            helm = helm_info.helm_bin.short_path,
        ),
        is_executable = True,
    )

    runfiles = ctx.runfiles(files = [chart_file, helm_info.helm_bin])
    return DefaultInfo(
        executable = script,
        runfiles = runfiles,
    )

helm_oci_push = rule(
    implementation = _helm_oci_push_impl,
    attrs = {
        "chart": attr.label(allow_single_file = True, mandatory = True),
        "repository": attr.string(mandatory = True),
    },
    executable = True,
    toolchains = ["//tools/helm-oci:toolchain"],
)
