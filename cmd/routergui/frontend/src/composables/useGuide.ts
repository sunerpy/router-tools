import { computed, reactive, ref } from "vue";
import type {
  CredentialEntry,
  GuideApplyReport,
  GuideGoal,
  GuidePlanReport,
  GuideRequest,
  OperationRequest,
  RouterAPI,
  Target,
} from "../services/api";
import { newOperationID, toPort } from "./useOperations";

type OperationsStore = {
  target: Target;
  profileID: string;
  hostKey: { KnownHostsPath: string; PinSHA256: string; AcceptNew: boolean };
  refreshHistory: () => Promise<void>;
};

export type SavedState = "saved" | "rejected" | "none";

/**
 * The guided flow's state: detect (read-only plan), then one confirmed run. Every password is cleared
 * as soon as the call that uses it is dispatched (detection sends only the current root password),
 * when a confirmation or the run is canceled, and when the page is left; none is stored. The plan's
 * confirmation is sent back unchanged, and a plan no longer runs once its inputs were edited.
 */
export function canonicalHost(host: string): string {
  // Mirrors device.CanonicalHost for names and IPv4 literals: lower case without a trailing dot.
  return host.trim().toLowerCase().replace(/\.$/, "");
}

export function useGuide(api: RouterAPI, operations: OperationsStore) {
  const goals = reactive({ backup: true, soft: true, reboot: false });
  const useSaved = ref(true);
  const remember = ref(false);
  const resetRoot = ref(false);
  const adminPassword = ref("");
  const rootPassword = ref("");
  const newRootPassword = ref("");
  const newRootRepeat = ref("");
  const plan = ref<GuidePlanReport | null>(null);
  const run = ref<GuideApplyReport | null>(null);
  const phase = ref<"idle" | "detecting" | "planned" | "running" | "done">("idle");
  const error = ref("");
  const planChanged = ref(false);
  const ackError = ref("");
  // What the current plan was detected with; editing any of it makes the plan stale.
  const detectedWith = ref("");
  const operationID = ref("");
  const stepEvents = reactive<Record<string, Array<{ stage: string; status: string }>>>({});
  const credentials = ref<CredentialEntry[]>([]);
  const busy = computed(() => phase.value === "detecting" || phase.value === "running");

  const goalList = computed<GuideGoal[]>(() => {
    const list: GuideGoal[] = ["ssh"];
    if (goals.backup) list.push("backup");
    if (goals.soft) {
      list.push("soft");
      if (goals.reboot) list.push("reboot");
    }
    return list;
  });

  const needs = computed(() => {
    const inputs = plan.value?.required_inputs ?? [];
    return {
      admin: inputs.includes("admin_password"),
      currentRoot: inputs.includes("root_password"),
      newRoot: inputs.includes("new_root_password"),
    };
  });

  const enableRuns = computed(() =>
    Boolean(plan.value?.steps.some((step) => step.id === "enable_ssh" && step.action === "run")),
  );

  api.OnProgress((payload) => {
    if (payload.operation_id !== operationID.value) return;
    const step = payload.event.step || "detect";
    (stepEvents[step] ||= []).push({ stage: payload.event.stage, status: payload.event.status });
  });

  function target(): Target {
    return {
      host: operations.target.host,
      web_port: toPort(operations.target.web_port),
      ssh_port: toPort(operations.target.ssh_port),
      telnet_port: toPort(operations.target.telnet_port),
    };
  }

  function guideRequest(): GuideRequest {
    return {
      OperationID: operationID.value,
      Target: target(),
      ProfileID: operations.profileID,
      Goals: goalList.value,
      AdminPassword: "",
      RootPassword: "",
      ResetRootPassword: resetRoot.value,
      Confirmation: null,
      PlanDigest: "",
      Confirmed: false,
      ToolVersion: "dev",
      HostKey: { ...operations.hostKey },
      UseSavedPassword: useSaved.value,
      RememberPassword: remember.value,
    };
  }

  function inputsKey(): string {
    const value = target();
    return JSON.stringify([
      canonicalHost(value.host),
      value.web_port,
      value.ssh_port,
      value.telnet_port,
      operations.profileID,
      goalList.value,
      useSaved.value,
    ]);
  }
  const stale = computed(() => plan.value !== null && detectedWith.value !== inputsKey());

  function resetEvents() {
    for (const key of Object.keys(stepEvents)) delete stepEvents[key];
  }

  function clearSecrets() {
    adminPassword.value = "";
    rootPassword.value = "";
    newRootPassword.value = "";
    newRootRepeat.value = "";
  }

  /** DESIGN.md §4: each visit starts with saved passwords allowed and remembering off. */
  function resetChoices() {
    useSaved.value = true;
    remember.value = false;
  }

  async function loadCredentials() {
    try {
      credentials.value = (await api.CredentialStatus()).entries ?? [];
    } catch {
      credentials.value = [];
    }
  }

  function savedState(kind: "web-admin" | "root"): SavedState {
    const host = canonicalHost(operations.target.host);
    const port = toPort(kind === "root" ? operations.target.ssh_port : operations.target.web_port);
    const entry = credentials.value.find(
      (candidate) => candidate.kind === kind && candidate.host === host && candidate.port === port,
    );
    if (!entry) return "none";
    return entry.rejected_at ? "rejected" : "saved";
  }

  async function detect() {
    if (busy.value) return;
    phase.value = "detecting";
    error.value = "";
    planChanged.value = false;
    run.value = null;
    operationID.value = newOperationID();
    resetEvents();
    const request = { ...guideRequest(), RootPassword: rootPassword.value };
    detectedWith.value = inputsKey();
    let pending: Promise<GuidePlanReport>;
    try {
      pending = api.GuidePlan(request);
    } finally {
      rootPassword.value = "";
    }
    try {
      plan.value = await pending;
    } catch (caught) {
      plan.value = null;
      error.value = caught instanceof Error ? caught.message : String(caught);
    }
    phase.value = plan.value ? "planned" : "idle";
    void loadCredentials();
  }

  /** Field errors for the inputs the plan still needs; empty when the run may start. */
  function validate(): Record<string, string> {
    const errors: Record<string, string> = {};
    if (needs.value.admin && !adminPassword.value) errors["guide-admin"] = "请输入 Web 管理密码";
    if (needs.value.newRoot) {
      if (!newRootPassword.value) errors["guide-new-root"] = "请输入新的 root 密码";
      else if (newRootRepeat.value !== newRootPassword.value)
        errors["guide-new-root-repeat"] = "两次输入的新 root 密码不一致";
    }
    if (needs.value.currentRoot && !rootPassword.value)
      errors["guide-root"] = "请输入当前 root 密码";
    return errors;
  }

  async function apply() {
    const current = plan.value;
    if (busy.value || !current?.confirmation || !current.ready || stale.value) return;
    if (Object.keys(validate()).length > 0) return;
    phase.value = "running";
    error.value = "";
    run.value = null;
    operationID.value = newOperationID();
    resetEvents();
    const request: GuideRequest = {
      ...guideRequest(),
      Confirmation: current.confirmation,
      PlanDigest: current.plan_digest ?? "",
      Confirmed: true,
      AdminPassword: enableRuns.value ? adminPassword.value : "",
      RootPassword: enableRuns.value ? newRootPassword.value : rootPassword.value,
    };
    let pending: Promise<GuideApplyReport>;
    try {
      pending = api.GuideApply(request);
    } finally {
      clearSecrets();
      resetRoot.value = false;
    }
    try {
      run.value = await pending;
      if (run.value?.error) error.value = run.value.error.message || run.value.error.code;
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : String(caught);
    }
    phase.value = "done";
    await operations.refreshHistory().catch(() => {});
    void loadCredentials();
    const wrote = (run.value?.steps ?? []).some((step) =>
      ["succeeded", "partial", "unknown"].includes(step.status),
    );
    if (
      run.value?.error?.code === "CONFIRMATION_REQUIRED" &&
      run.value.status === "failed" &&
      !wrote
    ) {
      // The router changed before anything was written: show what it is now; never retry. A run that
      // already changed the router keeps its result and its do-not-start-over warning instead.
      await detect();
      planChanged.value = true;
    }
  }

  async function acknowledge() {
    ackError.value = "";
    // The record belongs to the router the blocked plan was detected for, not to an edited form.
    const request: OperationRequest = {
      OperationID: newOperationID(),
      Target: { ...target(), host: plan.value?.host || operations.target.host },
      ProfileID: operations.profileID,
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
      HostKey: { ...operations.hostKey },
      UseSavedPassword: false,
      RememberPassword: false,
    };
    let report;
    try {
      report = await api.AcknowledgeRecovery(request);
    } catch (caught) {
      ackError.value = caught instanceof Error ? caught.message : String(caught);
      return null;
    }
    await operations.refreshHistory().catch(() => {});
    if (report?.status === "succeeded") await detect();
    else
      ackError.value = report?.error
        ? `${report.error.code}：${report.error.message}`
        : "确认没有完成";
    return report;
  }

  async function cancel() {
    if (operationID.value && busy.value) await api.Cancel(operationID.value);
    clearSecrets();
  }

  return reactive({
    goals,
    useSaved,
    remember,
    resetRoot,
    adminPassword,
    rootPassword,
    newRootPassword,
    newRootRepeat,
    plan,
    run,
    phase,
    error,
    planChanged,
    ackError,
    stale,
    operationID,
    stepEvents,
    credentials,
    busy,
    goalList,
    needs,
    enableRuns,
    detect,
    validate,
    apply,
    acknowledge,
    cancel,
    clearSecrets,
    resetChoices,
    loadCredentials,
    savedState,
  });
}
