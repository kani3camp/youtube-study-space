"""本番の既存の主要メール通知先だけを、値を変えずに取り込む通常経路の例外。

取り込みで変わってよいのは、宛先ラベルに付く sensitive の印だけです。値・未知値・
置換が少しでも変わる計画は、この例外では許可しません。取り込みを含む計画では、
ほかの資源の変更も許可しません（採用と通知の修復を別の Apply に分ける）。
"""
import json
import re

ADDRESS = 'module.notification_channels.google_monitoring_notification_channel.primary_email[0]'
PROJECT = 'youtube-study-space'
# Selection and recipient come only from the Environment's private inputs.
PRIVATE_KEYS = frozenset({'manage_primary_email', 'primary_email_address', 'primary_email_channel_name'})


def channel_name(tfvars):
    """取り込みを選んだ入力の通知先名。選んでいなければ None を返す。"""
    if type(tfvars) is not dict or tfvars.get('manage_primary_email') is not True:
        return None
    name = tfvars.get('primary_email_channel_name')
    return name if type(name) is str else None


def _canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'))


def _has_unknown(value):
    if isinstance(value, dict):
        return any(_has_unknown(item) for item in value.values())
    if isinstance(value, list):
        return any(_has_unknown(item) for item in value)
    return value is not False and value is not None


def allows(item, project, name):
    """この取り込みが、選択した既存通知先の値を保つ取り込みである場合だけ True。"""
    change = item.get('change', {})
    importing = change.get('importing')
    if not importing or type(name) is not str or project != PROJECT:
        return False
    if re.fullmatch('projects/' + re.escape(project) + '/notificationChannels/[0-9]+', name) is None:
        return False
    before, after = change.get('before'), change.get('after')
    masks = change.get('before_sensitive'), change.get('after_sensitive')
    if type(masks[0]) is not dict or type(masks[1]) is not dict:
        return False
    marked = {key for key in masks[0].keys() | masks[1].keys() if masks[0].get(key) != masks[1].get(key)}
    return (masks[1].get('labels') == {'email_address': True}
            and marked == ({'labels'} if change.get('actions') == ['update'] else set())
            and item.get('address') == ADDRESS
            and item.get('type') == 'google_monitoring_notification_channel'
            and item.get('mode') == 'managed'
            and item.get('provider_name') == 'registry.terraform.io/hashicorp/google'
            and importing == {'id': name}
            and change.get('actions') in (['no-op'], ['update'])
            and not change.get('replace_paths')
            and type(before) is dict and type(after) is dict
            and _canonical(before) == _canonical(after)
            and not _has_unknown(change.get('after_unknown', {}))
            and after.get('name') == name and after.get('project') == project
            and after.get('type') == 'email')
