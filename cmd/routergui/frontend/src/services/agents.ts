import type { AgentActivity, AgentOperation, AgentStatus } from "./api";
import { formatTime, type OperationRow } from "./summary";

const TOOL_LABELS: Record<string, string> = {
  list_profiles: "列出支持的设备",
  preflight: "设备预检",
  guide_plan: "引导检测",
  guide_apply: "执行引导计划",
  backup_create: "创建备份",
  deep_status: "深度固化状态",
  soft_remove: "移除软固化",
  saved_passwords: "列出已保存的密码",
  operation_wait: "等待操作",
  operation_list: "列出操作",
  operation_cancel: "取消操作",
};

/** The use case an agent tool calls, in words. */
export function agentToolLabel(tool: string): string {
  return TOOL_LABELS[tool] ?? tool;
}

/** The navigation row's state word: 已开启 while listening, 可写入 with router writes on, else nothing. */
export function agentNavState(status: AgentStatus | null): string {
  if (!status?.listener.listening) return "";
  return status.listener.allow_writes ? "可写入" : "已开启";
}

/** The page header's status on 「Agent 接入」: running while an agent operation runs, else idle. */
export function agentHeaderStatus(status: AgentStatus | null): string {
  // Go sends null for an empty list.
  return (status?.operations ?? []).some((operation) => !operation.done) ? "running" : "idle";
}

/** A failed run whose error is CANCELED reads as canceled, as everywhere else in the window. */
function settledStatus(status: string, code?: string): string {
  return status === "failed" && code === "CANCELED" ? "canceled" : status;
}

function operationStatus(operation: AgentOperation): string {
  if (!operation.done) return "running";
  return settledStatus(operation.status, operation.error_code);
}

/** Agent operations as history rows, newest first as the server lists them. */
export function agentOperationRows(
  operations: AgentOperation[],
  now: Date = new Date(),
): OperationRow[] {
  return operations.map((operation) => ({
    id: operation.operation_id,
    operation: operation.tool,
    title: agentToolLabel(operation.tool),
    meta: [formatTime(operation.started, now), operation.host, operation.error_code]
      .filter(Boolean)
      .join(" · "),
    status: operationStatus(operation),
    code: operation.error_code || undefined,
    agent: true,
  }));
}

function duration(milliseconds: number): string {
  return milliseconds >= 1000 ? `${Math.round(milliseconds / 1000)} 秒` : "";
}

/** The newest audit records as rows: the tool, whether the call returned or the operation finished, and status. */
export function agentActivityRows(
  activity: AgentActivity[],
  limit: number,
  now: Date = new Date(),
): OperationRow[] {
  return activity.slice(0, limit).map((record, index) => ({
    id: `${record.time}-${record.kind}-${record.operation_id ?? record.tool}-${index}`,
    operation: record.tool,
    title: `${agentToolLabel(record.tool)} · ${record.kind === "finish" ? "结束" : "调用"}`,
    meta: [
      formatTime(record.time, now),
      record.host,
      record.error_code,
      duration(record.duration_ms),
    ]
      .filter(Boolean)
      .join(" · "),
    status:
      record.kind === "call" && !record.done
        ? "running"
        : settledStatus(record.status || "unknown", record.error_code),
    code: record.error_code || undefined,
    agent: true,
  }));
}

/** 127.0.0.1 and ::1: an address only this computer reaches. */
export function isLoopback(host: string): boolean {
  return host === "127.0.0.1" || host === "::1";
}

/** The listen address choices: loopback, every IPv4 interface, and this computer's addresses. */
export function listenOptions(
  local: string[],
  current: string,
): Array<{ value: string; label: string }> {
  const options = [
    { value: "127.0.0.1", label: "仅本机（127.0.0.1）" },
    { value: "0.0.0.0", label: "所有网卡（0.0.0.0）" },
    ...local.map((address) => ({ value: address, label: `本机地址 ${address}` })),
  ];
  if (current && !options.some((option) => option.value === current)) {
    options.push({ value: current, label: `已保存的地址 ${current}` });
  }
  return options;
}
