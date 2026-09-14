# SeatAppearance V2 rollout runbook

Issue #1107 の SeatAppearance V2 は、互換期間を設けて段階的にリリースする。

## Release 1

次の順序でデプロイする。

1. V1/V2 dual-read 対応の YouTube monitor をデプロイする。
2. 配信で使用する全 monitor / OBS browser source の DOM を確認し、`[data-seat-appearance-schema-read="1,2"]` が存在することを確認する。
3. V2 canonical field と V1 legacy field を dual-write する backend をデプロイする。
4. `youtube-bot` と `youtube_organize_database` の両方の起動ログで `seat-appearance-schema-write=2` と `seat-appearance-legacy-write=true` を確認する。

この期間は V1 document の generic update では V1 のまま保持する。新規入室、移動、`!my rank`、`!my color`、`!rank` など、appearance 全体を再計算する操作だけが V2 へ昇格させる。

## Drain 診断

`seat-appearance-drain-audit` は read-only で `seats` と `member-seats` を全件読み込み、application 側で `schema-version == 2` ではない document を数える。欠落、0、1、未知または不正な version はすべて未 drain と判定する。

対象環境の `.env` と認証情報を用意し、意図した GCP project ID を明示して実行する。

```sh
cd system
go run ./cmd/seat-appearance-drain-audit <expected-project-id>
```

出力形式は次のとおり。

```text
seats: <V1>
member-seats: <V1>
total: <V1>
```

## Release 2 Gate

次の条件をすべて満たすまで Release 2 に進まない。

- `youtube-bot` と `youtube_organize_database`（Seat を full Set する全 writer）の起動ログで V2 dual-write capability を確認できる。
- 配信で使用する全 frontend / OBS browser source の DOM に `[data-seat-appearance-schema-read="1,2"]` が存在する。
- production の drain 診断が十分な時間を空けた2回で連続して `total: 0` になる。

条件を満たさない場合は Release 1 の frontend/backend に戻し、強制 migration や配信停止は行わない。

Release 2 はこの実装範囲に含めない。Gate 通過後の別変更で legacy field の書き込みと frontend fallback を削除する。Release 2 で full Set を行った結果、既存 document から legacy field が自然に削除されることは許容する。
