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


def _bool_tree(value):
    """Terraform のマスク（sensitive / unknown）として読める形か。葉は真偽値だけを許す。"""
    if isinstance(value, dict):
        return all(type(key) is str and _bool_tree(item) for key, item in value.items())
    if isinstance(value, list):
        return all(_bool_tree(item) for item in value)
    return type(value) is bool


def _marked(mask, path=()):
    """sensitive の印が付いた葉のパス。"""
    if isinstance(mask, dict):
        return set().union(*(_marked(value, path + (key,)) for key, value in mask.items()))
    if isinstance(mask, list):
        return set().union(*(_marked(value, path + (index,)) for index, value in enumerate(mask)))
    return {path} if mask is True else set()


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
    unknown = change.get('after_unknown', {})
    # Fail closed on any mask that is not a tree of booleans, so an unexpected
    # leaf cannot be read as "not sensitive" or "known".
    if not all(type(mask) is dict and _bool_tree(mask) for mask in (*masks, unknown)):
        return False
    # Only the recipient label is sensitive afterwards. An update adds exactly
    # that mark to an unmarked import; a no-op keeps identical masks.
    expected_before = set() if change.get('actions') == ['update'] else {('labels', 'email_address')}
    return (_marked(masks[1]) == {('labels', 'email_address')}
            and _marked(masks[0]) == expected_before
            and (change.get('actions') == ['update'] or _canonical(masks[0]) == _canonical(masks[1]))
            and item.get('address') == ADDRESS
            and item.get('type') == 'google_monitoring_notification_channel'
            and item.get('mode') == 'managed'
            and item.get('provider_name') == 'registry.terraform.io/hashicorp/google'
            and importing == {'id': name}
            and change.get('actions') in (['no-op'], ['update'])
            and not change.get('replace_paths')
            and type(before) is dict and type(after) is dict
            and _canonical(before) == _canonical(after)
            and not _has_unknown(unknown)
            and after.get('name') == name and after.get('project') == project
            and after.get('type') == 'email')
