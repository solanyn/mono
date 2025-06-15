load("@bazel_gazelle//:deps.bzl", "go_repository")

def go_dependencies():
    go_repository(
        name = "com_github_davecgh_go_spew",
        importpath = "github.com/davecgh/go-spew",
        sum = "h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_dustin_go_humanize",
        importpath = "github.com/dustin/go-humanize",
        sum = "h1:GzkhY7T5VNhEkwH0PVJgjz+fX1rhBrR7pRT3mDkpeCY=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_go_chi_chi_v5",
        importpath = "github.com/go-chi/chi/v5",
        sum = "h1:KOIHODQj58PmL80G2Eak4WdvUzjSJSm0vG72crDCqb8=",
        version = "v5.2.1",
    )
    go_repository(
        name = "com_github_go_chi_cors",
        importpath = "github.com/go-chi/cors",
        sum = "h1:xEC8UT3Rlp2QuWNEr4Fs/c2EAGVKBwy/1vHx3bppil4=",
        version = "v1.2.1",
    )
    go_repository(
        name = "com_github_go_chi_httplog_v3",
        importpath = "github.com/go-chi/httplog/v3",
        sum = "h1:j+tMqfljg9HCN82Te9GWjx9H8gc7Cmugqfen2bubUvA=",
        version = "v3.1.0",
    )
    go_repository(
        name = "com_github_go_ini_ini",
        importpath = "github.com/go-ini/ini",
        sum = "h1:z6ZrTEZqSWOTyH2FlglNbNgARyHG8oLW9gMELqKr06A=",
        version = "v1.67.0",
    )
    go_repository(
        name = "com_github_goccy_go_json",
        importpath = "github.com/goccy/go-json",
        sum = "h1:Fq85nIqj+gXn/S5ahsiTlK3TmC85qgirsdTP/+DeaC4=",
        version = "v0.10.5",
    )
    go_repository(
        name = "com_github_golang_protobuf",
        importpath = "github.com/golang/protobuf",
        sum = "h1:LUVKkCeviFUMKqHa4tXIIij/lbhnMbP7Fn5wKdKkRh4=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_google_go_cmp",
        importpath = "github.com/google/go-cmp",
        sum = "h1:Khx7svrCpmxxtHBq5j2mp/xVjsi8hQMfNLvJFAlrGgU=",
        version = "v0.5.5",
    )
    go_repository(
        name = "com_github_google_uuid",
        importpath = "github.com/google/uuid",
        sum = "h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_klauspost_compress",
        importpath = "github.com/klauspost/compress",
        sum = "h1:c/Cqfb0r+Yi+JtIEq73FWXVkRonBlf0CRNYc8Zttxdo=",
        version = "v1.18.0",
    )
    go_repository(
        name = "com_github_klauspost_cpuid_v2",
        importpath = "github.com/klauspost/cpuid/v2",
        sum = "h1:tBs3QSyvjDyFTq3uoc/9xFpCuOsJQFNPiAhYdw2skhE=",
        version = "v2.2.10",
    )
    go_repository(
        name = "com_github_minio_crc64nvme",
        importpath = "github.com/minio/crc64nvme",
        sum = "h1:DHQPrYPdqK7jQG/Ls5CTBZWeex/2FMS3G5XGkycuFrY=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_minio_md5_simd",
        importpath = "github.com/minio/md5-simd",
        sum = "h1:Gdi1DZK69+ZVMoNHRXJyNcxrMA4dSxoYHZSQbirFg34=",
        version = "v1.1.2",
    )
    go_repository(
        name = "com_github_minio_minio_go_v7",
        importpath = "github.com/minio/minio-go/v7",
        sum = "h1:lAB4QJp8Nq3vDMOU0eKgMuyBiEGMNlXQ5Glc8qAxqSU=",
        version = "v7.0.93",
    )
    go_repository(
        name = "com_github_philhofer_fwd",
        importpath = "github.com/philhofer/fwd",
        sum = "h1:dAMKvw0MlJT1GshSTtih8C2gDs04w8dReiOGXrGLNoY=",
        version = "v1.1.3-0.20240916144458-20a13a1f6b7c",
    )
    go_repository(
        name = "com_github_pmezard_go_difflib",
        importpath = "github.com/pmezard/go-difflib",
        sum = "h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_rs_xid",
        importpath = "github.com/rs/xid",
        sum = "h1:fV591PaemRlL6JfRxGDEPl69wICngIQ3shQtzfy2gxU=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_stretchr_testify",
        importpath = "github.com/stretchr/testify",
        sum = "h1:HtqpIVDClZ4nwg75+f6Lvsy/wHu+3BoSGCbBAcpTsTg=",
        version = "v1.9.0",
    )
    go_repository(
        name = "com_github_tinylib_msgp",
        importpath = "github.com/tinylib/msgp",
        sum = "h1:ULuf7GPooDaIlbyvgAxBV/FI7ynli6LZ1/nVUNu+0ww=",
        version = "v1.3.0",
    )
    go_repository(
        name = "in_gopkg_check_v1",
        importpath = "gopkg.in/check.v1",
        sum = "h1:yhCVgyC4o1eVCa2tZl7eS0r+SDo693bJlVdllGtEeKM=",
        version = "v0.0.0-20161208181325-20d25e280405",
    )
    go_repository(
        name = "in_gopkg_yaml_v3",
        importpath = "gopkg.in/yaml.v3",
        sum = "h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=",
        version = "v3.0.1",
    )
    go_repository(
        name = "org_golang_google_protobuf",
        importpath = "google.golang.org/protobuf",
        sum = "h1:z1NpPI8ku2WgiWnf+t9wTPsn6eP1L7ksHUlkfLvd9xY=",
        version = "v1.36.6",
    )
    go_repository(
        name = "org_golang_x_crypto",
        importpath = "golang.org/x/crypto",
        sum = "h1:AnAEvhDddvBdpY+uR+MyHmuZzzNqXSe/GvuDeob5L34=",
        version = "v0.36.0",
    )
    go_repository(
        name = "org_golang_x_mod",
        importpath = "golang.org/x/mod",
        sum = "h1:5+9lSbEzPSdWkH32vYPBwEpX8KwDbM52Ud9xBUvNlb0=",
        version = "v0.18.0",
    )
    go_repository(
        name = "org_golang_x_net",
        importpath = "golang.org/x/net",
        sum = "h1:vRMAPTMaeGqVhG5QyLJHqNDwecKTomGeqbnfZyKlBI8=",
        version = "v0.38.0",
    )
    go_repository(
        name = "org_golang_x_sync",
        importpath = "golang.org/x/sync",
        sum = "h1:MHc5BpPuC30uJk597Ri8TV3CNZcTLu6B6z4lJy+g6Jw=",
        version = "v0.12.0",
    )
    go_repository(
        name = "org_golang_x_sys",
        importpath = "golang.org/x/sys",
        sum = "h1:ioabZlmFYtWhL+TRYpcnNlLwhyxaM9kWTDEmfnprqik=",
        version = "v0.31.0",
    )
    go_repository(
        name = "org_golang_x_term",
        importpath = "golang.org/x/term",
        sum = "h1:PQ39fJZ+mfadBm0y5WlL4vlM7Sx1Hgf13sMIY2+QS9Y=",
        version = "v0.30.0",
    )
    go_repository(
        name = "org_golang_x_text",
        importpath = "golang.org/x/text",
        sum = "h1:D71I7dUrlY+VX0gQShAThNGHFxZ13dGLBHQLVl1mJlY=",
        version = "v0.23.0",
    )
    go_repository(
        name = "org_golang_x_tools",
        importpath = "golang.org/x/tools",
        sum = "h1:gqSGLZqv+AI9lIQzniJ0nZDRG5GBPsSi+DRNHWNz6yA=",
        version = "v0.22.0",
    )
    go_repository(
        name = "org_golang_x_xerrors",
        importpath = "golang.org/x/xerrors",
        sum = "h1:E7g+9GITq07hpfrRu66IVDexMakfv52eLZ2CXBWiKr4=",
        version = "v0.0.0-20191204190536-9bdfabe68543",
    )
