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

`setup` asks for one confirmation that covers several writes (the desktop wizard follows with the guided redesign): the confirmation lists every planned step and its effects, including the root password change, the committed NVRAM switches, the managed boot script and firewall include, and the reboot. Read the whole list before confirming.

The router can change between detection and the run. The run therefore detects again first and stops before any write if the device, the profile or any step would do more or something else than you confirmed; you then have to detect and confirm again. Steps may only do less (for example skip a backup that appeared meanwhile). A step decided after SSH works may only install the soft persistence that was listed.

When SSH is open but the root password is unknown, `setup` asks for the current password instead of changing it; it changes the root password only with an explicit `--reset-root-password` choice. Deep persistence and soft-persistence removal are never part of a guided run.

The router is identified by its normalized address, not by its ports: one lock, one recovery record and one set of managed directories exist per address. A host name and an IP address of the same router count as two routers, so use one form consistently.

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
