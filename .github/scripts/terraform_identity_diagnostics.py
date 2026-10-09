"""Fixed public failure labels shared by script and imported receipt modules."""

STAGES = frozenset({
    "oidc", "gcp-project", "gcp-quota", "gcp-export", "gcp-function",
    "aws-sts", "aws-state-read", "history-policy", "history-head",
    "history-get", "history-re-head", "history-lock-list",
    "history-workspace-list", "identity-mode", "identity-output",
})
CATEGORIES = frozenset({"check-failed", "invalid-evidence", "dependency-error"})


class SmokeFailure(Exception):
    pass


class StageFailure(Exception):
    """Only fixed diagnostic labels may cross the public output boundary."""

    def __init__(self, stage: str, category: str):
        if stage not in STAGES or category not in CATEGORIES:
            raise ValueError("Invalid identity smoke diagnostic label")
        self.stage, self.category = stage, category
        super().__init__(stage, category)


def at_stage(stage: str, operation):
    try:
        return operation()
    except StageFailure:
        raise
    except SmokeFailure:
        raise StageFailure(stage, "check-failed") from None
    except (ValueError, TypeError, KeyError, IndexError):
        raise StageFailure(stage, "invalid-evidence") from None
    except Exception:
        # Dependency exceptions may contain URLs, payloads, tokens and IDs.
        raise StageFailure(stage, "dependency-error") from None
