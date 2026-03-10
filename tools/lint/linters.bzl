load("@aspect_rules_lint//format:defs.bzl", "format_test")

def go_format_test(**kwargs):
    format_test(
        go = "@aspect_rules_lint//format:gofumpt",
        **kwargs
    )
