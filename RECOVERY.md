# Interruption assessment and recovery boundaries

There is deliberately no automatic `resume`, `restore`, or one-click rescue command. Preserve the run report, deep journal, backup directory, and all console output. Do not restart `deep-persist` merely because the previous attempt failed.

## If a write operation was interrupted

1. Do not issue the same write again until the device state is known.
2. Read the report status:
   - `failed`: the operation failed before evidence of a device write, or at a defined non-writing stage.
   - `partial`: at least one write or cleanup stage occurred.
   - `unknown`: a write may have been submitted but its result was lost.
3. Check SSH independently with the current root credentials and pinned host key.
4. For enable failures, inspect all `attempted_macs` and `cleanup` entries. Remove remaining temporary rules through the router Web UI only after confirming their exact values.
5. Do not change NVRAM or Telnet settings automatically; their previous values are unknown and they may be needed for diagnosis.

## Deep journal stages

| Last relevant stage                               | Required interpretation                                                                                                           |
| ------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `PRECHECK`                                        | No deep write should have occurred. Correct local/read-only preconditions before retrying preflight.                              |
| `CRASH_WRITE_STARTED` / write timeout             | Result is uncertain. Do not write `crash` again. Inspect the actual boot/debug state and journal manually.                        |
| `REBOOT_1_*`                                      | Stop before any flag writes. Assess why the router did not go down, return, or produce a new `boot_id`.                           |
| `FLAGS_WRITE_STARTED` / flag verification failure | Do not perform the second reboot. Inspect `nvram`/`bdata` tool output and values manually.                                        |
| `REBOOT_2_*`                                      | Do not erase `crash`. Preserve the debug-boot state for manual assessment.                                                        |
| `CRASH_ERASE_STARTED` / erase timeout             | Do not erase again. Determine the actual partition state first.                                                                   |
| `REBOOT_3_*`                                      | Use the verified backup and official recovery procedure for manual assessment. No automatic restore is attempted.                 |
| `FINAL_RESTORE` / `FINAL_VERIFY`                  | The three boot transitions occurred but completion is not proven. Check soft persistence and independent SSH root login manually. |
| `UNKNOWN` / `FAILED`                              | The stage is not a resume instruction. Do not start again from step one.                                                          |

## Backup use

`backup verify` establishes local file integrity. `backup verify --online` additionally compares device identity, current MTD layout, and `boot_id`. Neither check promises recoverability. Partition images are intended for an experienced operator using an appropriate official recovery procedure.

## When to stop completely

Stop and seek manual recovery if the host key changes unexpectedly, the device identity/layout differs from the backup, `boot_id` is invalid or unchanged after a reboot, the `crash` layout is missing, or power/network stability is in doubt. Do not bypass these failures with a different profile or Telnet.
