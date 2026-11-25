import type { HistoryEntry, Target } from "./api";

/** Tone names shared by badges, tiles and history rows (DESIGN.md §3). */
export type Tone = "ok" | "info" | "neutral" | "warn" | "danger";

const OPERATION_LABELS: Record<string, string> = {
  doctor: "设备预检",
  verify: "root 验证",
  enable: "开启 SSH",
  "persist soft": "安装软固化",
  "persist remove": "移除软固化",
  "reboot-verify": "重启验证",
  "backup create": "创建备份",
  "backup verify": "本地备份校验",
  "backup verify online": "在线备份校验",
  "deep-persist": "深度固化",
};

export function operationLabel(operation: string): string {
  return OPERATION_LABELS[operation] ?? operation;
}

/** History is newest-first (guiapi prepends), so the first match is the latest run. */
export function latestEntry(history: HistoryEntry[], operation: string): HistoryEntry | undefined {
  return history.find((item) => item.operation === operation);
}

export interface DeviceSummary {
  model: string;
  hardware: string;
  firmware: string;
  profileId: string;
  capabilities: string[];
  compatible: boolean;
}

export function deviceSummary(report: any): DeviceSummary | null {
  const device = report?.device;
  if (!device || !(device.hardware || device.display_name || device.rom_version)) return null;
  return {
    model: device.display_name || device.hardware || "未知型号",
    hardware: device.hardware || "",
    firmware: device.rom_version || "",
    profileId: report.profile?.id || "",
    capabilities: Array.isArray(report.profile?.capabilities) ? report.profile.capabilities : [],
    compatible: Boolean(report.compatible),
  };
}

export interface PortTile {
  key: "web" | "ssh" | "telnet";
  title: string;
  meta: string;
  tone: Tone;
  value: string;
  detail: string;
}

function webTile(report: any, target: Target | undefined): PortTile {
  const base = {
    key: "web" as const,
    title: "Web 管理",
    meta: target ? `HTTP ${target.web_port}` : "",
  };
  if (report?.error?.code === "WEB_UNREACHABLE") {
    return { ...base, tone: "danger", value: "无法连接", detail: "检查地址、端口和网络后重试" };
  }
  if (report?.admin_auth === "verified") {
    return { ...base, tone: "ok", value: "已验证", detail: "Web 管理密码正确" };
  }
  if (report?.admin_auth === "failed") {
    // The backend marks a requested check failed whenever the preflight stops before it passes,
    // so only a rejected login says anything about the password.
    if (report?.error?.code === "WEB_AUTH_FAILED") {
      return { ...base, tone: "danger", value: "验证失败", detail: "管理密码不正确或登录被拒绝" };
    }
    const code = report?.error?.code ? `：${report.error.code}` : "";
    return { ...base, tone: "neutral", value: "未验证", detail: `预检在验证密码之前停止${code}` };
  }
  if (report?.device?.rom_version) {
    return { ...base, tone: "info", value: "可访问", detail: "已读取型号与固件，未验证管理密码" };
  }
  return { ...base, tone: "neutral", value: "未检测", detail: "运行预检后显示" };
}

function sshTile(report: any, target: Target | undefined): PortTile {
  const base = { key: "ssh" as const, title: "SSH", meta: target ? String(target.ssh_port) : "" };
  if (report?.ssh_verified) {
    return { ...base, tone: "ok", value: "root 已验证", detail: "密码认证成功且 id -u = 0" };
  }
  switch (report?.ssh) {
    case "open":
      return { ...base, tone: "info", value: "端口开放", detail: "未验证 root · 开放端口不算认证" };
    case "closed":
      return { ...base, tone: "neutral", value: "已关闭", detail: "可在「开启 SSH」中临时开启" };
    case "unreachable":
      return { ...base, tone: "neutral", value: "不可达", detail: "端口没有响应" };
    default:
      return { ...base, tone: "neutral", value: "未检测", detail: "预检没有走到端口探测" };
  }
}

function telnetTile(report: any, target: Target | undefined): PortTile {
  const base = {
    key: "telnet" as const,
    title: "Telnet",
    meta: target ? String(target.telnet_port) : "",
  };
  if (report?.telnet_root_verified) {
    return {
      ...base,
      tone: "warn",
      value: "root 已验证",
      detail: "明文协议，不能作为最终 SSH 证据",
    };
  }
  switch (report?.telnet) {
    case "open":
      return {
        ...base,
        tone: "warn",
        value: "端口开放",
        detail: "明文协议，只在明确允许的步骤中使用",
      };
    case "closed":
      return { ...base, tone: "neutral", value: "已关闭", detail: "只在明确允许的步骤中使用" };
    case "unreachable":
      return { ...base, tone: "neutral", value: "不可达", detail: "端口没有响应" };
    default:
      return { ...base, tone: "neutral", value: "未检测", detail: "预检没有走到端口探测" };
  }
}

/** `target` is the target the report was produced for; without it the tiles carry no port label. */
export function portTiles(report: any, target: Target | undefined): [PortTile, PortTile, PortTile] {
  return [webTile(report, target), sshTile(report, target), telnetTile(report, target)];
}

export interface NextStep {
  title: string;
  detail: string;
  to?: string;
}

