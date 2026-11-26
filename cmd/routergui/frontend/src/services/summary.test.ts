import {
  formatTime,
  guideStatus,
  latestEntry,
  latestOwnEntry,
  operationLabel,
  recentOperations,
  stageList,
  targetStatus,
} from "./summary";
import type { HistoryEntry } from "./api";

const doctorOK = {
  operation: "doctor",
  status: "succeeded",
  host: "192.168.31.1",
  device: {
    device_id: "D1",
    hardware: "RD08",
    display_name: "Xiaomi BE6500 Pro",
    rom_version: "1.1.96",
  },
  profile: { id: "xiaomi-rd08-1.1.96", capabilities: ["doctor", "web_auth", "verify"] },
  compatible: true,
  ssh: "open",
  telnet: "closed",
  admin_auth: "verified",
  root_verify: "skipped",
  ssh_verified: false,
  telnet_root_verified: false,
};

function entry(operation: string, report: unknown, time: string, id = operation): HistoryEntry {
  return { operation_id: id, operation, report, time };
}

describe("operationLabel", () => {
  it.each([
    ["doctor", "设备预检"],
    ["verify", "root 验证"],
    ["enable", "开启 SSH"],
    ["persist soft", "安装软固化"],
    ["persist remove", "移除软固化"],
    ["reboot-verify", "重启验证"],
    ["backup create", "创建备份"],
    ["backup verify", "本地备份校验"],
    ["backup verify online", "在线备份校验"],
    ["deep-persist", "深度固化"],
    ["something-new", "something-new"],
  ])("labels %s as %s", (operation, label) => {
    expect(operationLabel(operation)).toBe(label);
  });
});

describe("latestEntry", () => {
  it("returns the newest entry of an operation from newest-first history", () => {
    const history = [
      entry("enable", { status: "partial" }, "2026-11-25T06:00:00Z"),
      entry("doctor", { status: "failed" }, "2026-11-25T05:00:00Z", "d2"),
      entry("doctor", doctorOK, "2026-11-25T04:00:00Z", "d1"),
    ];
    expect(latestEntry(history, "doctor")?.operation_id).toBe("d2");
    expect(latestEntry(history, "deep-persist")).toBeUndefined();
  });

  it("skips runs an agent started when the window summarizes its own detection", () => {
    const history = [
      {
        ...entry(
          "doctor",
          { status: "failed", host: "192.168.31.2" },
          "2026-11-25T06:00:00Z",
          "a1",
        ),
        source: "agent",
      },
      entry("doctor", doctorOK, "2026-11-25T05:00:00Z", "d1"),
    ];
    expect(latestEntry(history, "doctor")?.operation_id).toBe("a1");
    expect(latestOwnEntry(history, "doctor")?.operation_id).toBe("d1");
    expect(latestOwnEntry(history.slice(0, 1), "doctor")).toBeUndefined();
  });
});

describe("recentOperations", () => {
  it("maps newest-first history to labelled rows with status and error code", () => {
    const now = new Date("2026-11-25T12:00:00");
    const rows = recentOperations(
      [
        entry(
          "enable",
          { status: "partial", host: "192.168.31.1", error: { code: "MAC_CLEANUP_FAILED" } },
          new Date("2026-11-25T10:15:30").toISOString(),
          "e1",
        ),
        entry(
          "doctor",
          { status: "succeeded", host: "192.168.31.1" },
          new Date("2026-11-24T20:55:00").toISOString(),
          "d1",
        ),
        entry("doctor", {}, new Date("2026-11-24T20:00:00").toISOString(), "d0"),
      ],
      2,
      now,
    );
    expect(rows).toHaveLength(2);
    expect(rows[0]).toMatchObject({
      id: "e1",
      operation: "enable",
      title: "开启 SSH",
      status: "partial",
      code: "MAC_CLEANUP_FAILED",
    });
    expect(rows[0].meta).toBe("10:15:30 · 192.168.31.1 · MAC_CLEANUP_FAILED");
    expect(rows[1].meta).toBe("11-24 20:55 · 192.168.31.1");
  });

  it("treats a report without a status as unknown", () => {
    const [row] = recentOperations([entry("doctor", {}, new Date().toISOString())]);
    expect(row.status).toBe("unknown");
  });
});

