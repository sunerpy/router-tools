import { useGuide } from "./useGuide";
import { useOperations } from "./useOperations";
import type {
  GuideApplyReport,
  GuidePlanReport,
  ProgressPayload,
  RouterAPI,
} from "../services/api";

function freshPlan(): GuidePlanReport {
  return {
    schema_version: 1,
    operation: "guide plan",
    status: "succeeded",
    host: "192.168.31.1",
    device: { device_id: "DEVICE", hardware: "RD08", rom_version: "1.1.96", new_encrypt_mode: 1 },
    profile: {
      id: "xiaomi-rd08-1.1.96",
      model_id: "xiaomi-rd08",
      vendor: "Xiaomi",
      model: "BE6500 Pro",
      hardware: "RD08",
      firmware: "1.1.96",
      capabilities: [],
    },
    goals: ["ssh", "backup", "soft"],
    state: {
      ssh: "closed",
      root_ssh: "not_checked",
      recovery_blocked: false,
      locked: false,
      saved_admin: false,
      saved_root: false,
    },
    steps: [
      {
        id: "enable_ssh",
        action: "run",
        reason: "ssh_closed",
        effects: ["nvram_commit"],
        inputs: ["admin_password", "new_root_password"],
      },
      { id: "backup", action: "run", reason: "backup_missing", effects: ["local_backup"] },
      {
        id: "soft_install",
        action: "decide_after",
        reason: "after_ssh",
        effects: ["script_write"],
      },
    ],
    ready: true,
    changes: true,
    required_inputs: ["admin_password", "new_root_password"],
    confirmation: {
      schema_version: 1,
      target: { host: "192.168.31.1", web_port: 80, ssh_port: 22, telnet_port: 23 },
      identity: {
        device_id: "DEVICE",
        hardware: "RD08",
        rom_version: "1.1.96",
        profile_id: "xiaomi-rd08-1.1.96",
      },
      goals: ["ssh", "backup", "soft"],
      reset_root_password: false,
      steps: [
        { id: "enable_ssh", action: "run", effects: ["nvram_commit"] },
        { id: "backup", action: "run", effects: ["local_backup"] },
        { id: "soft_install", action: "decide_after", effects: ["script_write"] },
      ],
    },
    plan_digest: "ab".repeat(32),
  };
}

function configuredPlan(): GuidePlanReport {
  const plan = freshPlan();
  plan.steps[0] = { id: "enable_ssh", action: "skip", reason: "root_ssh_verified" };
  plan.confirmation!.steps[0] = { id: "enable_ssh", action: "skip" };
  plan.required_inputs = ["root_password"];
  plan.state = { ...plan.state, ssh: "open", root_ssh: "verified" };
  return plan;
}

function makeAPI(plan: GuidePlanReport = freshPlan()) {
  let progress: ((payload: ProgressPayload) => void) | undefined;
  const api = {
    GuidePlan: vi.fn().mockResolvedValue(plan),
    GuideApply: vi
      .fn()
      .mockResolvedValue({ status: "succeeded", steps: [] } as Partial<GuideApplyReport>),
    AcknowledgeRecovery: vi.fn().mockResolvedValue({ status: "succeeded", acknowledged: true }),
    CredentialStatus: vi.fn().mockResolvedValue({ status: "succeeded", entries: [] }),
    ListReports: vi.fn().mockResolvedValue([]),
    Cancel: vi.fn().mockResolvedValue(true),
    OnProgress: vi.fn((callback) => {
      progress = callback;
      return () => {};
    }),
    OnHostKey: vi.fn().mockReturnValue(() => {}),
  } as unknown as RouterAPI;
  return { api, emit: (payload: ProgressPayload) => progress?.(payload) };
}

function setup(plan?: GuidePlanReport) {
  localStorage.clear();
  const { api, emit } = makeAPI(plan);
  const operations = useOperations(api);
  const guide = useGuide(api, operations);
  return { api, emit, operations, guide };
}

