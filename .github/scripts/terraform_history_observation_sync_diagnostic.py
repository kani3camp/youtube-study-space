"""Failure-only finite diagnostics for the separate sync protocol; no admission."""
from __future__ import annotations

import copy
import json

import terraform_history_observation_sync_profile as profiles
import terraform_plan_diagnostic as legacy
import terraform_baseline_drift_diagnostic as drift
import validate_history_observation_sync as gate

STEPS = frozenset(('NOT_OBSERVED', 'OBSERVATION_PLAN', 'OBSERVATION_SHOW', 'OBSERVATION_VALIDATE'))
EXITS = frozenset(('NOT_OBSERVED', '0', '1', '2', 'OTHER', 'TIMEOUT'))
REASONS = frozenset(('NOT_OBSERVED', 'UNCLASSIFIED', 'EXIT_REJECTED', 'TIMEOUT',
    'OUTPUT_BOUND', 'SHOW_READ', 'JSON_PARSE', 'JSON_BOUND')) | gate._REASONS
FACTS = frozenset(('UNCHANGED', 'CHANGED', 'NOT_OBSERVED'))
MAX_BYTES = 2048


def scope(env):
    phase = env.get('HISTORY_OBSERVATION_SYNC_OPERATION')
    profiles.context(env, phase=phase)
    profiles.need(env.get('GITHUB_JOB') == 'history_observation_sync_' +
        ('plan' if phase == 'preview' else 'apply'))


def json_failure(raw):
    """Classify a failed existing parser without publishing its input or error."""
    try:
        if type(raw) is not bytes or not 0 < len(raw) <= gate.MAX_PLAN_BYTES:
            return 'SHOW_READ'
        def reject(_):
            raise ValueError('JSON rejected')
        value = json.loads(raw, object_pairs_hook=gate._unique, parse_constant=reject)
    except Exception:
        return 'JSON_PARSE'
    try:
        gate._bounded(value)
    except Exception:
        return 'JSON_BOUND'
    return 'UNCLASSIFIED'


