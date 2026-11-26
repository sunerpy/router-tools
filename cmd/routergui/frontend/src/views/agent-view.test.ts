import { flushPromises, mount } from "@vue/test-utils";
import { NCard, NInput, NModal } from "naive-ui";
import AgentView from "./AgentView.vue";
import { useAgents } from "../composables/useAgents";
import type { AgentStatus, RouterAPI } from "../services/api";

const hosts = [
  { id: "claude-code", name: "Claude Code", http: true, skill_dir: ".claude/skills" },
  { id: "codex", name: "Codex CLI", http: true, skill_dir: ".agents/skills" },
  { id: "workbuddy", name: "WorkBuddy", http: false },
  { id: "generic", name: "Other MCP clients", http: true },
];

function statusOf(
  listener: Partial<AgentStatus["listener"]> = {},
  extra: Partial<AgentStatus> = {},
): AgentStatus {
  return {
    listener: {
      listening: false,
      enabled: false,
      listen_host: "127.0.0.1",
      port: 18731,
      allow_writes: false,
      token_set: false,
      ...listener,
    },
    url: "http://127.0.0.1:18731/mcp",
    local_addresses: ["192.168.31.20"],
    operations: [],
    activity: [],
    agents: hosts,
    skill_paths: {
      "claude-code": "/srv/claude-config/skills/router-tools",
      codex: "/home/u/.agents/skills/router-tools",
    },
    ...extra,
  };
}

function setup(status = statusOf()) {
  const api = {
    AgentStatus: vi.fn().mockResolvedValue(status),
    AgentApply: vi.fn(),
    AgentNewToken: vi.fn().mockResolvedValue("rtmcp_rotated"),
    AgentSetup: vi.fn().mockResolvedValue({
      agent: "claude-code",
      transport: "http",
      format: "json",
      file: "~/.claude.json",
      snippet: '{"mcpServers":{}}',
      notes: ["Set ROUTERCTL_MCP_TOKEN."],
    }),
    AgentInstallSkill: vi
      .fn()
      .mockResolvedValue({ status: "installed", path: "/home/u/.claude/skills/router-tools" }),
    AgentCancel: vi.fn().mockResolvedValue(true),
    SelectDirectory: vi.fn().mockResolvedValue(""),
    OnAgentStatus: vi.fn().mockReturnValue(() => {}),
  } as unknown as RouterAPI;
  const agents = useAgents(api);
  const wrapper = mount(AgentView, {
    global: { provide: { agents }, components: { NModal, NCard, NInput } },
    attachTo: document.body,
  });
  return { api: api as any, agents, wrapper };
}

