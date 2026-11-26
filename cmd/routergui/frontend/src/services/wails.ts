import type {
  CatalogEntry,
  CredentialEntry,
  CredentialReport,
  DeepStatusReport,
  GuideApplyReport,
  GuideGoal,
  GuidePlanReport,
  GuideRequest,
  GuideStep,
  GuideStepResult,
  HistoryEntry,
  HostKeyPrompt,
  OperationRequest,
  Profile,
  RecoveryReport,
  RouterAPI,
} from "./api";

declare global {
  interface Window {
    go?: any;
    runtime?: {
      EventsOn: (name: string, callback: (data: any) => void) => () => void;
      WindowSetLightTheme?: () => void;
      WindowSetDarkTheme?: () => void;
      WindowSetSystemDefaultTheme?: () => void;
    };
    __ROUTER_PREVIEW_SCENARIO__?: string;
    __routerHostKey?: () => void;
  }
}

export const isPreview = import.meta.env.DEV && !window.go?.main?.App;

export function createAPI(): RouterAPI {
  if (window.go?.main?.App) {
    const app = window.go.main.App;
    return {
      Doctor: (request) => app.Doctor(request),
      Verify: (request) => app.Verify(request),
      EnableSSH: (request) => app.EnableSSH(request),
      PersistSoft: (request) => app.PersistSoft(request),
      PersistRemove: (request) => app.PersistRemove(request),
      RebootVerify: (request) => app.RebootVerify(request),
      BackupCreate: (request) => app.BackupCreate(request),
      BackupVerify: (request) => app.BackupVerify(request),
      BackupVerifyOnline: (request) => app.BackupVerifyOnline(request),
      DeepPersist: (request) => app.DeepPersist(request),
      GuidePlan: (request) => app.GuidePlan(request),
      GuideApply: (request) => app.GuideApply(request),
      AcknowledgeRecovery: (request) => app.AcknowledgeRecovery(request),
      DeepStatus: (request) => app.DeepStatus(request),
      CredentialStatus: () => app.CredentialStatus(),
      ForgetCredential: (filter) => app.ForgetCredential(filter),
      ForgetAllCredentials: () => app.ForgetAllCredentials(),
      Catalog: () => app.Catalog(),
      Cancel: (id) => app.Cancel(id),
      ListReports: () => app.ListReports(),
      SelectDirectory: () => app.SelectDirectory(),
      ResolveHostKey: (id, accept) => app.ResolveHostKey(id, accept),
      OnProgress: (callback) => window.runtime!.EventsOn("operation:progress", callback),
      OnHostKey: (callback) => window.runtime!.EventsOn("hostkey:confirm", callback),
    };
  }
  if (isPreview) return createMockAPI();
  const missing = () =>
    Promise.reject(new Error("Wails 后端绑定不可用；请使用桌面应用而不是静态页面。"));
  return new Proxy({} as RouterAPI, { get: () => missing });
}

// Preview-only RD08 profile shaped like device.Profile; the catalog and preview reports share it.
const previewProfile: Profile = {
  id: "xiaomi-rd08-1.1.96",
  model_id: "xiaomi-rd08",
  vendor: "Xiaomi",
  model: "BE6500 Pro",
  hardware: "RD08",
  firmware: "1.1.96",
  capabilities: [
    "doctor",
    "web_auth",
    "verify",
    "enable_ssh",
    "soft_persist",
    "backup",
    "deep_persist",
  ],
  deep: { acknowledgement: "RD08-DEEP", reboot_transitions: 3 },
};

const previewCatalog: CatalogEntry[] = [
  {
    model_id: "xiaomi-rd08",
    vendor: "Xiaomi",
    model: "BE6500 Pro",
    hardware: "RD08",
    profiles: [previewProfile],
  },
];

