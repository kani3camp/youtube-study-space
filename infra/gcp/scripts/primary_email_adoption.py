"""既存の主要メール通知先だけを、値を変えずに取り込む通常経路の例外。

取り込みで変わってよいのは、宛先ラベルに付く sensitive の印だけです。値・未知値・
置換が少しでも変わる計画は、この例外では許可しません。
"""
import json
import re

ADDRESS = 'module.notification_channels.google_monitoring_notification_channel.primary_email[0]'


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
    if not importing or type(name) is not str:
        return False
    if re.fullmatch('projects/' + re.escape(project) + '/notificationChannels/[0-9]+', name) is None:
        return False
    before, after = change.get('before'), change.get('after')
    return (item.get('address') == ADDRESS
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
