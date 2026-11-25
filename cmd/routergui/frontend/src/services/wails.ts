import type { HistoryEntry, HostKeyPrompt, OperationRequest, RouterAPI } from "./api";

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
      Cancel: (id) => app.Cancel(id),
      ListReports: () => app.ListReports(),
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

// Preview-only identity and port evidence shaped like app.DoctorReport (labelled 模拟数据 in the UI).
function previewDoctorFields(request: OperationRequest, failed: boolean) {
  if (failed) {
    return {
      device: { device_id: "", rom_version: "", new_encrypt_mode: 0 },
      profile: { id: "", vendor: "", model: "", hardware: "", firmware: "", capabilities: null },
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
    profile: {
      id: "xiaomi-rd08-1.1.96",
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
    },
    ssh: "open",
    telnet: "closed",
    admin_auth: request.WithAdminAuth ? "verified" : "skipped",
    root_verify: request.WithRootVerify ? "verified" : "skipped",
  };
}

function createMockAPI(): RouterAPI {
  const history: HistoryEntry[] = [];
  let progressListener: ((payload: any) => void) | undefined;
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
    progressListener?.({
      operation_id: request.OperationID,
      event: { operation, stage: "PRECHECK", status: "running", time: new Date().toISOString() },
    });
    await new Promise((resolve) => setTimeout(resolve, 180));
    progressListener?.({
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
      journal_path:
        operation === "deep-persist" && !failed
          ? `${request.RunDir}/192.168.31.1-deep.json`
          : undefined,
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
    Cancel: async () => true,
    ListReports: async () => [...history],
    ResolveHostKey: () => {},
    OnProgress: (callback) => {
      progressListener = callback;
      return () => {
        progressListener = undefined;
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
