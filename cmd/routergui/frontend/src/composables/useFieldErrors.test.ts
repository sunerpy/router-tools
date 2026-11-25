import { useFieldErrors } from "./useFieldErrors";

describe("useFieldErrors", () => {
  it("records the failed rules, focuses the first invalid field and clears on success", () => {
    const first = document.createElement("input");
    first.id = "field-a";
    document.body.append(first);
    const { errors, check } = useFieldErrors();

    expect(
      check([
        ["field-a", false, "请输入地址"],
        ["field-b", true, "unused"],
        ["field-c", false, "请输入密码"],
      ]),
    ).toBe(false);
    expect({ ...errors }).toEqual({ "field-a": "请输入地址", "field-c": "请输入密码" });
    expect(document.activeElement).toBe(first);

    expect(check([["field-a", true, "请输入地址"]])).toBe(true);
    expect({ ...errors }).toEqual({});
    first.remove();
  });
});

describe("portError", () => {
  it("accepts integer TCP ports only", async () => {
    const { portError } = await import("./useFieldErrors");
    expect(portError(80)).toBe("");
    expect(portError("443")).toBe("");
    expect(portError("")).not.toBe("");
    expect(portError("0")).not.toBe("");
    expect(portError("65536")).not.toBe("");
    expect(portError("1.5")).not.toBe("");
  });
});

describe("hostError", () => {
  it("accepts IP addresses and host names only", async () => {
    const { hostError } = await import("./useFieldErrors");
    expect(hostError("192.168.31.1")).toBe("");
    expect(hostError("router.local")).toBe("");
    expect(hostError("")).not.toBe("");
    expect(hostError(" 192.168.31.1")).not.toBe("");
    expect(hostError("http://192.168.31.1")).not.toBe("");
    expect(hostError("192.168.31.1/admin")).not.toBe("");
  });
});
