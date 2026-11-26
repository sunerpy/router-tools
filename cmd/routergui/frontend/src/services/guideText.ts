import type { DeepBlocked, GuideAction, GuideStepID } from "./api";
import type { Tone } from "./summary";

// Simplified Chinese wording for the codes the guided flow, the deep status and the vault report. The
// codes come from internal/app; an unknown code is shown as itself rather than hidden.

export const STEP_TITLE: Record<GuideStepID, string> = {
  enable_ssh: "开启 SSH",
  backup: "分区备份",
  soft_install: "软固化",
  reboot_verify: "重启验证",
};

export const ACTION: Record<GuideAction, { label: string; tone: Tone }> = {
  run: { label: "将执行", tone: "info" },
  skip: { label: "已满足", tone: "ok" },
  decide_after: { label: "SSH 可用后判断", tone: "neutral" },
  needs_input: { label: "需要输入", tone: "warn" },
  blocked: { label: "已阻止", tone: "danger" },
  unsupported: { label: "不支持", tone: "neutral" },
  not_run: { label: "未执行", tone: "neutral" },
};

export const REASON: Record<string, string> = {
  root_ssh_verified: "root 已能通过 SSH 登录，不需要再开启。",
  ssh_closed: "SSH 端口没有开放，将通过 Web 管理接口开启 SSH 并设置新的 root 密码。",
  root_password_missing: "SSH 端口已开放。输入当前 root 密码后重新检测。",
  ssh_auth_failed: "SSH 拒绝了这个 root 密码。输入当前密码后重新检测，或选择重置 root 密码。",
  reset_root_password: "将通过开启流程把 root 密码改为你输入的新密码。",
  backup_verified: "已有一份通过校验的托管备份。",
  backup_missing: "还没有通过校验的托管备份，将新建一份。",
  backup_root_missing: "没有配置托管备份目录。",
  after_ssh: "开启 SSH 后再检查是否需要安装。",
  soft_installed_verified: "软固化已安装并通过校验。",
  soft_not_installed: "尚未安装软固化。",
  soft_file_drift: "脚本内容与预期不一致，重新安装即可修复。",
  soft_mode_drift: "脚本不可执行。先在「维护」中移除软固化，再重新检测。",
  soft_uci_drift: "防火墙配置与预期不一致。先在「维护」中移除软固化，再重新检测。",
  soft_conflict: "发现不是本工具写入的防火墙配置。工具不会覆盖它，请人工检查。",
  soft_orphan_file: "脚本存在但缺少防火墙配置，请人工检查。",
  reboot_selected: "每次都会执行：重启后验证 SSH 与软固化。",
  capability_missing: "当前 profile 不支持这一步。",
  incompatible: "设备与所选 profile 不兼容。",
  host_key_rejected: "SSH 主机密钥被拒绝或已变化，已停止。",
  recovery_review_required: "上次变更的结果不确定，需要先人工检查并确认。",
  operation_locked: "另一个操作正在使用这台路由器。",
  detection_failed: "检测没有完成。",
};

export const EFFECT: Record<string, string> = {
  temporary_mac_rules: "通过 Web 接口添加临时 MAC 过滤规则，完成后删除",
  nvram_commit: "把 SSH 与 Telnet 开关写入 NVRAM，重启后仍然生效",
  dropbear_start: "把 SSH 服务切换到调试通道并启动",
  root_password_change: "把 root 密码改为你输入的新密码",
  local_backup: "通过 SSH 读取全部闪存分区，保存到新的本地备份目录",
  script_write: "在路由器上写入工具管理的开机脚本",
  uci_commit: "提交一条工具管理的防火墙 include，开机时运行该脚本",
  reboot: "重启路由器，网络会中断几分钟",
};

export const MAINTENANCE: Record<string, string> = {
  persist_remove: "在「维护」中移除软固化",
  manual_review: "人工检查 firewall.auto_ssh 配置",
};

export const DEEP_BLOCKED: Record<DeepBlocked, string> = {
  recovery_review_required: "上次变更的结果不确定，需要先人工检查并确认。",
  journal_exists: "托管目录里已有这台路由器的 journal：不要从头重跑，按 RECOVERY.md 检查。",
  flags_already_set: "启动标志已全部设置，深度固化可能已经完成。",
  flags_mixed: "启动标志只设置了一部分，需要人工检查。",
  crash_partition_unavailable: "crash 分区不存在或不可读。",
  no_verified_backup: "托管目录里没有通过校验的备份；可以选择其他已校验的目录。",
};

export const CREDENTIAL_KIND: Record<string, string> = {
  "web-admin": "Web 管理密码",
  root: "root 密码",
};

export const CREDENTIAL_NOTE: Record<string, string> = {
  used_saved: "已使用保存的密码",
  saved: "已保存",
  save_failed: "保存失败",
  rejected: "已保存的密码被路由器拒绝，已标记，不会再自动使用",
  unavailable: "无法读取保存的密码",
};

export const ERROR_TITLE: Record<string, string> = {
  UNSUPPORTED_FIRMWARE: "固件不受支持",
  UNSUPPORTED_DEVICE: "设备不受支持",
  UNSUPPORTED_PROFILE: "所选 profile 未注册",
  AMBIGUOUS_DEVICE: "无法确定唯一的设备 profile",
  OPERATION_LOCKED: "另一个操作正在使用这台路由器",
  HOST_KEY_MISMATCH: "SSH 主机密钥已变化",
  SSH_HOST_KEY_REJECTED: "SSH 主机密钥未被接受",
  SOFT_CONFIG_CONFLICT: "软固化需要先维护",
  WEB_UNREACHABLE: "无法连接路由器的 Web 管理端口",
  SSH_UNREACHABLE: "无法连接路由器的 SSH 端口",
  CONFIRMATION_REQUIRED: "需要重新确认",
  CREDENTIAL_STORE_FAILED: "无法读取保存的密码",
  CANCELED: "已取消",
};

export function errorTitle(code: string, fallback = "检测没有完成"): string {
  return ERROR_TITLE[code] ?? fallback;
}

export function reasonText(code: string): string {
  return REASON[code] ?? code;
}

export function effectText(code: string): string {
  return EFFECT[code] ?? code;
}