class Diagnostic:
    def __init__(self):
        self.step = 'NOT_OBSERVED'
        self.plan_exit = self.show_exit = 'NOT_OBSERVED'
        self.reason = self.guard_stage = self.guard_field = 'NOT_OBSERVED'
        self.counts = self.projection = None
        self.state_before = self.metadata_before = 'NOT_OBSERVED'

    def start(self, step):
        profiles.need(type(step) is str and step in STEPS)
        self.step = step

    def event(self, event, value=None):
        profiles.need(self.step in ('OBSERVATION_PLAN', 'OBSERVATION_SHOW'))
        if event == 'EXIT':
            code = str(value) if type(value) is int and value in (0, 1, 2) else 'OTHER'
            if self.step == 'OBSERVATION_PLAN': self.plan_exit = code
            else: self.show_exit = code
            if code != '0': self.reason = 'EXIT_REJECTED'
        elif event == 'TIMEOUT':
            if self.step == 'OBSERVATION_PLAN': self.plan_exit = 'TIMEOUT'
            else: self.show_exit = 'TIMEOUT'
            self.reason = 'TIMEOUT'
        else:
            profiles.need(event in ('OUTPUT_BOUND', 'SHOW_READ', 'JSON_PARSE', 'JSON_BOUND', 'UNCLASSIFIED'))
            self.reason = event

    def observe(self, plan):
        # Unknown/partial/overbudget shapes publish no partial projection.
        self.counts = self.projection = None
        try:
            gate._bounded(plan)
            profiles.need(type(plan) is dict and set(plan) <= gate._PLAN_KEYS
                and plan.get('format_version') == '1.2'
                and plan.get('terraform_version') == gate.TERRAFORM_VERSION
                and plan.get('complete') is True and plan.get('errored') is False)
            profiles.need(type(plan.get('resource_changes')) is list)
            budget = [drift.MAX_NODES, drift.MAX_TEXT_BYTES]
            for item in plan['resource_changes']:
                drift._entry(item, budget)
            counts, projection = legacy.counts(plan), drift.project(plan)
            profiles.need(counts is not None and projection is not None)
            legacy.validate(dict(schema_version=1, outcome='STOP', stage='COUNTS', field='COUNTS', counts=counts))
            drift.validate_projection(projection)
            self.counts, self.projection = counts, projection
        except Exception:
            pass

    def failure_show(self, event, value=None):
        # The original plan rejection remains authoritative. Diagnostic-only
        # show cannot overwrite its step, exit or reason.
        if event == 'EXIT':
            self.show_exit = str(value) if type(value) is int and value in (0, 1, 2) else 'OTHER'
        elif event == 'TIMEOUT': self.show_exit = 'TIMEOUT'

    def capture(self, plan):
        self.observe(plan)

    def rejected(self, error, plan, *, metadata, execution_email):
        self.reason = (error.reason if type(error) is gate.SyncRejected
            and type(error.reason) is str and error.reason in REASONS else 'UNCLASSIFIED')
        self.observe(plan)
        if self.reason != 'CONFIGURATION': return
        context = legacy.Context()
        try:
            # Failure-only pure replay. This copy is never an admission,
            # summary, candidate, issuer or Terraform input.
            projection = copy.deepcopy(plan)
            projection['resource_drift'] = []
            gate.validate_history(projection, metadata=metadata, phase='post',
                execution_email=execution_email, diagnostic=context)
        except Exception:
            self.guard_stage = context.stage if context.stage in legacy.STAGES else 'UNCLASSIFIED'
            self.guard_field = context.field if context.field in legacy.FIELDS else 'UNCLASSIFIED'

    def value(self):
        return dict(schema_version=1, kind='dev-history-observation-sync-stop-diagnostic-v1',
            step=self.step, plan_exit=self.plan_exit, show_exit=self.show_exit, reason=self.reason,
            guard_stage=self.guard_stage, guard_field=self.guard_field,
            counts=self.counts, drift=self.projection,
            state_before=self.state_before, metadata_before=self.metadata_before)

    def render(self, env):
        scope(env)
        value = self.value()
        validate(value)
        lines = ['SYNC_DIAGNOSTIC: step=' + self.step + ' plan_exit=' + self.plan_exit
            + ' show_exit=' + self.show_exit + ' reason=' + self.reason
            + ' guard_stage=' + self.guard_stage + ' guard_field=' + self.guard_field]
        lines.append('SYNC_DIAGNOSTIC_COUNTS=' + ('NOT_OBSERVED' if self.counts is None else
            ','.join(key + ':' + str(self.counts[key]) for key in legacy.COUNT_KEYS)))
        lines.append('SYNC_DIAGNOSTIC_DRIFT=NOT_OBSERVED' if self.projection is None else
            'SYNC_DIAGNOSTIC_DRIFT_TARGETS=' + str(len(self.projection['items'])))
        if self.projection is not None:
            for item in self.projection['items']:
                alias, _, resource_type = drift.ALIASES[item[0]]
                fields = drift.FIELDS[resource_type]
                kinds = [kind + '=' + (','.join(field.upper() for position, field in enumerate(fields)
                    if int(bits, 16) & (1 << position)) or 'NONE')
                    for kind, bits in zip(drift.KINDS, item[3:])]
                lines.append('SYNC_DIAGNOSTIC_TARGET: role=' + alias + ' action=' + item[1]
                    + ' object_presence=' + ('NEITHER', 'BEFORE', 'AFTER', 'BOTH')[item[2]] + ' ' + ' '.join(kinds))
        lines.extend(('SYNC_FAILURE_STATE_VS_BEFORE=' + self.state_before,
            'SYNC_FAILURE_METADATA_VS_BEFORE=' + self.metadata_before))
        return lines


def validate(value):
    profiles.need(type(value) is dict and set(value) == {'schema_version', 'kind', 'step',
        'plan_exit', 'show_exit', 'reason', 'guard_stage', 'guard_field', 'counts', 'drift',
        'state_before', 'metadata_before'} and type(value['schema_version']) is int
        and value['schema_version'] == 1 and value['kind'] == 'dev-history-observation-sync-stop-diagnostic-v1')
    for key, allowed in (('step', STEPS), ('plan_exit', EXITS), ('show_exit', EXITS), ('reason', REASONS),
            ('guard_stage', legacy.STAGES | {'UNCLASSIFIED'}), ('guard_field', legacy.FIELDS),
            ('state_before', FACTS), ('metadata_before', FACTS)):
        profiles.need(type(value[key]) is str and value[key] in allowed)
    legacy.validate(dict(schema_version=1, outcome='STOP', stage='COUNTS', field='COUNTS', counts=value['counts']))
    drift.validate_projection(value['drift'])
    profiles.need((value['counts'] is None) == (value['drift'] is None))
    if value['drift'] is not None:
        profiles.need(value['counts']['drift'] == len(value['drift']['items']))
    profiles.need(len(profiles.canonical(value)) <= MAX_BYTES)
    return value
