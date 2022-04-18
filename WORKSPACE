load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "f2dcd210c7095febe54b804bb1cd3a58fe8435a909db2ec04e31542631cf715c",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.31.0/rules_go-v0.31.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.31.0/rules_go-v0.31.0.zip",
    ],
)

# gazelle is used for generating BUILD.bazel files.
http_archive(
    name = "bazel_gazelle",
    sha256 = "de69a09dc70417580aabf20a28619bb3ef60d038470c7cf8442fafcf627c21cb",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.24.0/bazel-gazelle-v0.24.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.24.0/bazel-gazelle-v0.24.0.tar.gz",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")

go_rules_dependencies()

go_register_toolchains(version = "1.18")

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()


# go_repository is needed for repos that do not use bazel.
load("@bazel_gazelle//:deps.bzl", "go_repository")

go_repository(
    name = "com_github_google_uuid",
    importpath = "github.com/google/uuid",
    commit = "0cd6bf5da1e1c83f8b45653022c74f71af0538a4",
)

go_repository(
    name = "com_github_golang_glog",
    importpath = "github.com/golang/glog",
    commit = "23def4e6c14b4da8ac2ed8007337bc5eb5007998",
)

go_repository(
    name = "in_gopkg_yaml_v2",
    importpath = "gopkg.in/yaml.v2",
    commit = "53403b58ad1b561927d19068c655246f2db79d48",
)

go_repository(
    name = "com_github_ghodss_yaml",
    importpath = "github.com/ghodss/yaml",
    commit = "0ca9ea5df5451ffdf184b4428c902747c2c11cd7",
)

go_repository(
    name = "com_github_hashicorp_errwrap",
    importpath = "github.com/hashicorp/errwrap",
    commit = "8a6fb523712970c966eefc6b39ed2c5e74880354",
)

go_repository(
    name = "com_github_hashicorp_go_multierror",
    importpath = "github.com/hashicorp/go-multierror",
    commit = "886a7fbe3eb1c874d46f623bfa70af45f425b3d1",
)

go_repository(
    name = "org_golang_google_api",
    importpath = "google.golang.org/api",
    commit = "c24765c18bb761c90df819dcdfdd62f9a7f6fa22",
)

go_repository(
    name = "org_golang_x_oauth2",
    importpath = "golang.org/x/oauth2",
    commit = "bf48bf16ab8d622ce64ec6ce98d2c98f916b6303",
)

go_repository(
    name = "com_google_cloud_go",
    importpath = "cloud.google.com/go",
    commit = "7cfb4662a9aa5a065063c202c577acb0b582498b",
)

go_repository(
    name = "io_opencensus_go",
    importpath = "go.opencensus.io",
    commit = "d835ff86be02193d324330acdb7d65546b05f814",
)

go_repository(
    name = "com_github_googleapis_gax_go",
    importpath = "github.com/googleapis/gax-go",
    commit = "bd5b16380fd03dc758d11cef74ba2e3bc8b0e8c2",
    #build_extra_args = ["-known_import=github.com/googleapis/gax-go/v2"],
)

go_repository(
    name = "org_golang_google_grpc",
    importpath = "google.golang.org/grpc",
    commit = "142182889d38b76209f1d9f1d8e91d7608aff542",
)

go_repository(
    name = "org_golang_x_sys",
    importpath = "golang.org/x/sys",
    commit = "5766fd39f98ddd8d769ad4aedcee557dd28de90f",
)

go_repository(
    name = "org_golang_x_net",
    importpath = "golang.org/x/net",
    commit = "244492dfa37ae2ce87222fd06250a03160745faa",
)

go_repository(
    name = "com_github_golang_groupcache",
    importpath = "github.com/golang/groupcache",
    commit = "8c9f03a8e57eb486e42badaed3fb287da51807ba",
)

go_repository(
    name = "org_golang_x_text",
    importpath = "golang.org/x/text",
    commit = "f21a4dfb5e38f5895301dc265a8def02365cc3d0",
)

load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")

git_repository(
    name = "com_google_protobuf",
    commit = "7db4eca77f2b03f93632edca5825f33ab65590e7",
    remote = "https://github.com/protocolbuffers/protobuf",
    shallow_since = "1649184317 -0700",
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

git_repository(
    name = "com_google_googletest",
    remote = "https://github.com/google/googletest",
    tag = "release-1.10.0",
)

git_repository(
    name = "com_google_absl",
    remote = "https://github.com/abseil/abseil-cpp",
    commit = "df3ea785d8c30a9503321a3d35ee7d35808f190d",
    shallow_since = "1583355457 -0500",
)

load("@bazel_tools//tools/build_defs/repo:git.bzl", "new_git_repository")

new_git_repository(
    name = "com_jsoncpp",
    remote = "https://github.com/open-source-parsers/jsoncpp",
    commit = "ba3fd412929ec4822788b401298e8d9e4950cbab",
    shallow_since = "1529685178 -0500",
    build_file_content = """
cc_library(
    name = "json",
    hdrs = glob([
        "include/json/*.h",
        "src/lib_json/json_tool.h",
    ]),
    textual_hdrs = [
        "src/lib_json/json_valueiterator.inl",
    ],
    srcs = [
        "src/lib_json/json_reader.cpp",
        "src/lib_json/json_value.cpp",
        "src/lib_json/json_writer.cpp",
    ],
    includes = ["include"],
    visibility = ["//visibility:public"],
    alwayslink = 1,
)
"""
)