describe("AgentView", () => {
  afterEach(() => {
    document.body.innerHTML = "";
  });

  it("says running agent operations continue when the service is turned off", async () => {
    const running = {
      operation_id: "agent-1",
      tool: "guide_apply",
      status: "running",
      done: false,
      started: "2026-11-27T09:59:00Z",
    };
    const notice = "正在运行的 Agent 操作会继续";
    const listening = {
      listening: true,
      enabled: true,
      address: "127.0.0.1:18731",
      token_set: true,
    };
    const idle = setup(
      statusOf(listening, { operations: [{ ...running, done: true, status: "succeeded" }] }),
    );
    await flushPromises();
    await idle.wrapper.get("#agent-enabled").setValue(false);
    expect(idle.wrapper.text()).not.toContain(notice);
    idle.wrapper.unmount();

    const busy = setup(statusOf(listening, { operations: [running] }));
    await flushPromises();
    expect(busy.wrapper.text()).not.toContain(notice);
    await busy.wrapper.get("#agent-enabled").setValue(false);
    expect(busy.wrapper.text()).toContain(notice);
    busy.wrapper.unmount();

    const stopped = setup(statusOf({ token_set: true }, { operations: [running] }));
    await flushPromises();
    expect(stopped.wrapper.text()).toContain(notice);
  });

  it("reports what actually listens and warns about LAN addresses and writes in the draft", async () => {
    const { wrapper } = setup(
      statusOf({ listening: true, enabled: true, address: "127.0.0.1:18731", token_set: true }),
    );
    await flushPromises();
    expect(wrapper.get('[role="status"]').text()).toContain("正在监听 127.0.0.1:18731 · 只读");
    expect(wrapper.text()).not.toContain("明文：令牌和密码会经过网络");
    await wrapper.get("#agent-host").setValue("0.0.0.0");
    expect(wrapper.text()).toContain("明文：令牌和密码会经过网络");
    await wrapper.get("#agent-writes").setValue(true);
    expect(wrapper.text()).toContain("Agent 可以修改路由器");
    // The status line keeps reporting the listener, not the edited form.
    expect(wrapper.get('[role="status"]').text()).toContain("只读");
    const options = wrapper
      .findAll("#agent-host option")
      .map((option) => option.attributes("value"));
    expect(options).toEqual(["127.0.0.1", "0.0.0.0", "192.168.31.20"]);
  });

  it("shows a failed apply's error once, not again as the last error", async () => {
    const message =
      "端口已被其他程序占用：换一个端口，或先关闭占用它的程序（listen tcp4 127.0.0.1:18731: bind: address already in use）";
    const { wrapper, api } = setup();
    await flushPromises();
    api.AgentApply.mockRejectedValue(new Error(message));
    api.AgentStatus.mockResolvedValue(statusOf({ last_error: message }));
    await wrapper.get("#agent-enabled").setValue(true);
    await wrapper
      .findAll("button")
      .find((button) => button.text() === "应用更改")!
      .trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("没有应用更改");
    expect(wrapper.text().split("已被其他程序占用").length - 1).toBe(1);
    expect(wrapper.text()).not.toContain("上次的设置没有生效");
  });

  it("shows a last error from launch when no apply failed", async () => {
    const { wrapper } = setup(
      statusOf({
        enabled: true,
        last_error: "listen tcp4 192.0.2.10:18731: bind: cannot assign requested address",
      }),
    );
    await flushPromises();
    expect(wrapper.text()).toContain("上次的设置没有生效");
    expect(wrapper.text()).toContain("cannot assign requested address");
  });

  it("disables apply and undo while nothing changed", async () => {
    const { wrapper } = setup();
    await flushPromises();
    const buttons = () =>
      wrapper
        .findAll("button")
        .filter((button) => ["应用更改", "撤销更改"].includes(button.text()));
    expect(buttons().every((button) => button.attributes("disabled") !== undefined)).toBe(true);
    await wrapper.get("#agent-enabled").setValue(true);
    expect(buttons().every((button) => button.attributes("disabled") === undefined)).toBe(true);
  });

  it("shows a new token once and forgets it when saved or when the page closes", async () => {
    const { wrapper, agents } = setup(statusOf({ token_set: false }));
    await flushPromises();
    const rotate = wrapper
      .findAll("button")
      .find((button) => button.text().includes("生成新令牌"))!;
    await rotate.trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("rtmcp_rotated");
    expect(wrapper.text()).toContain("只显示这一次");
    await wrapper
      .findAll("button")
      .find((button) => button.text() === "我已保存")!
      .trigger("click");
    expect(wrapper.text()).not.toContain("rtmcp_rotated");
    agents.token = "rtmcp_again";
    await flushPromises();
    expect(wrapper.text()).toContain("rtmcp_again");
    wrapper.unmount();
    expect(agents.token).toBe("");
  });

  it("asks before replacing an existing token", async () => {
    const { wrapper, api } = setup(
      statusOf({ token_set: true, token_created_at: "2026-11-27T08:00:00Z" }),
    );
    await flushPromises();
    await wrapper
      .findAll("button")
      .find((button) => button.text().includes("生成新令牌"))!
      .trigger("click");
    await flushPromises();
    expect(api.AgentNewToken).not.toHaveBeenCalled();
    expect(document.body.textContent).toContain("旧令牌立即失效");
  });

  it("shows the chosen host's setup, explains WorkBuddy, and offers install or export", async () => {
    const { wrapper, api } = setup();
    await flushPromises();
    expect(api.AgentSetup).toHaveBeenCalledWith("claude-code");
    expect(wrapper.text()).toContain("~/.claude.json");
    // Go resolves the target (here CLAUDE_CONFIG_DIR); the page never guesses it.
    expect(wrapper.text()).toContain("安装到 /srv/claude-config/skills/router-tools");
    await wrapper
      .findAll("button")
      .find((button) => button.text().includes("安装 Skill"))!
      .trigger("click");
    await flushPromises();
    expect(api.AgentInstallSkill).toHaveBeenCalledWith({ agent: "claude-code", force: false });
    expect(wrapper.text()).toContain("Skill 已安装");

    const radios = wrapper.findAll('input[type="radio"]');
    await radios[2].setValue(true);
    await flushPromises();
    expect(wrapper.text()).toContain("WorkBuddy 只支持 stdio");
    expect(wrapper.text()).toContain("routerctl mcp config --agent workbuddy");
    expect(wrapper.findAll("button").some((button) => button.text().includes("导出到文件夹"))).toBe(
      true,
    );
    expect(wrapper.findAll("button").some((button) => button.text().includes("安装 Skill"))).toBe(
      false,
    );
  });

  it("lists agent operations and asks before canceling a running one", async () => {
    const { wrapper, api } = setup(
      statusOf(
        {},
        {
          operations: [
            {
              operation_id: "agent-1",
              tool: "guide_apply",
              host: "192.168.31.1",
              status: "running",
              done: false,
              started: "2026-11-27T09:59:00",
            },
            {
              operation_id: "agent-0",
              tool: "backup_create",
              status: "succeeded",
              done: true,
              started: "2026-11-27T09:50:00",
            },
          ],
          activity: [
            {
              time: "2026-11-27T09:59:00",
              kind: "call",
              tool: "guide_apply",
              status: "running",
              duration_ms: 20000,
              done: false,
            },
          ],
        },
      ),
    );
    await flushPromises();
    expect(wrapper.text()).toContain("Agent 操作 · 2");
    expect(wrapper.text()).toContain("执行引导计划 · 调用");
    const cancels = wrapper.findAll("button").filter((button) => button.text() === "取消");
    expect(cancels).toHaveLength(1);
    await cancels[0].trigger("click");
    await flushPromises();
    expect(api.AgentCancel).not.toHaveBeenCalled();
    expect(document.body.textContent).toContain("取消 Agent 操作");
  });
});
