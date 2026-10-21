# Development Runtimeの配備準備

対象は`test-youtube-study-space` / `asia-southeast2`だけ。Foundationの8資源とRuntime配備を区別し、実ユーザー公開のGateは[release手順](../../../docs/mypage/release/README.md)を維持する。

## コンテナ

`system/Dockerfile.mypage`は既存のdigest固定Go / distrolessイメージを使用し、Cloud Run用のlinux/amd64バイナリをビルドする。CGOなし、timezone dataをバイナリへ含め、nonrootで起動する。Secretやローカル環境ファイルをイメージへ渡さない。

```sh
docker build --platform linux/amd64 --file system/Dockerfile.mypage --tag mypage-api:ci system
bash system/scripts/check-mypage-image.sh mypage-api:ci
```

通常のSystem CIで実イメージをビルドし、ネットワーク無効・read-only filesystemで`--check-config`を実行する。設定なしの拒否、合成設定の受理、nonrootを確認する。この検証はADC、Secret値、実Google OAuth、Cloud Run起動や公開Readyを証明しない。CIはimage push / deploy / cloud認証を行わない。

## 実環境の現在地

### 専用Registryの段階

`CI`の明示dispatch `mypage_foundation=true` / `mypage_foundation_mode=registry-plan`または`registry-apply`で、同じ独立DEV Stateを使ってFoundation8資源と専用Registry1資源だけをPlanする。既存のexact actor/ref/workflow/SHA、2 identityの独立Plan照合、HMAC、native lock、private raw evidence、保存Plan適用の契約を維持する。旧4/8資源modeの許可範囲は増やさない。

Registryは固定project/region/ID、DOCKER、STANDARD_REPOSITORY、immutable tags、cleanup dry-run・自動削除なし。guardは9資源のcreate/no-opだけを許可し、既存の同名RepositoryがState外にあればcreateを拒否する。source追加だけでは実Registry・image・Cloud Runを作成しない。RegistryをStateへ追加した後は、旧foundation段階へ戻さず`registry-plan`でno-opを検証する。

実適用前にDEVの既存SAへ必要最小限のRegistry metadata readを追加し、Applyだけに初回Repository createを一時付与する。`repositories.get` / `operations.get`の読取りと`repositories.create`はmetadata作成権限であり、upload、delete、repository IAM、project IAM、Secret payloadは含めない。create grantは1資源の作成とreadback後に解除する。image pushに必要な資源単位権限は、別の実build/deploy接続時に確認する。既存Foundation bootstrap roleへSecret値・Cloud Run・project IAMを混在させない。

2026-10-21 JSTにCloud Shellを再接続し、GCPからFoundationのIAMを独立に再取得した。runtime自身のsigner、3 Secret単位のAccessor、bootstrap用の資源単位editor、Plan/Apply SAのproject metadata-read-onlyとWIF memberが想定どおり。project create grantなし、3 Secretのversionなし、3 SAのuser-managed keyなしを確認した。DEV Cloud Runは0件、同regionのArtifact Registryは既存`gcf-artifacts`のみ。

次の実配備では専用Artifact Registryとdigest固定image、実Firebase app/origin、番号固定Secret version、Runtime用typed Plan/Applyを接続する。既存Functionsのrepositoryやimport-only Stateを流用しない。Google Auth Platformには自動作成の既存Web clientがあるが、MyPage用clientとして採用・変更していない。

実際のServing revision / image digest / Hosting release / 同意版 / 試験結果を[release record](../../../docs/mypage/release/release-record.example.json)へ一対で記録する。OAuth code/tokenや生ログはpublic PR/CI artifactへ残さない。
