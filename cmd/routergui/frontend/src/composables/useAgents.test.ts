import { nextTick } from "vue";
import { useAgents, SKILL_HOSTS } from "./useAgents";
import type { AgentStatus, RouterAPI } from "../services/api";

function statusOf(overrides: Partial<AgentStatus["listener"]> = {}): AgentStatus {
  return {
    listener: {
      listening: false,
      enabled: false,
      listen_host: "127.0.0.1",
      port: 18731,
      allow_writes: false,
      token_set: false,
      ...overrides,
    },
    url: "http://127.0.0.1:18731/mcp",
    local_addresses: ["192.168.31.20"],
    operations: [],
    activity: [],
    agents: [],
  };
}

function makeAPI(status = statusOf()) {
  let listener: ((status: AgentStatus) => void) | undefined;
  const api = {
    AgentStatus: vi.fn().mockResolvedValue(status),
    AgentApply: vi.fn(),
    AgentNewToken: vi.fn(),
    AgentSetup: vi.fn().mockResolvedValue({
      agent: "claude-code",
      transport: "http",
      format: "json",
      snippet: "{}",
    }),
    AgentInstallSkill: vi.fn(),
    AgentCancel: vi.fn().mockResolvedValue(true),
    SelectDirectory: vi.fn().mockResolvedValue(""),
    OnAgentStatus: vi.fn((callback: (status: AgentStatus) => void) => {
      listener = callback;
      return () => (listener = undefined);
    }),
  };
  return {
    api: api as unknown as RouterAPI,
    raw: api,
    push: (next: AgentStatus) => listener?.(next),
  };
}

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0));
  await nextTick();
}

