# Third-party notices

The project uses the following direct Go modules and frontend packages. Transitive modules and exact checksums are recorded in `go.mod` and `go.sum`; `go list -m all` produces the build-time module inventory.

| Module                                                          | Purpose                                     | Upstream license reference                |
| --------------------------------------------------------------- | ------------------------------------------- | ----------------------------------------- |
| `github.com/spf13/cobra`                                        | CLI command tree                            | Apache-2.0                                |
| `github.com/spf13/pflag`                                        | POSIX/GNU-style flags (Cobra dependency)    | BSD-3-Clause                              |
| `github.com/inconshreveable/mousetrap`                          | Windows command behavior (Cobra dependency) | Apache-2.0                                |
| `golang.org/x/crypto`                                           | SSH and known_hosts                         | BSD-3-Clause                              |
| `golang.org/x/sys`                                              | Platform system interfaces                  | BSD-3-Clause                              |
| `golang.org/x/term`                                             | Hidden TTY password input                   | BSD-3-Clause                              |
| `github.com/gofrs/flock`                                        | Non-blocking cross-process operation locks  | BSD-3-Clause                              |
| `github.com/wailsapp/wails/v2`                                  | Desktop shell and bindings                  | MIT                                       |
| `github.com/modelcontextprotocol/go-sdk`                        | MCP server (stdio and Streamable HTTP)      | Apache-2.0 (MIT for legacy contributions) |
| `github.com/google/jsonschema-go`                               | MCP tool input schemas                      | MIT                                       |
| `github.com/segmentio/encoding`, `github.com/segmentio/asm`     | JSON encoding (SDK dependency)              | MIT                                       |
| `github.com/yosida95/uritemplate/v3`                            | URI templates (SDK dependency)              | BSD-3-Clause                              |
| `golang.org/x/oauth2`, `golang.org/x/sync`, `golang.org/x/time` | SDK dependencies                            | BSD-3-Clause                              |
| `vue` and `vue-router`                                          | Desktop reactivity and navigation           | MIT                                       |
| `naive-ui`                                                      | Desktop primitives and focus management     | MIT                                       |
| `lucide-vue-next`                                               | Desktop icon family                         | ISC                                       |
| `@fontsource-variable/instrument-sans`                          | Bundled UI typeface (Instrument Sans)       | OFL-1.1                                   |
| `@fontsource-variable/jetbrains-mono`                           | Bundled monospace typeface (JetBrains Mono) | OFL-1.1                                   |
| `@fontsource-variable/noto-sans-sc`                             | Bundled CJK typeface (Noto Sans SC)         | OFL-1.1                                   |

Before release, retain the corresponding upstream license texts in the distribution or provide them through the project's license bundle mechanism. Run a dependency vulnerability check and review newly introduced transitive dependencies whenever `go.mod` changes.

No original reference-repository source files, Python runtime, .NET runtime, third-party patcher, or router-side Go program is used.
