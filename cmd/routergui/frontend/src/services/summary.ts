import type { HistoryEntry } from "./api";

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
  "guide apply": "引导执行",
  "recovery ack": "确认已检查",
};

export function operationLabel(operation: string): string {
  return OPERATION_LABELS[operation] ?? operation;
}

/** History is newest-first (guiapi prepends), so the first match is the latest run. */
export function latestEntry(history: HistoryEntry[], operation: string): HistoryEntry | undefined {
  return history.find((item) => item.operation === operation);
}

/** The newest entry of an operation the window started itself; agent runs are shown on their own page. */
export function latestOwnEntry(
  history: HistoryEntry[],
  operation: string,
): HistoryEntry | undefined {
  return history.find((item) => item.operation === operation && item.source !== "agent");
}

/** A failed run whose error is CANCELED is shown as canceled; partial and unknown keep their status. */
export function displayStatus(report: any): string {
  const status = report?.status || "unknown";
  return report?.error?.code === "CANCELED" && status === "failed" ? "canceled" : status;
}

/** The guided page's header status: a run's result, or failed when the run was rejected before it began. */
export function guideStatus(phase: string, run: any, error: string): string {
  if (phase === "detecting" || phase === "running") return "running";
  if (run) return displayStatus(run);
  if (phase === "done" && error) return "failed";
  return "idle";
}

export interface OperationRow {
  id: string;
  operation: string;
  title: string;
  meta: string;
  status: string;
  code?: string;
  /** An agent started the operation. */
  agent?: boolean;
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
      agent: item.source === "agent",
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
  if (!report) return { tone: "neutral", label: "尚未检测", detail: "" };
  if (report.host && report.host !== currentHost) {
    return {
      tone: "neutral",
      label: "目标已更改，请重新检测",
      detail: `上次检测的是 ${report.host}`,
    };
  }
  const device = report.device;
  const identified = Boolean(
    device && (device.hardware || device.display_name || device.rom_version),
  );
  const detail = identified
    ? [device.display_name || device.hardware || "未知型号", device.rom_version]
        .filter(Boolean)
        .join(" · ")
    : "";
  if (report.status === "succeeded") return { tone: "ok", label: "上次检测成功", detail };
  return { tone: "danger", label: "上次检测失败", detail };
}
