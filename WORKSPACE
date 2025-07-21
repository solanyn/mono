# WORKSPACE setup for dbx_build_tools hermetic Python builds
# Main dependencies managed via MODULE.bazel using Bzlmod

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# dbx_build_tools setup
load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")

git_repository(
    name = "dbx_build_tools",
    remote = "https://github.com/dropbox/dbx_build_tools.git",
    tag = "build-tools-py-2.0.0",  # Use stable tag instead of commit
)

# External dependencies required by dbx_build_tools
http_archive(
    name = "net_zlib",
    build_file = "@dbx_build_tools//thirdparty/zlib:BUILD.zlib",
    sha256 = "17e88863f3600672ab49182f217281b6fc4d3c762bde361935e436a95214d05c",
    strip_prefix = "zlib-1.3.1",
    urls = ["https://github.com/madler/zlib/archive/v1.3.1.tar.gz"],
)

http_archive(
    name = "org_bzip_bzip2",
    build_file = "@dbx_build_tools//thirdparty/bzip2:BUILD.bzip2", 
    sha256 = "ab5a03176ee106d3f0fa90e381da478ddae405918153cca248e682cd0c4a2269",
    strip_prefix = "bzip2-1.0.8",
    urls = ["https://sourceware.org/pub/bzip2/bzip2-1.0.8.tar.gz"],
)

http_archive(
    name = "org_openssl",
    build_file = "@dbx_build_tools//thirdparty/openssl:BUILD.openssl",
    sha256 = "d7939ce614029cdff0b6c20f0e2e5703158a489a72b2507b8bd51bf8c8fd10ca",
    strip_prefix = "openssl-1.1.1q",
    urls = ["https://www.openssl.org/source/openssl-1.1.1q.tar.gz"],
)

http_archive(
    name = "org_sqlite",
    build_file = "@dbx_build_tools//thirdparty/sqlite:BUILD.sqlite",
    sha256 = "6fb55507d4517b5cbc80bd2db57b0cbe1b45880b28f2e4bd6dca4cfe3716a231",
    strip_prefix = "sqlite-amalgamation-3380100",
    urls = ["https://www.sqlite.org/2022/sqlite-amalgamation-3380100.zip"],
)

http_archive(
    name = "org_gnu_ncurses",
    build_file = "@dbx_build_tools//thirdparty/ncurses:BUILD.ncurses",
    sha256 = "30306e0c76e0f9f1f0de987cf1c82a5c21e1ce6568b9227f7da5b71cbea86c9d",
    strip_prefix = "ncurses-6.2",
    urls = ["https://ftp.gnu.org/gnu/ncurses/ncurses-6.2.tar.gz"],
)

http_archive(
    name = "org_gnu_readline",
    build_file = "@dbx_build_tools//thirdparty/readline:BUILD.readline",
    sha256 = "f8ceb4ee131e3232226a17f51b164afc46cd0b9e6cef344be87c65962cb82b02",
    strip_prefix = "readline-8.1",
    urls = ["https://ftp.gnu.org/gnu/readline/readline-8.1.tar.gz"],
)

http_archive(
    name = "org_sourceware_libffi",
    build_file = "@dbx_build_tools//thirdparty/libffi:BUILD.libffi",
    sha256 = "72fba7922703ddfa7a028d513ac15a85c8d54c8d67f55fa5a4802885dc652056",
    strip_prefix = "libffi-3.3",
    urls = ["https://github.com/libffi/libffi/releases/download/v3.3/libffi-3.3.tar.gz"],
)

http_archive(
    name = "org_tukaani",
    build_file = "@dbx_build_tools//thirdparty/xz:BUILD.xz",
    sha256 = "3e1e518ffc912f86608a8cb35e4bd41ad1aec210df2a47aaa1f95e7f5576ef56",
    strip_prefix = "xz-5.2.5",
    urls = ["https://downloads.sourceforge.net/project/lzmautils/xz-5.2.5.tar.xz"],
)

# Python 3.9 repository  
http_archive(
    name = "org_python_cpython_39",
    urls = ["https://www.python.org/ftp/python/3.9.14/Python-3.9.14.tar.xz"],
    sha256 = "651304d216c8203fe0adf1a80af472d8e92c3b0e0a7892222ae4d9f3ae4debcf",
    strip_prefix = "Python-3.9.14",
    build_file = "@dbx_build_tools//thirdparty/cpython:BUILD.python39",
)

# Register dbx_build_tools Python 3.9 toolchain
register_toolchains(
    "@dbx_build_tools//thirdparty/cpython:drte-off-39-toolchain",
)
