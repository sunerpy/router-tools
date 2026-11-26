import { mount } from "@vue/test-utils";
import { reactive } from "vue";
import { NCard, NInput, NModal } from "naive-ui";
import HostKeyDialog from "./HostKeyDialog.vue";

const prompt = {
  operation_id: "agent-7",
  address: "192.168.31.1:22",
  algorithm: "ssh-ed25519",
  fingerprint: "SHA256:abc",
};

async function open(source?: string, waiting = 0) {
  const store = reactive({
    hostKeyPrompt: null as Record<string, string> | null,
    hostKeyWaiting: waiting,
    resolveHostKey: vi.fn(),
  });
  const wrapper = mount(HostKeyDialog, {
    global: { components: { NModal, NCard, NInput }, provide: { operations: store } },
    attachTo: document.body,
  });
  store.hostKeyPrompt = source ? { ...prompt, source } : { ...prompt };
  await new Promise((resolve) => setTimeout(resolve, 60));
  const dialog = document.body.querySelector<HTMLElement>(".confirm-dialog")!;
  return { wrapper, dialog, store };
}

describe("HostKeyDialog", () => {
  afterEach(() => {
    document.body.innerHTML = "";
  });

  it("names an operation an agent started", async () => {
    const { wrapper, dialog } = await open("agent");
    expect(dialog.textContent).toContain("Agent 操作遇到未知的 SSH 主机密钥");
    expect(dialog.textContent).toContain("来源：Agent 发起的操作 agent-7");
    expect(dialog.textContent).toContain("SHA256:abc");
    wrapper.unmount();
  });

  it("keeps the plain title for the window's own operations", async () => {
    const { wrapper, dialog } = await open();
    expect(dialog.textContent).toContain("未知 SSH 主机密钥");
    expect(dialog.textContent).not.toContain("Agent");
    wrapper.unmount();
  });

  it("says how many more host keys wait for a decision", async () => {
    const single = await open("agent");
    expect(single.dialog.textContent).not.toContain("等待确认");
    single.wrapper.unmount();
    document.body.innerHTML = "";
    const queued = await open("agent", 2);
    expect(queued.dialog.textContent).toContain("另有 2 个主机密钥等待确认");
    queued.wrapper.unmount();
  });

  it("leaves acceptance to the user", async () => {
    const { wrapper, dialog, store } = await open("agent");
    const accept = [...dialog.querySelectorAll("button")].find((button) =>
      button.textContent?.includes("接受并保存此密钥"),
    )!;
    expect(document.activeElement).not.toBe(accept);
    accept.click();
    expect(store.resolveHostKey).toHaveBeenCalledWith(true);
    wrapper.unmount();
  });
});
