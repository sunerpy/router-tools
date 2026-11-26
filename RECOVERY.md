# Interruption assessment and recovery boundaries

There is deliberately no automatic `resume`, `restore`, or one-click rescue command. Preserve the run report, deep journal, backup directory, and all console output. Do not restart `deep-persist` merely because the previous attempt failed.

## If a write operation was interrupted

1. Do not issue the same write again until the device state is known.
2. Read the report status:
   - `failed`: the operation stopped with an error. It may still have written: soft install, soft removal and reboot verification also report `failed` after a write, and their reports carry `write_started`; enable reports carry `accepted_steps` and `attempted_macs`. When a write may have happened, the recovery record blocks further changes.
   - `partial`: at least one write or cleanup stage occurred.
   - `unknown`: a write may have been submitted but its result was lost.
3. Check SSH independently with the current root credentials and pinned host key.
4. For enable failures, inspect all `attempted_macs` and `cleanup` entries. Remove remaining temporary rules through the router Web UI only after confirming their exact values.
5. Do not change NVRAM or Telnet settings automatically; their previous values are unknown and they may be needed for diagnosis.

## Recovery record and acknowledgement

Every device change (`enable`, `persist soft`, `persist remove`, `reboot-verify`, `deep-persist`, and each write step of `setup`) first stores a record in `<user config dir>/routerctl/state/` and refuses to start if it cannot. While the last change of a router is recorded as `in_progress` (the process stopped or its final record failed), `partial`, `unknown`, or `failed` after a write may have been submitted, every further change of that router fails with `RECOVERY_REVIEW_REQUIRED` (exit 9), whichever ports are used.

1. Check the router as described above and for the stage the report names.
2. Only then run `routerctl recovery ack`. In the desktop app, the 向导 page offers 我已检查路由器 when detection is blocked; it records the same acknowledgement. Acknowledging records when you checked; the record itself is kept, and a later uncertain change blocks again.
3. Acknowledging is not a repair. It only states that you know the router's actual state and accept continuing from it.

The record keeps the adapter's evidence (`write_evidence`): whether a write started, accepted enable steps, the temporary MAC rules that were attempted and those whose removal failed, and the deep journal path. Use it when the run report is no longer available.

`STATE_STORE_FAILED` (exit 11) means the record could not be read or written; nothing was sent to the router if it happened before the change, and the record stays `in_progress` if it happened after. Fix the local disk or permissions problem, check the router, then acknowledge. If the record itself cannot be parsed, acknowledging fails too: check the router, then move the router's file in `<user config dir>/routerctl/state/` (named by its recovery key, whose first 12 characters also end the router's managed directory names) out of the directory, keeping a copy.

## Deep journals outside the managed directory

Without a run directory, `deep-persist` keeps its journal in the router's managed directory, so any later attempt finds it. The desktop deep flow always uses the managed directory too. Journals written to a custom `--run-dir`, and by desktop builds from before the 向导 page (which wrote to the directory you chose), are found only in that directory. Before another deep attempt, check every earlier run directory for a journal of the router.

## Runs started by an agent

Agent runs (through `routerctl mcp` or the desktop's 「Agent 接入」) leave the same recovery records as your own runs and are blocked by them in the same way. Agents cannot acknowledge a record: check the router yourself, then run `routerctl --host <address> recovery ack` or use 我已检查路由器 in the desktop app. In the desktop's 运行报告, such runs carry an Agent badge.

## If a guided run stopped

`setup` runs its steps through the same commands and stops at the first step that did not succeed. Steps after it are `not_run` and were not attempted. Read the failing step's own report (enable, backup, soft or reboot) and handle it as that command's failure. Never repeat the run only because it stopped: detect again with `setup --plan-only`, which shows what is still missing, and acknowledge first if the stopped step blocks further changes.

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
