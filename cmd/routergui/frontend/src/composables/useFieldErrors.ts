import { reactive } from "vue";

/** [field id, passes, message shown under the field when it fails] */
export type FieldRule = [id: string, ok: boolean, message: string];

/**
 * Per-form validation before calling Go (DESIGN.md §4): failed rules become field errors that
 * the inputs link through aria-describedby, and focus moves to the first invalid field.
 */
export function useFieldErrors() {
  const errors = reactive<Record<string, string>>({});

  function check(rules: FieldRule[]): boolean {
    for (const key of Object.keys(errors)) delete errors[key];
    for (const [id, ok, message] of rules) if (!ok) errors[id] = message;
    const first = rules.find(([, ok]) => !ok);
    if (first) document.getElementById(first[0])?.focus();
    return !first;
  }

  return { errors, check };
}

/** Empty when `value` is an integer TCP port, otherwise the message to show. */
export function portError(value: unknown): string {
  const port = typeof value === "string" && value.trim() === "" ? Number.NaN : Number(value);
  return Number.isInteger(port) && port >= 1 && port <= 65535
    ? ""
    : "端口必须是 1–65535 之间的整数";
}

/**
 * Empty when `value` looks like the host internal/device/target.go accepts (an IP address or a
 * host name without scheme, path, port or surrounding spaces). The backend still validates fully.
 */
export function hostError(value: unknown): string {
  const host = String(value ?? "");
  if (host === "") return "请输入路由器地址";
  if (host.trim() !== host || /[/\\?#@\s]/.test(host) || host.includes("://")) {
    return "地址只能是 IP 或主机名，不带协议、路径或空格";
  }
  if (host.length > 253 || host.includes("..")) return "地址格式不正确";
  return "";
}

/** Rules for the target form fields on the dashboard and settings pages. */
export function targetRules(
  target: { host: string; web_port: unknown; ssh_port: unknown; telnet_port: unknown },
  profileID: string,
): FieldRule[] {
  return [
    ["target-host", !hostError(target.host), hostError(target.host)],
    ["target-web-port", !portError(target.web_port), portError(target.web_port)],
    ["target-ssh-port", !portError(target.ssh_port), portError(target.ssh_port)],
    ["target-telnet-port", !portError(target.telnet_port), portError(target.telnet_port)],
    ["target-profile", profileID.trim() !== "", "请输入 Profile，或使用 auto"],
  ];
}

/** A rule that a required text value is present. */
export function required(id: string, value: unknown, message: string): FieldRule {
  return [id, String(value ?? "") !== "", message];
}
