import {
  agentActivityRows,
  agentHeaderStatus,
  agentNavState,
  agentOperationRows,
  agentToolLabel,
  isLoopback,
  listenOptions,
} from "./agents";
import { recentOperations } from "./summary";
import type { AgentStatus } from "./api";

const now = new Date("2026-11-27T10:00:00");

function status(listening: boolean, writes: boolean): AgentStatus {
  return {
    listener: {
      listening,
      enabled: listening,
      listen_host: "127.0.0.1",
      port: 18731,
      allow_writes: writes,
      token_set: true,
    },
    url: "http://127.0.0.1:18731/mcp",
    local_addresses: [],
    operations: [],
    activity: [],
    agents: [],
  };
}

describe("agent summaries", () => {
  it("names the service state in words for the navigation row", () => {
    expect(agentNavState(null)).toBe("");
    expect(agentNavState(status(false, true))).toBe("");
    expect(agentNavState(status(true, false))).toBe("已开启");
    expect(agentNavState(status(true, true))).toBe("可写入");
  });

  it("reports running in the page header while an agent operation runs", () => {
    expect(agentHeaderStatus(null)).toBe("idle");
    expect(agentHeaderStatus({ ...status(true, true), operations: null as never })).toBe("idle");
    const busy = status(true, true);
    busy.operations = [
      {
        operation_id: "agent-2",
        tool: "backup_create",
        status: "succeeded",
        done: true,
        started: "2026-11-27T09:58:00",
      },
    ];
    expect(agentHeaderStatus(busy)).toBe("idle");
    busy.operations.unshift({
      operation_id: "agent-3",
      tool: "guide_apply",
      status: "running",
      done: false,
      started: "2026-11-27T09:59:00",
    });
    expect(agentHeaderStatus(busy)).toBe("running");
  });

  it("labels tools by the use case they call", () => {
    expect(agentToolLabel("guide_apply")).toBe("执行引导计划");
    expect(agentToolLabel("backup_create")).toBe("创建备份");
    expect(agentToolLabel("something_new")).toBe("something_new");
  });

  it("turns agent operations into rows: running until done, canceled failures shown as canceled", () => {
    const rows = agentOperationRows(
      [
        {
          operation_id: "agent-1",
          tool: "backup_create",
          host: "192.168.31.1",
          status: "running",
          done: false,
          started: "2026-11-27T09:59:00",
        },
        {
          operation_id: "agent-2",
          tool: "guide_apply",
          host: "192.168.31.1",
          status: "failed",
          done: true,
          started: "2026-11-27T09:58:00",
          error_code: "CANCELED",
        },
        {
          operation_id: "agent-3",
          tool: "guide_apply",
          status: "partial",
          done: true,
          started: "2026-11-27T09:57:00",
          error_code: "ROOT_VERIFY_FAILED",
        },
      ],
      now,
    );
    expect(rows.map((row) => [row.id, row.title, row.status, row.agent])).toEqual([
      ["agent-1", "创建备份", "running", true],
      ["agent-2", "执行引导计划", "canceled", true],
      ["agent-3", "执行引导计划", "partial", true],
    ]);
    expect(rows[0].meta).toBe("09:59:00 · 192.168.31.1");
    expect(rows[2].meta).toBe("09:57:00 · ROOT_VERIFY_FAILED");
  });

  it("lists the newest activity with the call or finish kind", () => {
    const rows = agentActivityRows(
      [
        {
          time: "2026-11-27T09:59:30",
          kind: "finish",
          tool: "backup_create",
          operation_id: "agent-1",
          host: "192.168.31.1",
          status: "succeeded",
          duration_ms: 61000,
          done: true,
        },
        {
          time: "2026-11-27T09:59:00",
          kind: "call",
          tool: "backup_create",
          operation_id: "agent-1",
          host: "192.168.31.1",
          status: "running",
          duration_ms: 20000,
          done: false,
        },
        {
          time: "2026-11-27T09:58:00",
          kind: "call",
          tool: "list_profiles",
          status: "succeeded",
          duration_ms: 3,
          done: true,
        },
      ],
      2,
      now,
    );
    expect(rows.map((row) => [row.title, row.status])).toEqual([
      ["创建备份 · 结束", "succeeded"],
      ["创建备份 · 调用", "running"],
    ]);
    expect(rows[0].meta).toBe("09:59:30 · 192.168.31.1 · 61 秒");
    expect(new Set(rows.map((row) => row.id)).size).toBe(2);
  });

  it("shows a canceled operation's finish and call as canceled, like the operation rows", () => {
    const canceled = {
      tool: "guide_apply",
      operation_id: "agent-2",
      status: "failed",
      error_code: "CANCELED",
      duration_ms: 5000,
      done: true,
    };
    const rows = agentActivityRows(
      [
        { ...canceled, time: "2026-11-27T09:59:30", kind: "finish" as const },
        { ...canceled, time: "2026-11-27T09:59:00", kind: "call" as const },
        {
          ...canceled,
          time: "2026-11-27T09:58:00",
          kind: "call" as const,
          error_code: "WEB_UNREACHABLE",
        },
      ],
      3,
      now,
    );
    expect(rows.map((row) => row.status)).toEqual(["canceled", "canceled", "failed"]);
  });

  it("offers loopback, every interface and this computer's addresses", () => {
    expect(
      listenOptions(["192.168.31.20", "fd00::20"], "127.0.0.1").map((option) => option.value),
    ).toEqual(["127.0.0.1", "0.0.0.0", "192.168.31.20", "fd00::20"]);
    // A stored address that is gone stays selectable, so the form never shows another value than the stored one.
    expect(listenOptions([], "10.0.0.9").map((option) => option.value)).toEqual([
      "127.0.0.1",
      "0.0.0.0",
      "10.0.0.9",
    ]);
    expect(isLoopback("127.0.0.1")).toBe(true);
    expect(isLoopback("::1")).toBe(true);
    expect(isLoopback("0.0.0.0")).toBe(false);
    expect(isLoopback("192.168.31.20")).toBe(false);
  });

  it("marks history entries an agent started", () => {
    const rows = recentOperations(
      [
        {
          operation_id: "agent-1",
          operation: "backup create",
          report: { status: "succeeded" },
          time: "2026-11-27T09:59:00",
          source: "agent",
        },
        {
          operation_id: "desktop-1",
          operation: "doctor",
          report: { status: "succeeded" },
          time: "2026-11-27T09:58:00",
        },
      ],
      5,
      now,
    );
    expect(rows.map((row) => row.agent)).toEqual([true, false]);
  });
});
