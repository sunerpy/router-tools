import { createAPI } from "./wails";
import type { GuidePlanReport, GuideRequest } from "./api";

function guideRequest(overrides: Partial<GuideRequest> = {}): GuideRequest {
  return {
    OperationID: "guide-op",
    Target: { host: "192.168.31.1", web_port: 80, ssh_port: 22, telnet_port: 23 },
    ProfileID: "auto",
    Goals: ["ssh", "backup", "soft"],
    AdminPassword: "",
    RootPassword: "",
    ResetRootPassword: false,
    Confirmation: null,
    PlanDigest: "",
    Confirmed: false,
    ToolVersion: "dev",
    HostKey: { KnownHostsPath: "", PinSHA256: "", AcceptNew: false },
    UseSavedPassword: false,
    RememberPassword: false,
    ...overrides,
  };
}

const actions = (plan: GuidePlanReport) => plan.steps.map((step) => `${step.id}:${step.action}`);

describe("preview guided flow", () => {
  afterEach(() => {
    delete window.__ROUTER_PREVIEW_SCENARIO__;
  });

  it("plans a fresh device with a confirmation and no history entry", async () => {
    const api = createAPI();
    const plan = await api.GuidePlan(guideRequest());

    expect(plan.status).toBe("succeeded");
    expect(actions(plan)).toEqual(["enable_ssh:run", "backup:run", "soft_install:decide_after"]);
    expect(plan.ready).toBe(true);
    expect(plan.changes).toBe(true);
    expect(plan.required_inputs).toEqual(["admin_password", "new_root_password"]);
    expect(plan.confirmation?.identity.profile_id).toBe("xiaomi-rd08-1.1.96");
    expect(plan.plan_digest).toMatch(/^[0-9a-f]{64}$/);
    expect(await api.ListReports()).toEqual([]);
  });

  it("only includes the selected goals", async () => {
    const plan = await createAPI().GuidePlan(guideRequest({ Goals: ["ssh", "soft", "reboot"] }));
    expect(actions(plan)).toEqual([
      "enable_ssh:run",
      "soft_install:decide_after",
      "reboot_verify:run",
    ]);
  });

  it("has nothing to do on an already configured device", async () => {
    window.__ROUTER_PREVIEW_SCENARIO__ = "configured";
    const plan = await createAPI().GuidePlan(guideRequest());
    expect(plan.steps.every((step) => step.action === "skip")).toBe(true);
    expect(plan.changes).toBe(false);
    expect(plan.state.root_ssh).toBe("verified");
  });

  it("resolves a blocked plan with the recovery record instead of rejecting", async () => {
    window.__ROUTER_PREVIEW_SCENARIO__ = "recovery";
    const plan = await createAPI().GuidePlan(guideRequest());
    expect(plan.status).toBe("failed");
    expect(plan.error?.code).toBe("RECOVERY_REVIEW_REQUIRED");
    expect(plan.state.recovery_blocked).toBe(true);
    expect(plan.state.recovery?.status).toBe("unknown");
    expect(plan.steps.every((step) => step.action === "blocked")).toBe(true);
    expect(plan.confirmation).toBeUndefined();
  });

  it("lists the supported firmware when the firmware is not registered", async () => {
    window.__ROUTER_PREVIEW_SCENARIO__ = "unsupported";
    const plan = await createAPI().GuidePlan(guideRequest());
    expect(plan.error?.code).toBe("UNSUPPORTED_FIRMWARE");
    expect(plan.candidates?.map((candidate) => candidate.profile.id)).toEqual([
      "xiaomi-rd08-1.1.96",
    ]);
  });

  it("applies a confirmed plan into one history entry and never echoes passwords", async () => {
    const api = createAPI();
    const plan = await api.GuidePlan(guideRequest());
    const report = await api.GuideApply(
      guideRequest({
        OperationID: "guide-run",
        Confirmation: plan.confirmation!,
        PlanDigest: plan.plan_digest!,
        Confirmed: true,
        AdminPassword: "admin-secret",
        RootPassword: "root-secret",
      }),
    );
    expect(report.status).toBe("succeeded");
    expect(report.steps.map((step) => `${step.id}:${step.status}`)).toEqual([
      "enable_ssh:succeeded",
      "backup:succeeded",
      "soft_install:succeeded",
    ]);
    const history = await api.ListReports();
    expect(history.map((entry) => entry.operation)).toEqual(["guide apply"]);
    expect(JSON.stringify(history)).not.toMatch(/admin-secret|root-secret/);
  });

  it("stops a partial run at the failing step", async () => {
    window.__ROUTER_PREVIEW_SCENARIO__ = "partial";
    const api = createAPI();
    const plan = await api.GuidePlan(guideRequest());
    const report = await api.GuideApply(
      guideRequest({
        Confirmation: plan.confirmation!,
        PlanDigest: plan.plan_digest!,
        Confirmed: true,
        AdminPassword: "admin-secret",
        RootPassword: "root-secret",
      }),
    );
    expect(report.status).toBe("partial");
    expect(report.steps.map((step) => step.status)).toEqual(["succeeded", "failed", "not_run"]);
    expect(report.error?.code).toBe("BACKUP_INCOMPLETE");
  });

  it("rejects an apply without a confirmation like the desktop binding", async () => {
    await expect(createAPI().GuideApply(guideRequest({ Confirmed: true }))).rejects.toThrow();
  });

  it("records a recovery acknowledgement", async () => {
    const api = createAPI();
    const report = await api.AcknowledgeRecovery({
      OperationID: "ack",
      Target: { host: "192.168.31.1", web_port: 80, ssh_port: 22, telnet_port: 23 },
      ProfileID: "auto",
      AdminPassword: "",
      RootPassword: "",
      Transport: "ssh",
      AllowTelnet: false,
      Confirmed: true,
      WithAdminAuth: false,
      WithRootVerify: false,
      OutputDir: "",
      BackupDir: "",
      RunDir: "",
      AckDeep: "",
      ToolVersion: "dev",
      HostKey: { KnownHostsPath: "", PinSHA256: "", AcceptNew: false },
      UseSavedPassword: false,
      RememberPassword: false,
    });
    expect(report.acknowledged).toBe(true);
    expect((await api.ListReports())[0].operation).toBe("recovery ack");
  });
});
