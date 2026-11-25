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
- exact `RD08-DEEP` acknowledgement;
- no existing journal for that host/run directory.

`--yes` cannot replace the deep acknowledgement. The tool never automatically retries an uncertain crash write/erase, never resumes from a guessed stage, and never attempts automatic rescue.

**Implementation and offline fake coverage are not real-device validation.** Run deep persistence only after the ordered real-device QA in `docs/compatibility.md`, on a device whose loss you can recover from, with an official recovery method available.
