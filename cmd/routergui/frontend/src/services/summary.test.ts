import {
  deviceSummary,
  formatTime,
  latestEntry,
  nextStep,
  operationLabel,
  portTiles,
  recentOperations,
  stageList,
  targetStatus,
} from "./summary";
import type { HistoryEntry, Target } from "./api";

const target: Target = { host: "192.168.31.1", web_port: 80, ssh_port: 22, telnet_port: 23 };

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
});

describe("deviceSummary", () => {
  it("summarises the identified device and profile", () => {
    expect(deviceSummary(doctorOK)).toEqual({
      model: "Xiaomi BE6500 Pro",
      hardware: "RD08",
      firmware: "1.1.96",
      profileId: "xiaomi-rd08-1.1.96",
      capabilities: ["doctor", "web_auth", "verify"],
      compatible: true,
    });
  });

  it("returns null when no device was observed", () => {
    expect(deviceSummary(undefined)).toBeNull();
    expect(
      deviceSummary({ status: "failed", device: { device_id: "", rom_version: "" } }),
    ).toBeNull();
  });

  it("falls back to the hardware name and an empty capability list", () => {
    const summary = deviceSummary({
      device: { hardware: "RD08", rom_version: "1.2.0" },
      compatible: false,
    });
    expect(summary?.model).toBe("RD08");
    expect(summary?.capabilities).toEqual([]);
    expect(summary?.compatible).toBe(false);
  });
});

describe("portTiles", () => {
  it("separates port state from authentication evidence", () => {
    const [web, ssh, telnet] = portTiles(doctorOK, target);
    expect(web).toMatchObject({ key: "web", meta: "HTTP 80", tone: "ok", value: "已验证" });
    expect(ssh).toMatchObject({ key: "ssh", meta: "22", tone: "info", value: "端口开放" });
    expect(ssh.detail).toContain("开放端口不算认证");
    expect(telnet).toMatchObject({ key: "telnet", meta: "23", tone: "neutral", value: "已关闭" });
  });

  it("reports verified root SSH and an open plaintext Telnet port", () => {
    const [, ssh, telnet] = portTiles({ ...doctorOK, ssh_verified: true, telnet: "open" }, target);
    expect(ssh).toMatchObject({ tone: "ok", value: "root 已验证" });
    expect(telnet).toMatchObject({ tone: "warn", value: "端口开放" });
    expect(telnet.detail).toContain("明文");
  });

  it("shows an unreachable Web port and unprobed shells after a failed preflight", () => {
    const failed = {
      status: "failed",
      ssh: "",
      telnet: "",
      admin_auth: "skipped",
      error: { code: "WEB_UNREACHABLE", message: "Web TCP port is closed" },
    };
    const [web, ssh, telnet] = portTiles(failed, target);
    expect(web).toMatchObject({ tone: "danger", value: "无法连接" });
    expect(ssh).toMatchObject({ tone: "neutral", value: "未检测" });
    expect(telnet).toMatchObject({ tone: "neutral", value: "未检测" });
  });

  it("marks a rejected admin password", () => {
    const [web] = portTiles(
      { ...doctorOK, status: "failed", admin_auth: "failed", error: { code: "WEB_AUTH_FAILED" } },
      target,
    );
    expect(web).toMatchObject({ tone: "danger", value: "验证失败" });
  });
});

describe("nextStep", () => {
  it("follows the documented order: preflight, enable, then backup", () => {
    expect(nextStep(undefined)).toMatchObject({ title: "运行预检" });
    expect(nextStep(doctorOK)).toMatchObject({ title: "开启 SSH", to: "/enable" });
    expect(nextStep({ ...doctorOK, ssh_verified: true })).toMatchObject({
      title: "创建分区备份",
      to: "/backup",
    });
  });

  it("points a failed preflight at its report", () => {
    const step = nextStep({
      status: "failed",
      error: { code: "WEB_UNREACHABLE", message: "Web TCP port is closed" },
    });
    expect(step).toMatchObject({ title: "排查后重试预检", to: "/reports" });
    expect(step.detail).toContain("Web TCP port is closed");
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
  it("labels port tiles with the target the report was produced for, or not at all", () => {
    const [web, ssh] = portTiles(doctorOK, { ...target, web_port: 8080, ssh_port: 2222 });
    expect(web.meta).toBe("HTTP 8080");
    expect(ssh.meta).toBe("2222");
    const [unknownWeb, unknownSsh] = portTiles(doctorOK, undefined);
    expect(unknownWeb.meta).toBe("");
    expect(unknownSsh.meta).toBe("");
  });

  it("only blames the password when the Web login itself failed", () => {
    const [rejected] = portTiles(
      { ...doctorOK, status: "failed", admin_auth: "failed", error: { code: "WEB_AUTH_FAILED" } },
      target,
    );
    expect(rejected).toMatchObject({ tone: "danger", value: "验证失败" });
    const [unchecked] = portTiles(
      {
        ...doctorOK,
        status: "failed",
        admin_auth: "failed",
        error: { code: "UNSUPPORTED_FIRMWARE" },
      },
      target,
    );
    expect(unchecked).toMatchObject({ tone: "neutral", value: "未验证" });
    expect(unchecked.detail).toContain("UNSUPPORTED_FIRMWARE");
  });

  it("stops at a partial or unknown write before suggesting anything else", () => {
    const history = [
      entry(
        "enable",
        { status: "unknown", error: { code: "REMOTE_RESULT_UNKNOWN" } },
        "2026-11-25T06:00:00Z",
      ),
      entry("doctor", doctorOK, "2026-11-25T05:00:00Z"),
    ];
    expect(nextStep(doctorOK, history)).toMatchObject({
      title: "查看报告，不要从头重跑",
      to: "/reports",
    });
  });

  it("moves on to backup once root SSH is verified by a later run", () => {
    const viaEnable = [
      entry("enable", { status: "succeeded", ssh_verified: true }, "2026-11-25T06:00:00Z"),
      entry("doctor", doctorOK, "2026-11-25T05:00:00Z"),
    ];
    expect(nextStep(doctorOK, viaEnable)).toMatchObject({ title: "创建分区备份", to: "/backup" });
    const viaVerify = [
      entry("verify", { status: "succeeded", ssh_verified: true }, "2026-11-25T06:00:00Z"),
      entry("doctor", doctorOK, "2026-11-25T05:00:00Z"),
    ];
    expect(nextStep(doctorOK, viaVerify)).toMatchObject({ title: "创建分区备份" });
  });

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
      label: "尚未预检",
      detail: "",
    });
    expect(targetStatus(doctorOK, "192.168.31.1")).toEqual({
      tone: "ok",
      label: "上次预检成功",
      detail: "Xiaomi BE6500 Pro · 1.1.96",
    });
    expect(targetStatus({ status: "failed", host: "192.168.31.1" }, "192.168.31.1")).toMatchObject({
      tone: "danger",
      label: "上次预检失败",
    });
  });

  it("does not attribute another host's result to the edited target", () => {
    expect(targetStatus(doctorOK, "10.0.0.1")).toEqual({
      tone: "neutral",
      label: "目标已更改，请重新预检",
      detail: "上次预检的是 192.168.31.1",
    });
  });
});