// Preview-only identity and port evidence shaped like app.DoctorReport (labelled 模拟数据 in the UI).
function previewDoctorFields(request: OperationRequest, failed: boolean) {
  if (failed) {
    return {
      device: { device_id: "", rom_version: "", new_encrypt_mode: 0 },
      profile: {
        id: "",
        model_id: "",
        vendor: "",
        model: "",
        hardware: "",
        firmware: "",
        capabilities: null,
      },
      ssh: "",
      telnet: "",
      admin_auth: request.WithAdminAuth ? "failed" : "skipped",
      root_verify: request.WithRootVerify ? "failed" : "skipped",
    };
  }
  return {
    device: {
      device_id: "PREVIEW",
      hardware: "RD08",
      display_name: "Xiaomi BE6500 Pro",
      rom_version: "1.1.96",
      new_encrypt_mode: 1,
    },
    profile: previewProfile,
    ssh: "open",
    telnet: "closed",
    admin_auth: request.WithAdminAuth ? "verified" : "skipped",
    root_verify: request.WithRootVerify ? "verified" : "skipped",
  };
}

// Preview-only guided plans shaped like app.GuidePlanReport. Scenarios: default fresh device,
// "configured" (nothing to do), "recovery" (blocked by an unacknowledged change), "unsupported"
// (unregistered firmware with candidates) and "failed" (detection failed).
const previewDigest = "0123456789abcdef".repeat(4);
const previewIdentity = {
  device_id: "PREVIEW",
  hardware: "RD08",
  display_name: "Xiaomi BE6500 Pro",
  rom_version: "1.1.96",
  profile_id: "xiaomi-rd08-1.1.96",
};
const enableEffects = [
  "temporary_mac_rules",
  "nvram_commit",
  "dropbear_start",
  "root_password_change",
];
const softEffects = ["script_write", "uci_commit"];

function previewGoals(goals: GuideGoal[]): GuideGoal[] {
  const order: GuideGoal[] = ["ssh", "backup", "soft", "reboot"];
  return order.filter((goal) => goal === "ssh" || goals.includes(goal));
}

function previewSteps(goals: GuideGoal[], configured: boolean): GuideStep[] {
  const steps: Record<GuideGoal, GuideStep> = configured
    ? {
        ssh: { id: "enable_ssh", action: "skip", reason: "root_ssh_verified" },
        backup: { id: "backup", action: "skip", reason: "backup_verified" },
        soft: { id: "soft_install", action: "skip", reason: "soft_installed_verified" },
        reboot: {
          id: "reboot_verify",
          action: "run",
          reason: "reboot_selected",
          effects: ["reboot"],
        },
      }
    : {
        ssh: {
          id: "enable_ssh",
          action: "run",
          reason: "ssh_closed",
          effects: enableEffects,
          inputs: ["admin_password", "new_root_password"],
        },
        backup: {
          id: "backup",
          action: "run",
          reason: "backup_missing",
          effects: ["local_backup"],
        },
        soft: {
          id: "soft_install",
          action: "decide_after",
          reason: "after_ssh",
          effects: softEffects,
        },
        reboot: {
          id: "reboot_verify",
          action: "run",
          reason: "reboot_selected",
          effects: ["reboot"],
        },
      };
  return goals.map((goal) => steps[goal]);
}

