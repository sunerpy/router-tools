import { mount } from "@vue/test-utils";
import { nextTick } from "vue";
import CodeBlock from "./CodeBlock.vue";

describe("CodeBlock", () => {
  it("shows the label and the full content and copies it", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    const content = `[mcp_servers.router-tools]\nurl = "http://127.0.0.1:18731/mcp"\n${"x".repeat(120)}`;
    const wrapper = mount(CodeBlock, { props: { label: "配置片段", content } });
    expect(wrapper.text()).toContain("配置片段");
    const pre = wrapper.get("pre");
    expect(pre.text()).toBe(content);
    expect(pre.attributes("tabindex")).toBe("0");
    const button = wrapper.get("button");
    expect(button.attributes("aria-label")).toBe("复制配置片段");
    await button.trigger("click");
    await nextTick();
    expect(writeText).toHaveBeenCalledWith(content);
    expect(wrapper.get("button").attributes("aria-label")).toBe("已复制配置片段");
    expect(wrapper.get("button").text()).toBe("已复制");
    expect(wrapper.get('[aria-live="polite"]').text()).toBe("已复制");
    await wrapper.setProps({ content: "other" });
    expect(wrapper.get("button").attributes("aria-label")).toBe("复制配置片段");
  });

  it("says so when the clipboard refuses", async () => {
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: vi.fn().mockRejectedValue(new Error("denied")) },
      configurable: true,
    });
    const wrapper = mount(CodeBlock, { props: { label: "命令", content: "codex mcp add" } });
    await wrapper.get("button").trigger("click");
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(wrapper.get('[aria-live="polite"]').text()).toBe("无法复制，请手动选择文本");
  });
});

describe("CodeBlock names", () => {
  it("puts a space between Chinese and a Latin label", () => {
    const wrapper = mount(CodeBlock, {
      props: { label: "Agent 连接地址", content: "http://127.0.0.1:18731/mcp" },
    });
    expect(wrapper.get("button").attributes("aria-label")).toBe("复制 Agent 连接地址");
  });
});