describe("formatTime", () => {
  it("shows the time today and the date otherwise", () => {
    const now = new Date("2026-11-25T12:00:00");
    expect(formatTime(new Date("2026-11-25T09:05:07").toISOString(), now)).toBe("09:05:07");
    expect(formatTime(new Date("2026-11-03T21:40:00").toISOString(), now)).toBe("11-03 21:40");
    expect(formatTime("not a date", now)).toBe("not a date");
  });
});

describe("review follow-ups", () => {
  it("shows a failed run with the CANCELED code as canceled", () => {
    const [row] = recentOperations([
      entry("doctor", { status: "failed", error: { code: "CANCELED" } }, new Date().toISOString()),
    ]);
    expect(row.status).toBe("canceled");
    const [written] = recentOperations([
      entry("enable", { status: "partial", error: { code: "CANCELED" } }, new Date().toISOString()),
    ]);
    expect(written.status).toBe("partial");
  });
});

describe("stageList", () => {
  it("marks finished stages and folds the terminal event into the last stage", () => {
    const events = [
      { stage: "validate", status: "running" },
      { stage: "probe", status: "running" },
      { stage: "failed", status: "failed" },
    ];
    expect(stageList(events)).toEqual([
      { stage: "validate", status: "succeeded" },
      { stage: "probe", status: "failed" },
    ]);
  });

  it("keeps the current stage running while the operation runs", () => {
    expect(
      stageList([
        { stage: "validate", status: "running" },
        { stage: "probe", status: "running" },
      ]),
    ).toEqual([
      { stage: "validate", status: "succeeded" },
      { stage: "probe", status: "running" },
    ]);
  });

  it("completes every stage on success and keeps explicit statuses", () => {
    expect(
      stageList([
        { stage: "PRECHECK", status: "running" },
        { stage: "COMPLETED", status: "succeeded" },
      ]),
    ).toEqual([{ stage: "PRECHECK", status: "succeeded" }]);
    expect(
      stageList([
        { stage: "PRECHECK", status: "succeeded" },
        { stage: "CRASH_WRITE_STARTED", status: "unknown" },
      ]),
    ).toEqual([
      { stage: "PRECHECK", status: "succeeded" },
      { stage: "CRASH_WRITE_STARTED", status: "unknown" },
    ]);
  });

  it("shows a terminal event as its own row when no stage preceded it", () => {
    expect(stageList([{ stage: "failed", status: "failed" }])).toEqual([
      { stage: "failed", status: "failed" },
    ]);
  });
});

describe("targetStatus", () => {
  it("describes the last preflight in words, not only a lamp colour", () => {
    expect(targetStatus(undefined, "192.168.31.1")).toEqual({
      tone: "neutral",
      label: "尚未检测",
      detail: "",
    });
    expect(targetStatus(doctorOK, "192.168.31.1")).toEqual({
      tone: "ok",
      label: "上次检测成功",
      detail: "Xiaomi BE6500 Pro · 1.1.96",
    });
    expect(targetStatus({ status: "failed", host: "192.168.31.1" }, "192.168.31.1")).toMatchObject({
      tone: "danger",
      label: "上次检测失败",
    });
  });

  it("does not attribute another host's result to the edited target", () => {
    expect(targetStatus(doctorOK, "10.0.0.1")).toEqual({
      tone: "neutral",
      label: "目标已更改，请重新检测",
      detail: "上次检测的是 192.168.31.1",
    });
  });
});

describe("guideStatus", () => {
  it("names the guided page's state, including canceled and rejected runs", () => {
    expect(guideStatus("idle", null, "")).toBe("idle");
    expect(guideStatus("detecting", null, "")).toBe("running");
    expect(guideStatus("running", null, "")).toBe("running");
    expect(guideStatus("done", { status: "partial" }, "")).toBe("partial");
    expect(guideStatus("done", { status: "failed", error: { code: "CANCELED" } }, "")).toBe(
      "canceled",
    );
    expect(guideStatus("done", null, "duplicate operation")).toBe("failed");
    expect(guideStatus("planned", null, "")).toBe("idle");
  });
});
