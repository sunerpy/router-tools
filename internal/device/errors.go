package device

import "fmt"

const (
	CodeInvalidArgument       = "INVALID_ARGUMENT"
	CodeInvalidPassword       = "INVALID_PASSWORD"
	CodeUnsupportedDevice     = "UNSUPPORTED_DEVICE"
	CodeUnsupportedFirmware   = "UNSUPPORTED_FIRMWARE"
	CodeUnsupportedProfile    = "UNSUPPORTED_PROFILE"
	CodeAmbiguousDevice       = "AMBIGUOUS_DEVICE"
	CodeInvalidProfile        = "INVALID_PROFILE"
	CodeUnsupportedCapability = "UNSUPPORTED_CAPABILITY"
	CodeWebUnreachable        = "WEB_UNREACHABLE"
	CodeWebProtocolError      = "WEB_PROTOCOL_ERROR"
	CodeWebAuthFailed         = "WEB_AUTH_FAILED"
	CodeSSHAuthFailed         = "SSH_AUTH_FAILED"
	CodeSSHUnreachable        = "SSH_UNREACHABLE"
	CodeSSHHostKeyRejected    = "SSH_HOST_KEY_REJECTED"
	CodeHostKeyMismatch       = "HOST_KEY_MISMATCH"
	CodeRootVerifyFailed      = "ROOT_VERIFY_FAILED"
	CodeRemoteCommandFailed   = "REMOTE_COMMAND_FAILED"
	CodeRemoteResultUnknown   = "REMOTE_RESULT_UNKNOWN"
	CodeRemoteAPIRejected     = "REMOTE_API_REJECTED"
	CodeMACCleanupFailed      = "MAC_CLEANUP_FAILED"
	CodeConfirmationRequired  = "CONFIRMATION_REQUIRED"
	CodeSoftConfigConflict    = "SOFT_CONFIG_CONFLICT"
	CodeBackupHashMismatch    = "BACKUP_HASH_MISMATCH"
	CodeBackupIncomplete      = "BACKUP_INCOMPLETE"
	CodeBackupLayoutMismatch  = "BACKUP_LAYOUT_MISMATCH"
	CodeBackupMissing         = "BACKUP_MISSING"
	CodeRebootNotObserved     = "REBOOT_NOT_OBSERVED"
	CodeBootIDUnchanged       = "BOOT_ID_UNCHANGED"
	CodeRebootTimeout         = "REBOOT_TIMEOUT"
	CodeDeepAckRequired       = "DEEP_ACK_REQUIRED"
	CodeDeepJournalExists     = "DEEP_JOURNAL_EXISTS"
	CodeOperationLocked       = "OPERATION_LOCKED"
	// CodeRecoveryReviewRequired blocks a device change after an uncertain one until it is acknowledged.
	CodeRecoveryReviewRequired = "RECOVERY_REVIEW_REQUIRED"
	// CodeStateStoreFailed means the write-ahead outcome record could not be read or stored.
	CodeStateStoreFailed = "STATE_STORE_FAILED"
	// CodeCredentialNotFound means saved passwords were requested but no usable entry exists.
	CodeCredentialNotFound = "CREDENTIAL_NOT_FOUND"
	// CodeCredentialStoreFailed means the password vault could not be read, decrypted, or written.
	CodeCredentialStoreFailed = "CREDENTIAL_STORE_FAILED"
	CodeTelnetAuthFailed      = "TELNET_AUTH_FAILED"
	CodeTelnetUnreachable     = "TELNET_UNREACHABLE"
	// CodeAgentWritesDisabled refuses an agent's router write while writes are not enabled for agents.
	CodeAgentWritesDisabled = "AGENT_WRITES_DISABLED"
	CodeCanceled            = "CANCELED"
	CodeInternalError       = "INTERNAL_ERROR"
)

// Error is a stable, frontend-independent device error.
type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
	Stage     string `json:"stage,omitempty"`
	cause     error
}

func (e *Error) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return e.Code
}

// Unwrap retains the internal cause without exposing it in serialized reports.
func (e *Error) Unwrap() error { return e.cause }

func newError(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// WrapError creates a stable error while preserving cancellation and other causes.
func WrapError(code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, cause: cause}
}