function previewPlan(
  request: GuideRequest,
  scenario: string,
  vault: CredentialEntry[],
): GuidePlanReport {
  const goals = previewGoals(request.Goals);
  const device = {
    device_id: "PREVIEW",
    hardware: "RD08",
    display_name: "Xiaomi BE6500 Pro",
    rom_version: scenario === "unsupported" ? "1.1.97" : "1.1.96",
    new_encrypt_mode: 1,
  };
  const base = {
    schema_version: 1,
    operation: "guide plan" as const,
    host: request.Target.host,
    device,
    profile: previewProfile,
    goals,
    state: {
      ssh: "closed",
      telnet: "closed",
      root_ssh: "not_checked" as const,
      recovery_blocked: false,
      locked: false,
      saved_admin: false,
      saved_root: false,
    },
  };
  const blocked = (reason: string) =>
    goals.map((goal) => ({
      ...previewSteps([goal], false)[0],
      action: "blocked" as const,
      reason,
      effects: undefined,
      inputs: undefined,
    }));
  switch (scenario) {
    case "recovery":
      return {
        ...base,
        status: "failed",
        device: { device_id: "", rom_version: "", new_encrypt_mode: 0 },
        profile: { ...previewProfile, id: "", capabilities: null },
        state: {
          ...base.state,
          ssh: undefined,
          telnet: undefined,
          recovery_blocked: true,
          recovery: {
            schema_version: 1,
            operation: "enable",
            host: request.Target.host,
            status: "unknown",
            error_code: "REMOTE_RESULT_UNKNOWN",
            device_write_possible: true,
            write_evidence: {
              write_started: true,
              accepted_steps: 1,
              attempted_macs: ["0A:11:22:33:44:10"],
            },
            started_at: "2026-11-26T03:00:00Z",
            finished_at: "2026-11-26T03:01:00Z",
          },
        },
        steps: blocked("recovery_review_required"),
        ready: false,
        changes: false,
        error: {
          code: "RECOVERY_REVIEW_REQUIRED",
          message: "预览：上次开启 SSH 的结果未知，请先人工检查路由器并确认。",
        },
      };
    case "unsupported":
      return {
        ...base,
        status: "failed",
        profile: { ...previewProfile, id: "", capabilities: null },
        candidates: [{ profile: previewProfile, exact: false }],
        steps: blocked("incompatible"),
        ready: false,
        changes: false,
        error: { code: "UNSUPPORTED_FIRMWARE", message: "预览：固件 1.1.97 没有注册的 profile。" },
      };
    case "failed":
      return {
        ...base,
        status: "failed",
        state: { ...base.state, ssh: undefined, telnet: undefined },
        steps: blocked("detection_failed"),
        ready: false,
        changes: false,
        error: {
          code: "WEB_UNREACHABLE",
          message: "预览：无法连接 Web 管理端口，请检查地址和网络。",
        },
      };
  }
  const configured = scenario === "configured";
  const steps = previewSteps(goals, configured);
  const changes = steps.some((step) => step.action === "run" || step.action === "decide_after");
  const saved = (kind: CredentialEntry["kind"], port: number) =>
    request.UseSavedPassword &&
    vault.some(
      (entry) =>
        entry.kind === kind &&
        entry.host === request.Target.host &&
        entry.port === port &&
        !entry.rejected_at,
    );
  const savedAdmin = !configured && saved("web-admin", request.Target.web_port);
  const savedRoot = configured && saved("root", request.Target.ssh_port);
  const inputs = !changes
    ? []
    : configured
      ? ["root_password"]
      : ["admin_password", "new_root_password"];
  const requiredInputs = inputs.filter(
    (input) =>
      !(input === "admin_password" && savedAdmin) && !(input === "root_password" && savedRoot),
  );
  return {
    ...base,
    status: "succeeded",
    state: {
      ...(configured
        ? {
            ...base.state,
            ssh: "open",
            root_ssh: "verified" as const,
            soft: "installed_verified",
            backup: {
              path: "~/RouterTools/backups/xiaomi-rd08/preview",
              created_at: "2026-11-26T02:00:00Z",
              boot_id: "preview",
            },
          }
        : base.state),
      saved_admin: savedAdmin,
      saved_root: savedRoot,
    },
    steps,
    ready: true,
    changes,
    required_inputs: requiredInputs.length > 0 ? requiredInputs : undefined,
    confirmation: {
      schema_version: 1,
      target: request.Target,
      identity: previewIdentity,
      goals,
      reset_root_password: request.ResetRootPassword,
      steps: steps.map((step) => ({ id: step.id, action: step.action, effects: step.effects })),
    },
    plan_digest: previewDigest,
  };
}