describe("useGuide", () => {
  it("detects with the selected goals, the shared target and saved passwords on by default", async () => {
    const { api, operations, guide } = setup();
    (operations.target as any).ssh_port = "2222";
    guide.goals.backup = false;
    guide.goals.reboot = true;
    guide.rootPassword = "current-root";
    await guide.detect();
    const request = vi.mocked(api.GuidePlan).mock.calls[0][0];
    expect(request.Goals).toEqual(["ssh", "soft", "reboot"]);
    expect(request.Target.ssh_port).toBe(2222);
    expect(request.RootPassword).toBe("current-root");
    expect(request.AdminPassword).toBe("");
    expect(request.UseSavedPassword).toBe(true);
    expect(request.RememberPassword).toBe(false);
    expect(guide.plan?.ready).toBe(true);
  });

  it("drops reboot verification when soft persistence is not selected", async () => {
    const { api, guide } = setup();
    guide.goals.soft = false;
    guide.goals.reboot = true;
    await guide.detect();
    expect(vi.mocked(api.GuidePlan).mock.calls[0][0].Goals).toEqual(["ssh", "backup"]);
  });

  it("asks only for the inputs the plan needs", async () => {
    const { guide } = setup();
    await guide.detect();
    expect(guide.needs).toEqual({ admin: true, currentRoot: false, newRoot: true });
    const configured = setup(configuredPlan());
    await configured.guide.detect();
    expect(configured.guide.needs).toEqual({ admin: false, currentRoot: true, newRoot: false });
  });

  it("applies the plan's confirmation unchanged with the new root password and clears every password", async () => {
    const { api, guide } = setup();
    await guide.detect();
    guide.adminPassword = "admin-secret";
    guide.newRootPassword = "new-root";
    guide.newRootRepeat = "new-root";
    guide.remember = true;
    expect(guide.validate()).toEqual({});
    await guide.apply();
    const request = vi.mocked(api.GuideApply).mock.calls[0][0];
    expect(request.Confirmation).toEqual(freshPlan().confirmation);
    expect(request.PlanDigest).toBe(freshPlan().plan_digest);
    expect(request.Confirmed).toBe(true);
    expect(request.AdminPassword).toBe("admin-secret");
    expect(request.RootPassword).toBe("new-root");
    expect(request.RememberPassword).toBe(true);
    expect([
      guide.adminPassword,
      guide.rootPassword,
      guide.newRootPassword,
      guide.newRootRepeat,
    ]).toEqual(["", "", "", ""]);
    expect(JSON.stringify(localStorage)).not.toMatch(/admin-secret|new-root/);
  });

  it("clears the current root password once detection is dispatched and asks for it again", async () => {
    const { api, guide } = setup(configuredPlan());
    guide.rootPassword = "current-root";
    await guide.detect();
    expect(vi.mocked(api.GuidePlan).mock.calls[0][0].RootPassword).toBe("current-root");
    expect(guide.rootPassword).toBe("");
    expect(Object.keys(guide.validate())).toEqual(["guide-root"]);
    guide.rootPassword = "current-root-again";
    await guide.apply();
    expect(vi.mocked(api.GuideApply).mock.calls[0][0].RootPassword).toBe("current-root-again");
  });

  it("canonicalizes the host like the vault when it shows saved passwords", async () => {
    const { api, operations, guide } = setup();
    vi.mocked(api.CredentialStatus).mockResolvedValue({
      status: "succeeded",
      entries: [{ kind: "root", scope: "xiaomi-rd08", host: "router.lan", port: 22, saved_at: "" }],
    } as any);
    operations.target.host = " Router.LAN. ";
    await guide.loadCredentials();
    expect(guide.savedState("root")).toBe("saved");
  });

  it("treats a plan as stale once the target, profile, goals or saved-password choice change", async () => {
    const { api, operations, guide } = setup();
    await guide.detect();
    expect(guide.stale).toBe(false);
    operations.target.host = "10.0.0.9";
    expect(guide.stale).toBe(true);
    operations.target.host = "192.168.31.1";
    expect(guide.stale).toBe(false);
    guide.useSaved = false;
    expect(guide.stale).toBe(true);
    guide.useSaved = true;
    guide.goals.backup = false;
    expect(guide.stale).toBe(true);
    guide.goals.backup = true;
    operations.profileID = "xiaomi-rd08-1.1.96";
    expect(guide.stale).toBe(true);
    guide.adminPassword = "admin-secret";
    guide.newRootPassword = guide.newRootRepeat = "new-root";
    await guide.apply();
    expect(api.GuideApply).not.toHaveBeenCalled();
  });

  it("shows why a run was rejected before it started", async () => {
    const { api, guide } = setup();
    await guide.detect();
    guide.adminPassword = "admin-secret";
    guide.newRootPassword = guide.newRootRepeat = "new-root";
    vi.mocked(api.GuideApply).mockRejectedValue(new Error("duplicate operation"));
    await guide.apply();
    expect(guide.run).toBeNull();
    expect(guide.phase).toBe("done");
    expect(guide.error).toBe("duplicate operation");
  });

  it("cancels a running detection or run by its own operation ID", async () => {
    const { api, guide } = setup();
    await guide.detect();
    guide.adminPassword = "admin-secret";
    guide.newRootPassword = guide.newRootRepeat = "new-root";
    let resolve: (value: unknown) => void = () => {};
    vi.mocked(api.GuideApply).mockReturnValue(new Promise((done) => (resolve = done)) as any);
    const running = guide.apply();
    const id = vi.mocked(api.GuideApply).mock.calls[0][0].OperationID;
    await guide.cancel();
    expect(api.Cancel).toHaveBeenCalledWith(id);
    resolve({ status: "failed", steps: [], error: { code: "CANCELED", message: "canceled" } });
    await running;
    expect(guide.run?.error?.code).toBe("CANCELED");
  });

  it("refuses a run whose inputs are missing or whose new passwords differ", async () => {
    const { api, guide } = setup();
    await guide.detect();
    guide.newRootPassword = "new-root";
    guide.newRootRepeat = "other";
    const errors = guide.validate();
    expect(Object.keys(errors).sort()).toEqual(["guide-admin", "guide-new-root-repeat"]);
    await guide.apply();
    expect(api.GuideApply).not.toHaveBeenCalled();
  });

  it("counts a saved admin password as provided", async () => {
    const plan = freshPlan();
    plan.state.saved_admin = true;
    plan.required_inputs = ["new_root_password"];
    const { guide } = setup(plan);
    await guide.detect();
    expect(guide.needs.admin).toBe(false);
  });

  it("collects progress per guided step", async () => {
    const { api, emit, guide } = setup();
    await guide.detect();
    guide.adminPassword = "admin-secret";
    guide.newRootPassword = guide.newRootRepeat = "new-root";
    let resolve: (value: unknown) => void = () => {};
    vi.mocked(api.GuideApply).mockReturnValue(new Promise((done) => (resolve = done)) as any);
    const running = guide.apply();
    const id = vi.mocked(api.GuideApply).mock.calls[0][0].OperationID;
    emit({
      operation_id: id,
      event: {
        operation: "enable",
        stage: "probe",
        status: "running",
        step: "enable_ssh",
        time: "",
      },
    });
    emit({
      operation_id: "other",
      event: {
        operation: "enable",
        stage: "probe",
        status: "running",
        step: "enable_ssh",
        time: "",
      },
    });
    expect(guide.stepEvents.enable_ssh).toHaveLength(1);
    resolve({ status: "succeeded", steps: [] });
    await running;
    expect(guide.phase).toBe("done");
  });

  it("acknowledges the router the blocked plan belongs to, then detects again", async () => {
    const { api, operations, guide } = setup();
    await guide.detect();
    operations.target.host = "192.168.31.2";
    await guide.acknowledge();
    expect(vi.mocked(api.AcknowledgeRecovery).mock.calls[0][0].Target.host).toBe("192.168.31.1");
    expect(api.GuidePlan).toHaveBeenCalledTimes(2);
    expect(guide.ackError).toBe("");
  });

  it("reports an acknowledgement that failed or was rejected and does not detect again", async () => {
    const { api, guide } = setup();
    await guide.detect();
    vi.mocked(api.AcknowledgeRecovery).mockResolvedValueOnce({
      status: "failed",
      error: { code: "STATE_STORE_FAILED", message: "disk full" },
    } as any);
    await guide.acknowledge();
    expect(guide.ackError).toContain("disk full");
    vi.mocked(api.AcknowledgeRecovery).mockRejectedValueOnce(new Error("bridge closed"));
    await guide.acknowledge();
    expect(guide.ackError).toBe("bridge closed");
    expect(api.GuidePlan).toHaveBeenCalledTimes(1);
  });

  it("keeps a partial run and its warning when a later step finds that the router changed", async () => {
    const { api, guide } = setup();
    await guide.detect();
    guide.adminPassword = "admin-secret";
    guide.newRootPassword = guide.newRootRepeat = "new-root";
    vi.mocked(api.GuideApply).mockResolvedValue({
      status: "partial",
      steps: [
        { id: "enable_ssh", action: "run", status: "succeeded" },
        {
          id: "backup",
          action: "run",
          status: "failed",
          error: { code: "CONFIRMATION_REQUIRED", message: "changed" },
        },
      ],
      error: { code: "CONFIRMATION_REQUIRED", message: "changed" },
    } as any);
    await guide.apply();
    expect(guide.run?.status).toBe("partial");
    expect(guide.planChanged).toBe(false);
    expect(api.GuidePlan).toHaveBeenCalledTimes(1);
  });

  it("shows a fresh detection, not a retry, when the run reports that the plan changed", async () => {
    const { api, guide } = setup();
    await guide.detect();
    guide.adminPassword = "admin-secret";
    guide.newRootPassword = guide.newRootRepeat = "new-root";
    vi.mocked(api.GuideApply).mockResolvedValue({
      status: "failed",
      steps: [],
      error: { code: "CONFIRMATION_REQUIRED", message: "changed" },
    } as any);
    await guide.apply();
    expect(api.GuideApply).toHaveBeenCalledTimes(1);
    expect(api.GuidePlan).toHaveBeenCalledTimes(2);
    expect(guide.planChanged).toBe(true);
  });
});
