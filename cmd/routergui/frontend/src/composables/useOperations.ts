import { computed, reactive, ref, watch } from "vue";
import type {
  HistoryEntry,
  HostKeyPrompt,
  OperationName,
  OperationRequest,
  RouterAPI,
  Target,
} from "../services/api";

const defaultTarget: Target = { host: "192.168.31.1", web_port: 80, ssh_port: 22, telnet_port: 23 };
const preferenceKey = "router.preferences";

function loadPreferences() {
  try {
    const saved = JSON.parse(localStorage.getItem(preferenceKey) || "{}");
    return {
      target: { ...defaultTarget, ...saved.target },
      profileID: saved.profileID || "auto",
      hostKey: { KnownHostsPath: "", PinSHA256: "", AcceptNew: false, ...saved.hostKey },
    };
  } catch {
    return {
      target: { ...defaultTarget },
      profileID: "auto",
      hostKey: { KnownHostsPath: "", PinSHA256: "", AcceptNew: false },
    };
  }
}

// Number inputs emit strings and saved preferences may hold them, but the Go
// binding only decodes integers that fit its int type; anything else fails to
// decode before the application runs. A value that is not a TCP port becomes 0,
// which the application rejects as an invalid argument before network access.
export function toPort(value: unknown): number {
  const port = typeof value === "string" && value.trim() === "" ? Number.NaN : Number(value);
  return Number.isInteger(port) && port >= 1 && port <= 65535 ? port : 0;
}

export function newOperationID(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function")
    return crypto.randomUUID();
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

export function useOperations(api: RouterAPI) {
  const preferences = loadPreferences();
  const target = reactive<Target>(preferences.target);
  const profileID = ref(preferences.profileID);
  const hostKey = reactive(preferences.hostKey);
  const adminPassword = ref("");
  const rootPassword = ref("");
  const transport = ref("ssh");
  const allowTelnet = ref(false);
  const confirmed = ref(false);
  const withAdminAuth = ref(false);
  const withRootVerify = ref(false);
  const outputDir = ref("");
  const backupDir = ref("");
  const runDir = ref("");
  const ackDeep = ref("");
  const operationID = ref("");
  const operation = ref<OperationName | "">("");
  const status = ref("idle");
  const events = ref<Array<{ stage: string; status: string }>>([]);
  const report = ref<any>(null);
  const error = ref("");
  const history = ref<HistoryEntry[]>([]);
  // The target each run was sent to, so summaries never label old evidence with an edited form.
  const targets = new Map<string, Target>();
  const hostKeyPrompt = ref<HostKeyPrompt | null>(null);
  const busy = computed(() => status.value === "running" || status.value === "canceling");

  watch(
    [target, profileID, hostKey],
    () => {
      try {
        localStorage.setItem(
          preferenceKey,
          JSON.stringify({ target, profileID: profileID.value, hostKey }),
        );
      } catch {
        // Storage is optional; private browsing or embedded hosts may disable it.
      }
    },
    { deep: true },
  );

  api.OnProgress((payload) => {
    if (payload.operation_id !== operationID.value) return;
    events.value.push({ stage: payload.event.stage, status: payload.event.status });
    status.value = payload.event.status;
  });
  api.OnHostKey((prompt) => {
    hostKeyPrompt.value = prompt;
  });

  function clearSecrets() {
    adminPassword.value = "";
    rootPassword.value = "";
    ackDeep.value = "";
    confirmed.value = false;
  }

  function makeRequest(): OperationRequest {
    return {
      OperationID: operationID.value,
      Target: {
        host: target.host,
        web_port: toPort(target.web_port),
        ssh_port: toPort(target.ssh_port),
        telnet_port: toPort(target.telnet_port),
      },
      ProfileID: profileID.value,
      AdminPassword: adminPassword.value,
      RootPassword: rootPassword.value,
      Transport: transport.value,
      AllowTelnet: allowTelnet.value,
      Confirmed: confirmed.value,
      WithAdminAuth: withAdminAuth.value,
      WithRootVerify: withRootVerify.value,
      OutputDir: outputDir.value,
      BackupDir: backupDir.value,
      RunDir: runDir.value,
      AckDeep: ackDeep.value,
      ToolVersion: "dev",
      UseSavedPassword: false,
      RememberPassword: false,
      HostKey: { ...hostKey },
    };
  }

  async function refreshHistory() {
    history.value = await api.ListReports();
  }

  async function refreshHistoryQuietly() {
    try {
      await refreshHistory();
    } catch {
      // History is best effort; the Reports view offers a manual refresh.
    }
  }

  /** Runs an operation. `overrides` replace request fields for this run only (for example the
   * deep flow's own Telnet acknowledgement) without touching the shared form state. */
  async function run(name: OperationName, overrides: Partial<OperationRequest> = {}) {
    if (busy.value) return;
    operationID.value = newOperationID();
    operation.value = name;
    status.value = "running";
    events.value = [];
    report.value = null;
    error.value = "";
    const request = { ...makeRequest(), ...overrides };
    targets.set(request.OperationID, { ...request.Target });
    let pending: Promise<any>;
    try {
      pending = api[name](request);
    } catch (caught) {
      clearSecrets();
      throw caught;
    }
    clearSecrets();
    try {
      const result = await pending;
      report.value = result;
      // The bindings resolve failed, partial, and unknown reports instead of rejecting.
      // A canceled run carries the CANCELED code. Only a failed one is shown as canceled:
      // partial and unknown mean a write may have happened, so they keep their status
      // and the "do not retry from the beginning" warning.
      const canceled =
        result?.error?.code === "CANCELED" && (!result?.status || result.status === "failed");
      status.value = canceled
        ? "canceled"
        : result?.status || (result?.error ? "failed" : "succeeded");
      if (result?.error) error.value = result.error.message || result.error.code || "操作失败";
      await refreshHistoryQuietly();
    } catch (caught) {
      const message = caught instanceof Error ? caught.message : String(caught);
      error.value = message;
      status.value = message.toLowerCase().includes("cancel") ? "canceled" : "failed";
      // A rejected call can still leave a report in history (deep persistence whose final
      // journal write failed), so the history is refreshed here too.
      await refreshHistoryQuietly();
    }
  }

  function targetFor(id: string): Target | undefined {
    return targets.get(id);
  }

  async function cancel() {
    if (!operationID.value) return;
    status.value = "canceling";
    await api.Cancel(operationID.value);
    clearSecrets();
  }

  async function resolveHostKey(accept: boolean) {
    if (!hostKeyPrompt.value) return;
    api.ResolveHostKey(hostKeyPrompt.value.prompt_id, accept);
    hostKeyPrompt.value = null;
  }

  return reactive({
    target,
    profileID,
    hostKey,
    adminPassword,
    rootPassword,
    transport,
    allowTelnet,
    confirmed,
    withAdminAuth,
    withRootVerify,
    outputDir,
    backupDir,
    runDir,
    ackDeep,
    operationID,
    operation,
    status,
    events,
    report,
    error,
    history,
    hostKeyPrompt,
    busy,
    run,
    cancel,
    resolveHostKey,
    refreshHistory,
    clearSecrets,
    targetFor,
  });
}