function previewApply(request: GuideRequest, scenario: string): GuideApplyReport {
  const confirmation = request.Confirmation!;
  const failure: Record<string, { at: number; status: string; code: string; message: string }> = {
    partial: {
      at: 1,
      status: "failed",
      code: "BACKUP_INCOMPLETE",
      message: "预览：备份读取不完整。",
    },
    unknown: {
      at: 0,
      status: "unknown",
      code: "REMOTE_RESULT_UNKNOWN",
      message: "预览：请检查报告和设备，不要从头重跑。",
    },
    failed: {
      at: 0,
      status: "failed",
      code: "WEB_AUTH_FAILED",
      message: "预览：Web 管理密码不正确。",
    },
  };
  const stop = failure[scenario];
  let stopped = false;
  let wrote = false;
  const steps: GuideStepResult[] = confirmation.steps.map((step, index) => {
    if (stopped)
      return { id: step.id, action: "not_run", status: "not_run", effects: step.effects };
    if (step.action === "skip") return { id: step.id, action: "skip", status: "skipped" };
    if (stop && index === stop.at) {
      stopped = true;
      return {
        id: step.id,
        action: "run",
        status: stop.status,
        effects: step.effects,
        error: { code: stop.code, message: stop.message },
      };
    }
    wrote = true;
    return { id: step.id, action: "run", status: "succeeded", effects: step.effects };
  });
  const status = !stop
    ? "succeeded"
    : stop.status === "unknown"
      ? "unknown"
      : wrote
        ? "partial"
        : "failed";
  return {
    schema_version: 1,
    operation: "guide apply",
    status,
    host: confirmation.target.host,
    device: {
      device_id: previewIdentity.device_id,
      hardware: previewIdentity.hardware,
      display_name: previewIdentity.display_name,
      rom_version: previewIdentity.rom_version,
      new_encrypt_mode: 1,
    },
    profile: previewProfile,
    plan_digest: request.PlanDigest,
    steps,
    error: stop ? { code: stop.code, message: stop.message } : undefined,
  };
}

// Preview-only deep status shaped like app.DeepStatusReport.
// Like the Go use case, the preview keeps the deep journal in the managed directory; the window never
// chooses another one.
const PREVIEW_JOURNAL =
  "~/.config/routerctl/deep/xiaomi-rd08/192.168.31.1-preview/192.168.31.1-deep.json";

function previewDeepStatus(
  request: OperationRequest,
  scenario: string,
  vault: CredentialEntry[],
): DeepStatusReport {
  const host = request.Target.host.toLowerCase();
  const savedRoot =
    request.UseSavedPassword &&
    vault.some(
      (entry) =>
        entry.kind === "root" && entry.host === host && entry.port === request.Target.ssh_port,
    );
  const verified = Boolean(request.RootPassword) || savedRoot;
  const blocked: DeepStatusReport["blocked"] = [];
  if (scenario === "recovery") blocked.push("recovery_review_required");
  if (scenario === "journal") blocked.push("journal_exists");
  const backups =
    scenario === "failed"
      ? []
      : [
          {
            path: "~/RouterTools/backups/xiaomi-rd08/192.168.31.1-preview/20261126T010000Z",
            created_at: "2026-11-26T01:00:00Z",
            boot_id: "preview",
          },
        ];
  if (backups.length === 0) blocked.push("no_verified_backup");
  return {
    schema_version: 1,
    operation: "deep status",
    status: "succeeded",
    host: request.Target.host,
    device: {
      device_id: "PREVIEW",
      hardware: "RD08",
      display_name: "Xiaomi BE6500 Pro",
      rom_version: "1.1.96",
      new_encrypt_mode: 1,
    },
    profile: previewProfile,
    journal_path: PREVIEW_JOURNAL,
    journal_exists: scenario === "journal",
    backups,
    ssh: "open",
    root_ssh: verified ? "verified" : "not_checked",
    inspection: verified
      ? { flags: "none", crash_partition: true, crash_readable: true }
      : undefined,
    blocked,
    ready: verified && blocked.length === 0,
    credentials:
      savedRoot && !request.RootPassword ? [{ kind: "root", action: "used_saved" }] : undefined,
  };
}

