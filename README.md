<div align="center">

# router-tools

### 按精确型号和固件管理路由器：先只读检测，确认一次后只执行缺少的步骤，每次写入前都有安全闸门

[![CI](https://github.com/sunerpy/router-tools/actions/workflows/ci.yml/badge.svg)](https://github.com/sunerpy/router-tools/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/sunerpy/router-tools)](https://github.com/sunerpy/router-tools/releases)
[![License](https://img.shields.io/badge/license-MIT-green)](./LICENSE)

[安装](#安装) · [快速开始](#快速开始) · [用法](#用法) · [配合 AI Agent 使用](#配合-ai-agent-使用) · [安全边界](#安全边界) · [开发](#开发)

[**简体中文**](./README.md) · [English](./docs/readme/README.en.md)

</div>

---

`routerctl` 是一个 Go 命令行工具，同一套核心也驱动 Wails 桌面端（Router Tools）。首个适配是**小米 BE6500 Pro（RD08）固件 1.1.96**。

> **验证状态：** 协议、状态机、回环 SSH/Telnet、文件系统和 CLI 测试都离线运行，本仓库的测试从未连接真实路由器，所有功能都还没有做真机验收（见 [docs/compatibility.md](docs/compatibility.md)）。尤其是深度固化：已经实现，但**从未在真实设备上运行**。

## 特性

- **只认精确的型号和固件**：写入只对注册过的型号 + 固件开放；未知设备、未知固件一律拒绝；未知的 SSH 主机密钥要你确认后才接受，变化的主机密钥一律拒绝。`routerctl profiles` 列出支持的设备。
- **引导流程**：`routerctl setup` 和桌面端「向导」先只读检测，只规划缺少的步骤（开启 SSH、备份全部分区、软固化、可选的重启验证），一次确认列出全部写入，执行时遇到第一个未成功的步骤就停。
- **写入前先留记录**：每台路由器一把操作锁；第一次写入前先记录这次变更。结果为部分完成、未知、写入可能已提交后失败或被中断时，后续改动一律拒绝，直到你检查路由器并运行 `routerctl recovery ack`。
- **备份与校验**：流式备份 MTD 分区，并在本地校验完整性；`backup verify --online` 还会比对设备身份和完整分区布局，并确认路由器在备份后没有重启（`boot_id` 未变）。
- **桌面端**：向导、维护、深度固化（带只读状态检查）、Agent 接入、运行报告和设置，与 CLI 共用同一套用例；浅色（白瓷）和深色（石墨）两套主题。
- **AI Agent 接入**：`routerctl mcp` 提供 MCP 服务（stdio，或带令牌的 Streamable HTTP），桌面端「Agent 接入」页可直接开启；写入路由器需要单独开启，深度固化和恢复确认始终只能由人操作。
- **可选的密码保存**：AES-256-GCM 加密保存，只在路由器接受密码后保存，从不经 Telnet 发送。

## 安装

### 安装脚本

安装脚本会校验下载文件的 SHA-256；`TOOL_VERSION` 指定版本，`TOOL_INSTALL_DIR` 指定安装目录（默认 `~/.local/bin`）：

```bash
curl -fsSL https://raw.githubusercontent.com/sunerpy/router-tools/main/scripts/install.sh | sh
```

```powershell
irm https://raw.githubusercontent.com/sunerpy/router-tools/main/scripts/install.ps1 | iex
```

### 预编译文件

从 [GitHub Releases](https://github.com/sunerpy/router-tools/releases) 下载：

| 文件                                                | 内容                                                                                                   |
| --------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| `routerctl_<version>_<os>_<arch>.tar.gz` / `.zip`   | Linux、macOS、Windows 的 amd64 和 arm64 版 CLI，附 `LICENSE`、`README.md`、`CHANGELOG.md` 和第三方声明 |
| `router-tools_<version>_windows_amd64_setup.exe`    | Windows 桌面端安装程序（NSIS；缺少 WebView2 运行时会自动安装）                                         |
| `router-tools_<version>_windows_amd64_portable.zip` | Windows 桌面端免安装版（`Router Tools.exe`，需要 WebView2 运行时）                                     |
| `router-tools_<version>_darwin_universal.zip`       | macOS 桌面端，Apple 芯片和 Intel 通用（`Router Tools.app`）                                            |
| `checksums.txt`                                     | 以上所有文件的 SHA-256                                                                                 |

校验下载的文件和构建来源证明：

```bash
sha256sum --check --ignore-missing checksums.txt
gh attestation verify router-tools_<version>_darwin_universal.zip --repo sunerpy/router-tools
```

这些构建没有代码签名，也没有公证。Windows SmartScreen 首次启动可能拦截；macOS 上解压后把 `Router Tools.app` 移到 `/Applications` 并去掉隔离属性，否则 Gatekeeper 会阻止运行：

```bash
xattr -dr com.apple.quarantine "/Applications/Router Tools.app"
```

### go install

需要 Go 1.27 或更新版本。这样安装的 `routerctl version` 显示为 `dev`：

```bash
go install github.com/sunerpy/router-tools/cmd/routerctl@latest
```

### 从源码构建

见[开发](#开发)。

## 快速开始

```bash
routerctl profiles                              # 支持的型号和固件
routerctl doctor --host 192.168.31.1            # 只读预检：识别型号和固件
routerctl setup --host 192.168.31.1 --plan-only # 只读检测并显示计划，不做任何改动
routerctl setup --host 192.168.31.1             # 只询问计划需要的密码，确认一次后执行
```

SSH 未开启时，`setup` 询问 Web 管理密码和新的 root 密码（输入两次）；SSH 已开启时询问当前 root 密码。确认界面列出每一项写入及其影响。

## 用法

```text
routerctl
├── version · profiles
├── doctor [--with-admin-auth] [--with-root-verify]
├── setup [--goal ssh,backup,soft[,reboot]] [--reset-root-password] [--plan-only]
├── verify [--transport ssh|telnet|auto]
├── enable · reboot-verify
├── persist soft | remove
├── backup create [--output <新目录>] | verify --dir <目录> [--online]
├── deep-persist --backup-dir <已校验的目录>
├── recovery ack
├── credentials list | forget
├── mcp [--http <ip:port>] [--allow-writes] [--token-file <文件>] · mcp token · mcp config --agent <id>
└── skill show | install --agent <id> | --dir <目录> [--force]
```

`routerctl <命令> --help` 查看参数。密码输入、配置文件、受管目录和退出码见 [docs/usage.md](docs/usage.md)。

桌面端的页面：「向导」检测并执行引导流程；「维护」移除软固化、校验备份、验证 root 登录、运行原始预检；「深度固化」是带只读状态检查的高风险流程；「Agent 接入」开启 MCP 服务；「运行报告」列出本次会话的操作和报告；「设置」管理默认连接和设备、已保存的密码、主机密钥和主题。运行要求和打包说明见 [docs/gui.md](docs/gui.md)。

## 配合 AI Agent 使用

先按[安装](#安装)装好 `routerctl`，再让 Agent 通过 MCP 调用它：

<details>
<summary>Agent 接入速查</summary>

- `routerctl mcp`：stdio MCP 服务，由 Agent 宿主自己启动；`--allow-writes` 才提供会写路由器的 `guide_apply` 和 `soft_remove`。
- `routerctl mcp --http 127.0.0.1:18731`：Streamable HTTP，每个请求都要带 `Authorization: Bearer <令牌>`（令牌来自 `ROUTERCTL_MCP_TOKEN` 或 `--token-file`，用 `routerctl mcp token` 生成）。桌面端「Agent 接入」页提供同样的服务。
- `routerctl mcp config --agent <id>`：打印 Claude Code、Codex CLI、opencode、pi、DeepSeek Harness（dsh）、WorkBuddy 或其他 MCP 客户端的配置片段。
- `routerctl skill install --agent <id>`：安装内置的 skill，教 Agent 按流程操作。
- 流程：`guide_plan` 只读检测并给出计划；征得用户同意后，`guide_apply` 只执行一次；用 `operation_wait` 跟进；结果为 `partial` 或 `unknown` 时停下，等用户检查。
- 约定：stdout 只有 MCP 消息；警告和 JSON 审计记录写到 stderr，每次工具调用和每个结束的操作各一行；结果里从不出现密码。深度固化、恢复确认、忘记密码和 Telnet 从不提供给 Agent。

各宿主的配置、审批和超时见 [docs/agents.md](docs/agents.md)，风险见 [RISKS.md](RISKS.md#agent-access-mcp)。

</details>

## 安全边界

- CLI 的密码从不作为命令行参数或环境变量传入，只用隐藏输入或从标准输入读取指定行。保存密码需要明确选择，威胁模型见 [RISKS.md](RISKS.md#saved-passwords)。
- 通过 MCP 时，Agent 可以在工具参数里传密码，这些密码会留在 Agent 宿主的对话记录里并发给它的模型提供方；优先让 Agent 使用已保存的密码（`use_saved_password`），见 [RISKS.md](RISKS.md#agent-access-mcp)。
- 端口开着不算认证成功：SSH 成功必须是密码认证且 `id -u == 0`。
- 未知的 SSH 主机密钥默认拒绝，只有用 `--host-key-sha256` 固定指纹、加 `--accept-new-host-key` 或在桌面端确认后才接受；变化的主机密钥一律拒绝，密钥不匹配时从不改用 Telnet。
- `--yes` 只确认普通改动，对深度固化无效。深度固化需要 `--allow-telnet`、当前启动的已校验备份，以及 profile 规定的确认短语（RD08 1.1.96 为 `RD08-DEEP`）；深度写入超时记为 `unknown`，不会重试；已有的深度固化 journal 会阻止再次运行。
- 没有自动恢复、自动救砖、通用远程 shell 或脚本化的 profile。

任何会改动设备的操作之前，先读 [RISKS.md](RISKS.md) 和 [RECOVERY.md](RECOVERY.md)。

## 开发

需要 Go 1.27.1（`make test` 还需要 CGO 和 C 编译器）、Node.js 和 pnpm。`make fmt-check` 另需 goimports、gofumpt 和 oxfmt，`make lint` 另需 golangci-lint v2，缺少时这两个目标会打印安装命令。桌面端另需 Wails CLI 2.17.1，Linux 上还需 GTK3 和 WebKit2GTK 4.1 开发包。

`cmd/routergui` 内嵌了前端构建产物，新检出的仓库要先构建一次前端，否则 `make lint` 和 `make test` 会报 `pattern all:frontend/dist: no matching files found`：

```bash
(cd cmd/routergui/frontend && pnpm install && pnpm build)
make fmt          # goimports、gofumpt、oxfmt
make fmt-check
make lint
make test         # race 检测和覆盖率
make build-all    # Windows、macOS、Linux 的 amd64 和 arm64
make gui-test     # Vitest、vue-tsc、Vite 构建和 Go 绑定测试
make gui-build    # Linux 桌面端
```

测试从不连接真实路由器：HTTP 用 `httptest`，SSH 和 Telnet 用回环服务器，协议状态机用内存中的假 shell；CI 从不执行 MTD 写入。

- 架构：[docs/architecture.md](docs/architecture.md)；兼容性记录：[docs/compatibility.md](docs/compatibility.md)
- RD08 协议与安全规范（实现必须遵守）：[docs/rd08-protocol-spec.md](docs/rd08-protocol-spec.md)
- 设计系统：[DESIGN.md](DESIGN.md)；TDD 记录：[docs/tdd-log.md](docs/tdd-log.md)
- 贡献规则：[AGENTS.md](AGENTS.md)

## 许可证

MIT，见 [LICENSE](LICENSE) 和 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
