# Router operation risks

## General

Use this tool only on a router you own or are explicitly authorized to administer. Keep stable power and a wired/local management path. Stop Web UI configuration, firmware upgrades, and other writers during any operation. A local flock cannot prevent a person from changing the same router through another interface.

Web administration and Telnet use plaintext local-network protocols. Telnet is disabled unless explicitly allowed and is never accepted as final SSH evidence.

## Temporary SSH enable

`enable` is not fully reversible:

- `nvram` SSH and Telnet switches are committed.
- Dropbear's init script channel is changed and the service is started.
- The root password is changed.
- Three MAC filter rules are attempted and cleanup is always attempted.

The word “temporary” applies only to the usual Dropbear-script lifetime. It does not roll back NVRAM or the root password. A successful HTTP request is not command-success evidence; final success requires a new SSH root login and completed cleanup.

## Guided setup

`setup` and the desktop 向导 page ask for one confirmation that covers several writes: the confirmation lists every planned step and its effects, including the root password change, the committed NVRAM switches, the managed boot script and firewall include, and the reboot. Read the whole list before confirming.

The router can change between detection and the run. The run therefore detects again first and stops before any write if the device, the profile or any step would do more or something else than you confirmed; you then have to detect and confirm again. Steps may only do less (for example skip a backup that appeared meanwhile). A step decided after SSH works may only install the soft persistence that was listed.

When SSH is open but the root password is unknown, `setup` asks for the current password instead of changing it; it changes the root password only with an explicit `--reset-root-password` choice. Deep persistence and soft-persistence removal are never part of a guided run.

The router is identified by its normalized address, not by its ports: one lock, one recovery record and one set of managed directories exist per address. A host name and an IP address of the same router count as two routers, so use one form consistently.

## Saved passwords

`--remember-password` saves a typed Web admin or root password after the router accepted it, in `<user config dir>/routerctl/credentials/` (on the desktop, the 记住密码 switch of the 向导 page, off each time the page opens). Each password is encrypted with AES-256-GCM. On every platform, any program running as your user can decrypt the saved passwords. On Windows the vault key is additionally protected with DPAPI, so a copied `vault.key` is useless under another account or on another computer; on macOS and Linux the key is a 0600 file, and copying `vault.json` together with `vault.key` is enough to decrypt. This protects against casual viewing and against someone copying only `vault.json`; it is not a master password. Uninstalling the app does not delete the vault; `routerctl credentials forget --all` does.

A saved root password is never sent over Telnet and never to an SSH host key that was not already known, pinned or confirmed by you. A saved password the router refuses is marked rejected and not used again until you type the right one and save it again with `--remember-password`. Deep persistence never uses saved passwords.

## Agent access (MCP)

`routerctl mcp` lets an AI agent call the same detection, backup and guided setup as the CLI. It runs only when you start it, and router writes (`guide_apply`, `soft_remove`) exist only when you start it with `--allow-writes`. Deep persistence, the recovery acknowledgement, forgetting saved passwords and Telnet are never available to agents.

- **Passwords in the agent's transcript.** Passwords an agent passes as tool arguments are stored in the agent host's conversation history and sent to its model provider. Router Tools never logs or echoes them, but it cannot remove them from the host. Prefer passwords you saved with `--remember-password` and let the agent use `use_saved_password`.
- **The write switch is consent, not a sandbox.** An agent that also has shell or file access can run `routerctl` directly or edit its own MCP configuration to add `--allow-writes`. Restricting that is the job of the agent host's permission settings. Hosts that auto-approve tool calls (bypass or auto modes) will run writes without asking you; in Claude Code, `permissions.ask` rules for `mcp__router-tools__guide_apply`, `mcp__router-tools__soft_remove` and `mcp__router-tools__operation_cancel` force a prompt.
- **One confirmation, several writes.** `guide_apply` runs the whole plan that `guide_plan` returned, like `setup`. Read the plan's steps and effects (or have the agent show them) before letting it apply. A plan can be applied once; a repeat needs a new `guide_plan`.
- **Long operations continue after the agent stops waiting.** A call returns after at most 50 seconds and the operation keeps running; the agent must poll `operation_wait`. A host timeout does not stop a write. When the host ends the session it may kill a stdio server within a second; an interrupted write then leaves a blocking recovery record (see RECOVERY.md).
- **HTTP on the local network.** `--http` always requires a token. Bound to a non-loopback address, the token and any passwords in tool arguments cross the network in plaintext, and anyone holding the token can use every tool the server offers. Use `127.0.0.1`, or a network you trust, and rotate the token when in doubt.
- **Host keys.** A saved root password is never sent with a fingerprint the agent supplied, because a pin replaces the known_hosts check. With a password the agent passes itself, `host_key_sha256` and `accept_new_host_key` trust whatever the agent says; only accept fingerprints you gave it.
- **Local paths in results.** Reports include the managed backup and journal paths, which contain your user name; they reach the agent's model provider.
- **Device text in results.** Model names and other strings reported by the router appear in tool results. Agents should treat them as data, not instructions.

## Soft persistence and reboot

Soft persistence installs `/data/auto_ssh/auto_ssh.sh` and the `firewall.auto_ssh` include. Installation verification does not prove boot behavior. Only `reboot-verify` or an equivalent manual reboot can establish that.

Removal is restricted to the managed path. It does not restore the old root password, disable NVRAM services, stop Dropbear, or modify partitions.

## Backup limitations

An MTD backup is a sequence of runtime reads, not an atomic cross-partition snapshot. A backup does not prove that recovery is possible or safe. There is no `restore` command and no automatic partition rewrite.

Keep backup directories private. They can contain device-specific or otherwise sensitive firmware data even though passwords, `stok`, Web keys, and private keys are excluded from manifests.

## Deep persistence — high risk

Deep persistence writes and erases MTD, modifies NVRAM and `bdata`, and reboots exactly three times. Interruption, wrong layout, power loss, or firmware differences can make the router unbootable.

Before the first write, the tool requires:

- exact RD08 `1.1.96` compatibility;
- initial SSH root authentication;
- explicit Telnet permission;
- local backup integrity verification;
- online identity, full MTD layout, and unchanged `boot_id` comparison;
- required tools and a readable `crash` partition;
- the resolved profile's exact deep acknowledgement (`RD08-DEEP` for `xiaomi-rd08-1.1.96`), checked before any network access and again after resolution;
- no existing journal for that host/run directory.

`--yes` cannot replace the deep acknowledgement. The tool never automatically retries an uncertain crash write/erase, never resumes from a guessed stage, and never attempts automatic rescue.

**Implementation and offline fake coverage are not real-device validation.** Run deep persistence only after the ordered real-device QA in `docs/compatibility.md`, on a device whose loss you can recover from, with an official recovery method available.