function createMockAPI(): RouterAPI {
  const history: HistoryEntry[] = [];
  // Preview-only vault: metadata of "saved" passwords, never a value.
  let vault: CredentialEntry[] = [];
  const credentialReport = (operation: string, entries: CredentialEntry[]): CredentialReport => ({
    schema_version: 1,
    operation,
    status: "succeeded",
    entries: entries.map((entry) => ({ ...entry })),
  });
  const progressListeners = new Set<(payload: any) => void>();
  const progressListener = (payload: any) =>
    progressListeners.forEach((listener) => listener(payload));
  let hostKeyListener: ((prompt: HostKeyPrompt) => void) | undefined;

  window.__routerHostKey = () =>
    hostKeyListener?.({
      prompt_id: "preview-host-key",
      operation_id: "preview",
      address: "192.168.31.1:22",
      algorithm: "ssh-ed25519",
      fingerprint: "SHA256:previewOnlyDoNotUseOnRealDevice",
    });

  async function run(operation: string, request: OperationRequest): Promise<any> {
    const scenario = window.__ROUTER_PREVIEW_SCENARIO__ || "success";
    const failed = scenario === "failed";
    progressListener({
      operation_id: request.OperationID,
      event: { operation, stage: "PRECHECK", status: "running", time: new Date().toISOString() },
    });
    await new Promise((resolve) => setTimeout(resolve, 180));
    progressListener({
      operation_id: request.OperationID,
      event: {
        operation,
        stage: failed ? "failed" : "COMPLETED",
        status: failed ? "failed" : scenario === "unknown" ? "unknown" : "succeeded",
        time: new Date().toISOString(),
      },
    });
    const status = failed
      ? "failed"
      : scenario === "unknown"
        ? "unknown"
        : scenario === "partial"
          ? "partial"
          : "succeeded";
    // Like the desktop bindings, failed, partial, and unknown reports resolve rather than reject.
    const report: any = {
      schema_version: 1,
      operation,
      status,
      host: request.Target.host,
      compatible: !failed,
      ssh_verified:
        !failed &&
        (operation === "doctor" ? request.WithRootVerify : operation !== "backup verify"),
      ...(operation === "doctor" ? previewDoctorFields(request, failed) : {}),
      result:
        operation === "deep-persist" && !failed
          ? {
              final_ssh_verified: status === "succeeded",
              soft_verified: true,
              transitions: [
                { old_boot_id: "1", new_boot_id: "2" },
                { old_boot_id: "2", new_boot_id: "3" },
                { old_boot_id: "3", new_boot_id: "4" },
              ],
            }
          : undefined,
      journal_path: operation === "deep-persist" && !failed ? PREVIEW_JOURNAL : undefined,
      message: scenario === "long" ? "X".repeat(6000) : undefined,
      error:
        status === "succeeded"
          ? undefined
          : failed
            ? {
                code: "WEB_UNREACHABLE",
                message: "预览：无法连接 Web 管理端口，请检查地址和网络。",
              }
            : {
                code: status === "unknown" ? "REMOTE_RESULT_UNKNOWN" : "MAC_CLEANUP_FAILED",
                message: "预览：请检查报告和设备，不要从头重跑。",
              },
    };
    history.unshift({
      operation_id: request.OperationID,
      operation,
      report,
      time: new Date().toISOString(),
    });
    return report;
  }

  return {
    Doctor: (request) => run("doctor", request),
    Verify: (request) => run("verify", request),
    EnableSSH: (request) => run("enable", request),
    PersistSoft: (request) => run("persist soft", request),
    PersistRemove: (request) => run("persist remove", request),
    RebootVerify: (request) => run("reboot-verify", request),
    BackupCreate: (request) => run("backup create", request),
    BackupVerify: (request) => run("backup verify", request),
    BackupVerifyOnline: (request) => run("backup verify online", request),
    DeepPersist: (request) => run("deep-persist", request),
    // A plan changes nothing and stays out of the history, like the desktop binding.
    GuidePlan: async (request) => {
      await new Promise((resolve) => setTimeout(resolve, 120));
      return previewPlan(request, window.__ROUTER_PREVIEW_SCENARIO__ || "success", vault);
    },
    GuideApply: async (request) => {
      if (!request.Confirmation) {
        throw new Error("a guided run requires the confirmation returned by its plan");
      }
      if (!request.RootPassword && !request.UseSavedPassword) {
        // Failed reports resolve, as the desktop binding does.
        const refused: GuideApplyReport = {
          schema_version: 1,
          operation: "guide apply",
          status: "failed",
          host: request.Confirmation.target.host,
          device: { device_id: "", rom_version: "", new_encrypt_mode: 0 },
          profile: { ...previewProfile, id: "", capabilities: null },
          plan_digest: request.PlanDigest,
          steps: [],
          error: { code: "INVALID_PASSWORD", message: "password must not be empty" },
        };
        return refused;
      }
      const scenario = window.__ROUTER_PREVIEW_SCENARIO__ || "success";
      for (const step of request.Confirmation.steps) {
        progressListener({
          operation_id: request.OperationID,
          event: {
            operation: "guide apply",
            stage: step.id,
            status: "running",
            step: step.id,
            time: new Date().toISOString(),
          },
        });
      }
      await new Promise((resolve) => setTimeout(resolve, 180));
      const report = previewApply(request, scenario);
      if (request.RememberPassword) {
        // Like the application: a typed password is saved once a step that used it succeeded.
        const target = request.Confirmation.target;
        const host = target.host.toLowerCase();
        const remember = (kind: CredentialEntry["kind"], port: number) => {
          vault = vault.filter(
            (entry) => !(entry.kind === kind && entry.host === host && entry.port === port),
          );
          vault.push({
            kind,
            scope: "xiaomi-rd08",
            host,
            port,
            saved_at: new Date().toISOString(),
          });
        };
        const enabled = report.steps.some(
          (step) => step.id === "enable_ssh" && step.status === "succeeded",
        );
        if (request.AdminPassword && enabled) remember("web-admin", target.web_port);
        if (request.RootPassword && report.steps.some((step) => step.status === "succeeded")) {
          remember("root", target.ssh_port);
        }
      }
      history.unshift({
        operation_id: request.OperationID,
        operation: "guide apply",
        report,
        time: new Date().toISOString(),
      });
      return report;
    },
    AcknowledgeRecovery: async (request) => {
      const report: RecoveryReport = {
        schema_version: 1,
        operation: "recovery ack",
        status: "succeeded",
        host: request.Target.host,
        acknowledged: true,
        last: {
          schema_version: 1,
          operation: "enable",
          host: request.Target.host,
          status: "unknown",
          device_write_possible: true,
          started_at: "2026-11-26T03:00:00Z",
          finished_at: "2026-11-26T03:01:00Z",
          acknowledged_at: new Date().toISOString(),
        },
      };
      history.unshift({
        operation_id: request.OperationID,
        operation: "recovery ack",
        report,
        time: new Date().toISOString(),
      });
      return report;
    },
    DeepStatus: async (request) => {
      await new Promise((resolve) => setTimeout(resolve, 120));
      return previewDeepStatus(request, window.__ROUTER_PREVIEW_SCENARIO__ || "success", vault);
    },
    CredentialStatus: async () => credentialReport("credentials list", vault),
    ForgetCredential: async (filter) => {
      const invalid =
        (!filter.Kind && !filter.Scope && !filter.Host && !filter.Port) ||
        (filter.Kind !== "" && filter.Kind !== "web-admin" && filter.Kind !== "root") ||
        filter.Port < 0 ||
        filter.Port > 65535;
      if (invalid) {
        return {
          ...credentialReport("credentials forget", []),
          status: "failed",
          error: {
            code: "INVALID_ARGUMENT",
            message: "choose which saved passwords to forget, or forget all",
          },
        };
      }
      const matches = (entry: CredentialEntry) =>
        (!filter.Kind || entry.kind === filter.Kind) &&
        (!filter.Scope || entry.scope === filter.Scope) &&
        (!filter.Host || entry.host === filter.Host.toLowerCase()) &&
        (!filter.Port || entry.port === filter.Port);
      const forgotten = vault.filter(matches);
      vault = vault.filter((entry) => !matches(entry));
      return credentialReport("credentials forget", forgotten);
    },
    ForgetAllCredentials: async () => {
      vault = [];
      return credentialReport("credentials forget --all", []);
    },
    Catalog: async () => previewCatalog,
    Cancel: async () => true,
    ListReports: async () => [...history],
    SelectDirectory: async () =>
      "~/RouterTools/backups/xiaomi-rd08/192.168.31.1-preview/20261126T010000Z",
    ResolveHostKey: () => {},
    OnProgress: (callback) => {
      progressListeners.add(callback);
      return () => {
        progressListeners.delete(callback);
      };
    },
    OnHostKey: (callback) => {
      hostKeyListener = callback;
      return () => {
        hostKeyListener = undefined;
      };
    },
  };
}
