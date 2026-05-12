"""Rules to run controller-gen for CRD/object/RBAC generation."""

load("@rules_go//go:def.bzl", "go_context", "go_path")
load("@rules_go//go/private:providers.bzl", "GoPath")

def _controller_gen_action(ctx, cg_cmd, outputs, output_path):
    go_ctx = go_context(ctx)
    cg_info = ctx.toolchains["//tools/controller-gen:toolchain"].controller_gen_info
    gopath = ""
    if ctx.attr.gopath_dep:
        gopath = "$(pwd)/" + ctx.bin_dir.path + "/" + ctx.attr.gopath_dep[GoPath].gopath

    if hasattr(ctx.attr, "paths") and ctx.attr.paths:
        paths_value = ctx.attr.paths
    else:
        paths_value = "{{{files}}}".format(files = ",".join([f.path for f in ctx.files.srcs]))

    extra_args = []
    if hasattr(ctx.attr, "extra_args") and ctx.attr.extra_args:
        extra_args = ctx.attr.extra_args

    cmd = """
          source <($PWD/{godir}/go env) &&
          export PATH=$GOROOT/bin:$PWD/{godir}:$PATH &&
          export GOPATH={gopath} &&
          mkdir -p .gocache &&
          export GOCACHE=$PWD/.gocache &&
          {cmd} {args}
        """.format(
        godir = go_ctx.go.path[:-1 - len(go_ctx.go.basename)],
        gopath = gopath,
        cmd = "$(pwd)/" + cg_info.controller_gen_bin.path,
        args = " ".join([
            "{cg_cmd}".format(cg_cmd = cg_cmd),
            "paths={paths}".format(paths = paths_value),
            "output:dir={outputpath}".format(outputpath = output_path),
        ] + extra_args),
    )
    ctx.actions.run_shell(
        mnemonic = "ControllerGen",
        outputs = outputs,
        inputs = _inputs(ctx, go_ctx),
        env = {"GO111MODULE": "off"},
        command = cmd,
        tools = [
            go_ctx.go,
            cg_info.controller_gen_bin,
        ],
    )

def _inputs(ctx, go_ctx):
    inputs = depset(
        direct = ctx.files.srcs,
        transitive = [
            go_ctx.sdk.srcs,
            go_ctx.sdk.tools,
            go_ctx.sdk.headers,
            go_ctx.stdlib.libs,
        ],
    )
    if ctx.attr.gopath_dep:
        inputs = depset(transitive = [inputs, ctx.attr.gopath_dep.files])
    if hasattr(ctx.attr, "headerFile") and ctx.file.headerFile:
        inputs = depset(direct = [ctx.file.headerFile], transitive = [inputs])
    return inputs

COMMON_ATTRS = {
    "srcs": attr.label_list(
        allow_empty = False,
        allow_files = True,
        mandatory = True,
    ),
    "paths": attr.string(default = ""),
    "gopath_dep": attr.label(
        providers = [GoPath],
        mandatory = False,
    ),
    "_go_context_data": attr.label(
        default = "@rules_go//:go_context_data",
    ),
}

def _crd_extra_attrs():
    ret = dict(COMMON_ATTRS)
    ret.update({
        "trivialVersions": attr.bool(default = False),
        "preserveUnknownFields": attr.bool(default = False),
        "crdVersions": attr.string_list(),
        "maxDescLen": attr.int(),
        "headerFile": attr.label(allow_single_file = True),
        "extra_args": attr.string_list(),
    })
    return ret

def _controller_gen_crd_impl(ctx):
    outputdir = ctx.actions.declare_directory(ctx.label.name)
    cg_cmd = "crd"
    extra_args = []
    if hasattr(ctx.attr, "headerFile") and ctx.file.headerFile:
        extra_args.append("headerFile={}".format(ctx.file.headerFile.path))
    if ctx.attr.trivialVersions:
        extra_args.append("trivialVersions=true")
    if ctx.attr.preserveUnknownFields:
        extra_args.append("preserveUnknownFields=true")
    if len(extra_args) > 0:
        cg_cmd += ":{args}".format(args = ",".join(extra_args))
    _controller_gen_action(ctx, cg_cmd, [outputdir], outputdir.path)
    return DefaultInfo(files = depset([outputdir]))

_controller_gen_crd = rule(
    implementation = _controller_gen_crd_impl,
    attrs = _crd_extra_attrs(),
    toolchains = [
        "@rules_go//go:toolchain",
        "//tools/controller-gen:toolchain",
    ],
)

def _maybe_add_gopath_dep(name, kwargs):
    if kwargs.get("deps", None):
        gopath_name = name + "_controller_gen"
        go_path(
            name = gopath_name,
            deps = kwargs["deps"],
        )
        kwargs["gopath_dep"] = gopath_name
        kwargs.pop("deps")

def controller_gen_crd(name, **kwargs):
    _maybe_add_gopath_dep(name, kwargs)
    _controller_gen_crd(name = name, **kwargs)
