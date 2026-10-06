# MyPage implementation contract

現行仕様から実装に必要な非機密 wire contract だけを抽出したもの。`openapi.yaml` は six-endpoint API の契約、`fixtures/` は実ユーザーに依存しない合成例。

- 本人識別は YouTube channel ID と同じ Firebase custom-provider uid。client から対象 ID を受け取らない。
- OAuth callback の atomic claim と confirm の atomic consume の後に、外部 API / token mint を transaction callback の外で実行する。
- 現在・今日・月曜開始の週・生涯累計・直近7暦日は同じ snapshot / `asOf` に揃える。
- seconds を合計し、表示時だけ分に切り捨てる。取得障害・不完全履歴・矛盾は正常な0に置き換えない。
- account metadata と作業統計の部分障害を分離し、raw ID / credentials / DB path を response・log に載せない。

契約検証:

```sh
python -m pip install -r docs/mypage/contract-requirements.txt
python .github/scripts/check-mypage-contract.py
```

Frontend と backend は独立して検証する。provisioning / production deployment はこの実装 stack に含まない。
