"""Fixed public failure labels shared by script and imported receipt modules."""
from __future__ import annotations

STAGES = frozenset({
    "oidc", "gcp-project", "gcp-quota", "gcp-export", "gcp-function",
    "aws-sts", "aws-state-read", "aws-state-probe-request", "aws-state-probe-post",
    "aws-state-probe-invariant", "aws-state-probe-deny", "aws-state-probe-receipt",
    "history-policy", "history-head",
    "history-get", "history-re-head", "history-lock-list",
    "history-workspace-list", "identity-mode", "identity-output", "gcp-plan-service-account-target",
})
CATEGORIES = frozenset({"check-failed", "invalid-evidence", "dependency-error"})
RECEIPT_REASONS = frozenset({
    "json-parse", "json-duplicate", "json-nonfinite", "file-open-read", "file-open-write",
    "s3-request", "s3-cli-failure", "checks-catalog-source", "checks-catalog-read",
    "get-file-open",
    "context-project", "context-history", "context-mode", "context-sha", "context-temp",
    "file-owner-mode", "file-size", "timestamp-shape",
    "cost-json-size", "cost-profile-shape", "cost-model-sha", "cost-time-window",
    "cost-month", "cost-month-margin", "cost-attestation", "cost-state-size",
    "cost-overhead-shape", "cost-overhead-range", "cost-cap",
    "policy-record-shape", "policy-record-match", "policy-state-target",
    "policy-account-shape", "policy-bucket-shape", "policy-retry",
    "s3-response-bound", "s3-json-envelope", "list-fields", "list-encoding",
    "list-owner-prefix", "list-max", "list-empty-count", "list-empty-contents",
    "list-no-pagination", "head-size", "head-version", "head-etag",
    "head-storage-marker", "head-restoration-encoding", "head-encryption",
    "state-envelope", "state-version", "state-serial", "state-tf-version",
    "state-lineage", "state-output-names", "state-output-value",
    "state-resource-list", "state-resource-envelope", "state-resource-owner",
    "state-resource-duplicate", "state-instance-list", "state-instance-envelope",
    "state-instance-version", "state-instance-identity-version", "state-instance-identity",
    "state-instance-lifecycle", "state-instance-sensitive-paths", "state-instance-private",
    "state-instance-dependencies", "state-instance-project", "state-instance-index",
    "state-address-set", "checks-list", "checks-envelope", "checks-address",
    "checks-result", "checks-object-list", "checks-object-envelope",
    "checks-object-address", "checks-object-result", "checks-resource-address",
    "get-version-etag", "get-size", "rehead-equal", "before-state-hash",
    "after-initial-evidence", "after-metadata-status", "after-safety",
    "after-cost-fresh", "after-outcomes", "after-plan-exit", "after-plan-envelope",
    "after-plan-version", "after-plan-target", "after-plan-count-types",
    "after-plan-counts", "after-plan-resources-list", "after-plan-resources-envelope",
    "after-plan-addresses", "after-plan-actions",
})


class SmokeFailure(Exception):
    pass


class ReceiptInvariantError(ValueError):
    """A failed receipt guard with a fixed, payload-free reason code."""

    def __init__(self, reason: str):
        if reason not in RECEIPT_REASONS:
            raise ValueError("Invalid receipt diagnostic label")
        self.reason = reason
        super().__init__("Protected history plan receipt STOP; private diagnostic suppressed.")


class ReceiptDependencyError(Exception):
    """A dependency failure with a fixed, payload-free reason code."""

    def __init__(self, reason: str):
        if reason not in RECEIPT_REASONS:
            raise ValueError("Invalid receipt diagnostic label")
        self.reason = reason
        super().__init__("Protected history plan dependency STOP; private diagnostic suppressed.")


class StageFailure(Exception):
    """Only fixed diagnostic labels may cross the public output boundary."""

    def __init__(self, stage: str, category: str, reason: str | None = None):
        if stage not in STAGES or category not in CATEGORIES or (reason is not None and reason not in RECEIPT_REASONS):
            raise ValueError("Invalid identity smoke diagnostic label")
        self.stage, self.category, self.reason = stage, category, reason
        super().__init__(stage, category, reason)


def at_stage(stage: str, operation):
    try:
        return operation()
    except StageFailure:
        raise
    except ReceiptInvariantError as error:
        raise StageFailure(stage, "invalid-evidence", error.reason) from None
    except ReceiptDependencyError as error:
        raise StageFailure(stage, "dependency-error", error.reason) from None
    except SmokeFailure:
        raise StageFailure(stage, "check-failed") from None
    except (ValueError, TypeError, KeyError, IndexError):
        raise StageFailure(stage, "invalid-evidence") from None
    except Exception:
        # Dependency exceptions may contain URLs, payloads, tokens and IDs.
        raise StageFailure(stage, "dependency-error") from None
