import { computed, reactive, toRefs } from "vue";
import type {
  AgentSetup,
  AgentStatus,
  RouterAPI,
  SkillReport,
  SkillRequest,
} from "../services/api";
import { useFieldErrors } from "./useFieldErrors";

/** Hosts whose skills directory is documented; the others export into a folder the user picks. */
export const SKILL_HOSTS = ["claude-code", "codex", "opencode", "pi", "dsh"];

/** WorkBuddy documents only stdio servers, so the desktop's Streamable HTTP endpoint has no setup for it. */
export const STDIO_ONLY_HOSTS = ["workbuddy"];

interface Draft {
  enabled: boolean;
  listen_host: string;
  /** TextInput emits strings; apply converts (CLAUDE.md: convert numeric fields like toPort). */
  port: string;
  allow_writes: boolean;
}

function message(caught: unknown): string {
  return caught instanceof Error ? caught.message : String(caught);
}

function draftOf(status: AgentStatus | null): Draft {
  const listener = status?.listener;
  return {
    enabled: listener?.enabled ?? false,
    listen_host: listener?.listen_host ?? "127.0.0.1",
    port: String(listener?.port ?? 18731),
    allow_writes: listener?.allow_writes ?? false,
  };
}

/** Empty when value is an integer port the agent service may listen on (1024-65535). */
export function agentPortError(value: string): string {
  const port = value.trim() === "" ? Number.NaN : Number(value);
  return Number.isInteger(port) && port >= 1024 && port <= 65535
    ? ""
    : "端口必须是 1024–65535 之间的整数";
}

/**
 * State of the 「Agent 接入」 page (DESIGN.md §4): the stored status from Go, a draft of the service form that
 * applies as a whole, a token shown once, the chosen host's setup, and skill installation.
 */
export function useAgents(api: RouterAPI) {
  const { errors, check } = useFieldErrors();
  const state = reactive({
    status: null as AgentStatus | null,
    loadError: "",
    draft: draftOf(null),
    applying: false,
    applyError: "",
    /** A token Go created; shown once and never stored by the window. */
    token: "",
    rotating: false,
    host: "claude-code",
    setup: null as AgentSetup | null,
    setupError: "",
    skillBusy: false,
    skillReport: null as SkillReport | null,
    skillError: "",
    pendingForce: null as SkillRequest | null,
    errors,
  });

  const stored = computed(() => draftOf(state.status));
  const dirty = computed(() => {
    const draft = state.draft;
    const saved = stored.value;
    return (
      draft.enabled !== saved.enabled ||
      draft.listen_host !== saved.listen_host ||
      draft.port.trim() !== saved.port ||
      draft.allow_writes !== saved.allow_writes
    );
  });

  function adopt(status: AgentStatus) {
    const keepDraft = dirty.value;
    state.status = status;
    if (!keepDraft) state.draft = draftOf(status);
  }

  async function load() {
    try {
      adopt(await api.AgentStatus());
      state.loadError = "";
    } catch (caught) {
      state.loadError = message(caught);
    }
  }

  function reset() {
    state.draft = draftOf(state.status);
    state.applyError = "";
    for (const key of Object.keys(errors)) delete errors[key];
  }

  async function apply() {
    const portMessage = agentPortError(state.draft.port);
    if (!check([["agent-port", !portMessage, portMessage]])) return;
    state.applying = true;
    state.applyError = "";
    try {
      const result = await api.AgentApply({
        enabled: state.draft.enabled,
        listen_host: state.draft.listen_host,
        port: Number(state.draft.port),
        allow_writes: state.draft.allow_writes,
      });
      if (result.token) state.token = result.token;
      state.draft = draftOf({ ...(state.status as AgentStatus), listener: result.status });
      await load();
    } catch (caught) {
      state.applyError = message(caught);
      await load();
    } finally {
      state.applying = false;
    }
  }

  async function rotate() {
    state.rotating = true;
    state.applyError = "";
    try {
      state.token = await api.AgentNewToken();
      await load();
    } catch (caught) {
      state.applyError = message(caught);
    } finally {
      state.rotating = false;
    }
  }

  function dismissToken() {
    state.token = "";
  }

  async function selectHost(id: string) {
    state.host = id;
    state.setup = null;
    state.setupError = "";
    state.skillReport = null;
    state.skillError = "";
    state.pendingForce = null;
    if (STDIO_ONLY_HOSTS.includes(id)) return;
    try {
      const setup = await api.AgentSetup(id);
      if (state.host === id) state.setup = setup;
    } catch (caught) {
      if (state.host === id) state.setupError = message(caught);
    }
  }

  async function install(request: SkillRequest) {
    state.skillBusy = true;
    state.skillError = "";
    state.skillReport = null;
    try {
      const report = await api.AgentInstallSkill(request);
      if (report.status === "differs" && !request.force) {
        state.pendingForce = { ...request, force: true };
      } else {
        state.pendingForce = null;
      }
      state.skillReport = report;
    } catch (caught) {
      state.skillError = message(caught);
    } finally {
      state.skillBusy = false;
    }
  }

  function installSkill() {
    return install({ agent: state.host, force: false });
  }

  function confirmForce() {
    const request = state.pendingForce;
    state.pendingForce = null;
    return request ? install(request) : Promise.resolve();
  }

  function cancelForce() {
    state.pendingForce = null;
  }

  async function exportSkill() {
    let dir = "";
    try {
      dir = await api.SelectDirectory();
    } catch (caught) {
      state.skillError = message(caught);
      return;
    }
    if (!dir) return;
    await install({ dir, force: false });
  }

  async function cancel(operationID: string) {
    try {
      await api.AgentCancel(operationID);
    } finally {
      await load();
    }
  }

  function subscribe(): () => void {
    return api.OnAgentStatus((status) => adopt(status));
  }

  return reactive({
    ...toRefs(state),
    dirty,
    load,
    reset,
    apply,
    rotate,
    dismissToken,
    selectHost,
    installSkill,
    confirmForce,
    cancelForce,
    exportSkill,
    cancel,
    subscribe,
  });
}

export type AgentsStore = ReturnType<typeof useAgents>;