const WRITE_OPERATIONS = new Set([
  "enable",
  "persist soft",
  "persist remove",
  "reboot-verify",
  "deep-persist",
]);

/**
 * Suggests the next step in the order of docs/compatibility.md: preflight, enable, backup. The
 * newest write in `history` (newest-first) wins when its result is partial or unknown, because
 * then nothing should be started from the beginning.
 */
export function nextStep(report: any, history: HistoryEntry[] = []): NextStep {
  const lastWrite = history.find((item) => WRITE_OPERATIONS.has(item.operation));
  const writeStatus = (lastWrite?.report as any)?.status;
  if (writeStatus === "partial" || writeStatus === "unknown") {
    return {
      title: "查看报告，不要从头重跑",
      detail: `最近一次${operationLabel(lastWrite!.operation)}${writeStatus === "partial" ? "部分完成" : "结果未知"}，先人工检查设备`,
      to: "/reports",
    };
  }
  if (!report) return { title: "运行预检", detail: "先识别型号与固件，不会修改设备" };
  if (report.status === "failed") {
    return {
      title: "排查后重试预检",
      detail: report.error?.message || "查看报告了解失败原因",
      to: "/reports",
    };
  }
  const rootVerified =
    report.ssh_verified ||
    history.some(
      (item) =>
        (item.operation === "verify" || item.operation === "enable") &&
        (item.report as any)?.status === "succeeded" &&
        (item.report as any)?.ssh_verified,
    );
  if (rootVerified) {
    return {
      title: "创建分区备份",
      detail: "任何固化操作之前，先备份并校验全部分区",
      to: "/backup",
    };
  }
  if (report.compatible) {
    return { title: "开启 SSH", detail: "需要 Web 管理密码和新的 root 密码", to: "/enable" };
  }
  return { title: "查看报告", detail: "设备未通过兼容性检查", to: "/reports" };
}

/** A failed run whose error is CANCELED is shown as canceled; partial and unknown keep their status. */
export function displayStatus(report: any): string {
  const status = report?.status || "unknown";
  return report?.error?.code === "CANCELED" && status === "failed" ? "canceled" : status;
}

export interface OperationRow {
  id: string;
  operation: string;
  title: string;
  meta: string;
  status: string;
  code?: string;
}

function pad(value: number): string {
  return String(value).padStart(2, "0");
}

/** Local time today (HH:MM:SS), otherwise the date (MM-DD HH:MM). */
export function formatTime(iso: string, now: Date = new Date()): string {
  const time = new Date(iso);
  if (Number.isNaN(time.getTime())) return iso;
  const sameDay =
    time.getFullYear() === now.getFullYear() &&
    time.getMonth() === now.getMonth() &&
    time.getDate() === now.getDate();
  const clock = `${pad(time.getHours())}:${pad(time.getMinutes())}`;
  return sameDay
    ? `${clock}:${pad(time.getSeconds())}`
    : `${pad(time.getMonth() + 1)}-${pad(time.getDate())} ${clock}`;
}

export function recentOperations(
  history: HistoryEntry[],
  limit = 5,
  now: Date = new Date(),
): OperationRow[] {
  return history.slice(0, limit).map((item) => {
    const report = (item.report ?? {}) as any;
    const code: string | undefined = report.error?.code || undefined;
    const meta = [formatTime(item.time, now), report.host, code].filter(Boolean).join(" · ");
    return {
      id: item.operation_id,
      operation: item.operation,
      title: operationLabel(item.operation),
      meta,
      status: displayStatus(report),
      code,
    };
  });
}

export interface StageRow {
  stage: string;
  status: string;
}

const TERMINAL_STAGES = new Set(["completed", "failed"]);

/**
 * Collapses progress events into one row per stage. The backend emits `running` when a stage
 * starts and one terminal event (`completed` or `failed`) at the end, so a stage is finished when
 * the next one starts, and the terminal status belongs to the stage that was still running.
 */
export function stageList(events: StageRow[]): StageRow[] {
  const rows: StageRow[] = [];
  for (const event of events) {
    const last = rows.at(-1);
    if (TERMINAL_STAGES.has(event.stage.toLowerCase())) {
      if (last?.status === "running") last.status = event.status;
      else if (!last || event.status !== "succeeded")
        rows.push({ stage: event.stage, status: event.status });
      continue;
    }
    if (last?.status === "running") last.status = "succeeded";
    rows.push({ stage: event.stage, status: event.status });
  }
  return rows;
}

export interface TargetStatus {
  tone: "ok" | "danger" | "neutral";
  label: string;
  detail: string;
}

/**
 * The sidebar's current-target summary, always in words (DESIGN.md §1). A report for another host
 * is not attributed to the edited target.
 */
export function targetStatus(report: any, currentHost: string): TargetStatus {
  if (!report) return { tone: "neutral", label: "尚未预检", detail: "" };
  if (report.host && report.host !== currentHost) {
    return {
      tone: "neutral",
      label: "目标已更改，请重新预检",
      detail: `上次预检的是 ${report.host}`,
    };
  }
  const summary = deviceSummary(report);
  const detail = summary ? [summary.model, summary.firmware].filter(Boolean).join(" · ") : "";
  if (report.status === "succeeded") return { tone: "ok", label: "上次预检成功", detail };
  return { tone: "danger", label: "上次预检失败", detail };
}