describe("useAgents", () => {
  it("loads the stored settings into the draft and tracks changes", async () => {
    const { api } = makeAPI(statusOf({ listen_host: "0.0.0.0", port: 18800, allow_writes: true }));
    const agents = useAgents(api);
    await agents.load();
    expect(agents.draft).toEqual({
      enabled: false,
      listen_host: "0.0.0.0",
      port: "18800",
      allow_writes: true,
    });
    expect(agents.dirty).toBe(false);
    agents.draft.enabled = true;
    expect(agents.dirty).toBe(true);
    agents.reset();
    expect(agents.dirty).toBe(false);
  });

  it("applies the whole form with an integer port and shows a created token once", async () => {
    const { api, raw } = makeAPI();
    raw.AgentApply.mockResolvedValue({
      status: statusOf({ listening: true, enabled: true, token_set: true }).listener,
      token: "rtmcp_secret",
    });
    raw.AgentStatus.mockResolvedValueOnce(statusOf()).mockResolvedValue(
      statusOf({ listening: true, enabled: true, token_set: true }),
    );
    const agents = useAgents(api);
    await agents.load();
    agents.draft.enabled = true;
    agents.draft.port = "18790";
    await agents.apply();
    expect(raw.AgentApply).toHaveBeenCalledWith({
      enabled: true,
      listen_host: "127.0.0.1",
      port: 18790,
      allow_writes: false,
    });
    expect(agents.token).toBe("rtmcp_secret");
    expect(agents.status?.listener.listening).toBe(true);
    agents.dismissToken();
    expect(agents.token).toBe("");
  });

  it("refuses a port outside 1024-65535 before calling Go", async () => {
    const { api, raw } = makeAPI();
    const agents = useAgents(api);
    await agents.load();
    for (const port of ["80", "70000", "abc", ""]) {
      agents.draft.port = port;
      await agents.apply();
      expect(agents.errors["agent-port"]).toBeTruthy();
    }
    expect(raw.AgentApply).not.toHaveBeenCalled();
  });

  it("keeps the real status and shows the reason when an apply is rejected", async () => {
    const { api, raw } = makeAPI(statusOf({ listening: true, enabled: true, token_set: true }));
    raw.AgentApply.mockRejectedValue(
      new Error("listen tcp4 127.0.0.1:18790: bind: address already in use"),
    );
    const agents = useAgents(api);
    await agents.load();
    agents.draft.port = "18790";
    await agents.apply();
    expect(agents.applyError).toContain("address already in use");
    expect(agents.status?.listener.port).toBe(18731);
    expect(agents.dirty).toBe(true);
    expect(agents.applying).toBe(false);
  });

  it("rotates the token and reveals the new one", async () => {
    const { api, raw } = makeAPI(statusOf({ token_set: true }));
    raw.AgentNewToken.mockResolvedValue("rtmcp_new");
    const agents = useAgents(api);
    await agents.load();
    await agents.rotate();
    expect(agents.token).toBe("rtmcp_new");
  });

  it("follows status events without overwriting unsaved edits", async () => {
    const { api, push } = makeAPI();
    const agents = useAgents(api);
    await agents.load();
    const stop = agents.subscribe();
    push(statusOf({ port: 18800 }));
    expect(agents.draft.port).toBe("18800");
    agents.draft.allow_writes = true;
    push(statusOf({ port: 18801 }));
    expect(agents.draft.port).toBe("18800");
    expect(agents.status?.listener.port).toBe(18801);
    stop();
  });

  it("fetches the setup of the chosen host and explains WorkBuddy instead", async () => {
    const { api, raw } = makeAPI();
    const agents = useAgents(api);
    await agents.load();
    await agents.selectHost("codex");
    expect(raw.AgentSetup).toHaveBeenLastCalledWith("codex");
    expect(agents.setup?.snippet).toBe("{}");
    raw.AgentSetup.mockClear();
    await agents.selectHost("workbuddy");
    expect(raw.AgentSetup).not.toHaveBeenCalled();
    expect(agents.setup).toBeNull();
    expect(agents.host).toBe("workbuddy");
  });

  it("ignores a setup that arrives after another host was chosen", async () => {
    const { api, raw } = makeAPI();
    const agents = useAgents(api);
    await agents.load();
    let answerClaude!: (setup: unknown) => void;
    raw.AgentSetup.mockImplementationOnce(() => new Promise((resolve) => (answerClaude = resolve)));
    raw.AgentSetup.mockImplementationOnce(async () => ({
      agent: "codex",
      transport: "http",
      format: "toml",
      snippet: "codex",
    }));
    const slow = agents.selectHost("claude-code");
    await agents.selectHost("codex");
    answerClaude({ agent: "claude-code", transport: "http", format: "json", snippet: "claude" });
    await slow;
    expect(agents.host).toBe("codex");
    expect(agents.setup?.snippet).toBe("codex");
  });

  it("installs the skill, asks before replacing a changed copy, and exports for hosts without a directory", async () => {
    const { api, raw } = makeAPI();
    const agents = useAgents(api);
    await agents.load();
    expect(SKILL_HOSTS).toEqual(["claude-code", "codex", "opencode", "pi", "dsh"]);
    raw.AgentInstallSkill.mockResolvedValueOnce({
      status: "differs",
      path: "/home/u/.claude/skills/router-tools",
      message: "differs",
    });
    await agents.installSkill();
    expect(raw.AgentInstallSkill).toHaveBeenLastCalledWith({ agent: "claude-code", force: false });
    expect(agents.pendingForce).toEqual({ agent: "claude-code", force: true });
    raw.AgentInstallSkill.mockResolvedValueOnce({
      status: "updated",
      path: "/home/u/.claude/skills/router-tools",
    });
    await agents.confirmForce();
    expect(raw.AgentInstallSkill).toHaveBeenLastCalledWith({ agent: "claude-code", force: true });
    expect(agents.skillReport?.status).toBe("updated");
    expect(agents.pendingForce).toBeNull();

    await agents.selectHost("generic");
    raw.SelectDirectory.mockResolvedValueOnce("");
    await agents.exportSkill();
    expect(raw.AgentInstallSkill).toHaveBeenCalledTimes(2);
    raw.SelectDirectory.mockResolvedValueOnce("/home/u/exports");
    raw.AgentInstallSkill.mockResolvedValueOnce({
      status: "installed",
      path: "/home/u/exports/router-tools",
    });
    await agents.exportSkill();
    expect(raw.AgentInstallSkill).toHaveBeenLastCalledWith({
      dir: "/home/u/exports",
      force: false,
    });
    expect(agents.skillReport?.path).toBe("/home/u/exports/router-tools");
  });

  it("cancels an agent operation and refreshes", async () => {
    const { api, raw } = makeAPI();
    const agents = useAgents(api);
    await agents.load();
    await agents.cancel("agent-1");
    await flush();
    expect(raw.AgentCancel).toHaveBeenCalledWith("agent-1");
    expect(raw.AgentStatus).toHaveBeenCalledTimes(2);
  });
});
