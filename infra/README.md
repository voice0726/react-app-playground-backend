# GCP デプロイ準備

専用プロジェクト `react-app-playground-backend` の基盤を管理します。
DB は、shared-infra リポジトリが管理する共有 DB VM の `playground` データベースを使います。
共有 DB へは VPC peering で接続します。

参考は auth-gateway の [#6 Terraform による GCP 基盤管理](https://github.com/voice0726/auth-gateway/pull/6) です。
準備段階では `init -backend=false` と `validate` まで実行します。
以下のデプロイ手順は、別途デプロイを承認した後に使う手順です。

## 共有 DB への接続

`production.tfvars` に次の 3 つを設定します。

| 変数 | 値の取得元（shared-infra の output） |
| --- | --- |
| `shared_db_network` | `db_network` |
| `shared_database_url_secret` | `database_url_secrets` の `react-app-playground-backend` |
| `shared_database_url_secret_version` | `database_url_secret_versions` の `react-app-playground-backend` |

`shared_db_network` を設定すると、共有 DB の VPC との peering を作ります。
peering は、shared-infra 側も逆向きの peering を作ったときに `ACTIVE` になります。
API と migrate Job は、`shared_database_url_secret` の Secret から DATABASE_URL を読みます。

## 構成

```text
Workers frontend
  | /api (Cookie, Origin)
  v
Cloud Run API -------- HTTPS --------> auth-gateway /internal/session
  |                                   X-Internal-Token
  | Direct VPC egress
  v
Dedicated VPC
  +-- Cloud Run subnet: 10.31.0.0/26
        +-- API
        +-- migrate Job (/app/migrate up)
  |
  | VPC peering
  v
shared-infra の共有 DB VM (PostgreSQL 15, DB 名 playground)

GitHub deploy.yml (main) -- WIF --> deploy service account
  +-- Artifact Registry push
  +-- Cloud Run API / Job update and Job execution
```

| 対象 | 設定 |
| --- | --- |
| 配置 | `us-central1` |
| VPC | 自動サブネットなし、アプリ専用 |
| DB 接続 | 共有 DB の内部 IP の 5432。peering 経由で、Cloud Run 専用サブネットから許可 |
| DB 認証 | `playground` ロール、SCRAM、DB 名 `playground` |
| API | 公開 ingress、IAM invoker 検査無効、アプリ側で認証 |
| API スケール | 最小 0、最大 2 |
| migrate Job | 1 タスク、並列数 1、再試行 0、制限時間 600 秒 |
| Secret Manager | gateway 内部トークン。DATABASE_URL は shared-infra の Secret を参照 |
| WIF | 当該リポジトリの main の `.github/workflows/deploy.yml` のみ |

WIF は repository ID `1410098688` と owner ID `42563149` も要求します。
リポジトリ名だけを再利用しても、信頼条件は一致しません。

API と migrate Job は別々の service account を使います。
API と Job は shared-infra の DATABASE_URL を読めます。
この権限は shared-infra の `database_url_readers` で付けます。
gateway 内部トークンは API だけが読めます。
deploy service account は Secret の値を読む権限を持ちません。

## auth-gateway と揃えた設定

Docker のビルドは linux/amd64、CGO 無効、trimpath に揃えました。
リンク時に `-s -w` を指定します。
実行イメージは `gcr.io/distroless/base-debian12:nonroot` です。
バイナリは `/app` に配置し、実行ユーザーを明示します。
`.dockerignore` の設定と PAT の除外も揃えました。

Cloud Run と Job は CPU とメモリの指定を省きます。
gateway と同じく provider と API の既定を使います。
イメージ、client、client_version の差分も同じく無視します。
Artifact Registry のリソース定義は既に同じ形式です。

Go は go.mod が要求する 1.27.0 を使います。
gateway の 1.25.5 には下げず、bookworm の指定だけ揃えました。
Artifact Registry は新規のため `us-central1` に置きます。
gateway の `asia-northeast1` は既存 Registry の維持が理由です。

API は既存の 8080 番と環境変数による設定を維持します。
gateway の 8800 番や YAML ファイル読込は採用しません。
Job は現行 migrate CLI が要求する `up` 引数を渡します。
SQL ファイルも `/app/migrations` に配置します。
DB 接続確認用の startup probe と生存確認用の probe は維持します。
Job の並列数 1、再試行 0、制限時間 600 秒も維持します。
API と Job の権限分離も維持します。
Job に gateway 内部トークンを読む権限を渡す必要がないためです。
Secret のバージョン番号も固定します。
新しいバージョンへの切替をデプロイ時に指定するためです。

初回の `deploy_workloads=false` と空の image の検証は維持します。
汎用の hello イメージには migrate バイナリがありません。
実際のアプリイメージを指定するまでは Job を作成しません。

## リソースを変更しない検証

API と migrate は起動時の DB 接続を最大 180 秒待ちます。
`DB_CONNECT_TIMEOUT` で接続待ち時間を変更できます。
各接続試行は最大 5 秒で、失敗後に 1 秒待って再試行します。
時間切れの場合は終了コード 1 で終了します。

API は GORM の自動 Ping を無効にし、HTTP 起動前に接続を待ちます。
startup probe の待機枠は 240 秒です。
HTTP 起動後は `/readyz` が DB の接続を確認します。
Job は migration を始める前の接続だけを再試行します。
migration の失敗と Job の失敗は再試行しません。

```sh
terraform -chdir=infra init -backend=false -input=false -lockfile=readonly
terraform -chdir=infra fmt -check
terraform -chdir=infra validate
go test -race ./...
go vet ./...
```

`init -backend=false` は state バケットへ接続しません。
`validate` は GCP の認証・権限や実際の接続を確認しません。
provider の取得にはネットワークが必要です。

## 承認後の初回デプロイ

1. プロジェクトと課金設定を用意します。
   Terraform はプロジェクトや課金設定を作りません。
   state バケットも管理外です。
   `versions.tf` のバケット名とプロジェクトを合わせます。

   ```sh
   gcloud storage buckets create gs://react-app-playground-backend-tfstate \
     --project react-app-playground-backend --location us-central1 \
     --uniform-bucket-level-access --public-access-prevention
   gcloud storage buckets update gs://react-app-playground-backend-tfstate --versioning
   terraform -chdir=infra init -reconfigure
   ```

2. `deploy_workloads=false` のまま基盤を作ります。
   VPC、共有 DB との peering、Secret、Artifact Registry、WIF が対象です。
   API と Job はまだ作りません。
   `<db_network>` は shared-infra の `terraform output -raw db_network` です。

   ```sh
   terraform -chdir=infra plan -var=shared_db_network=<db_network> -out=bootstrap.tfplan
   terraform -chdir=infra apply bootstrap.tfplan
   ```

3. gateway のアプリ設定と一致する内部トークンを登録します。
   アプリ ID は `react-app-playground` です。
   内部トークンの値は Terraform に渡しません。
   gateway の URL とフロントの origin も確定します。

   ```sh
   gcloud secrets versions add playground-auth-gateway-internal-token \
     --project react-app-playground-backend --data-file=/path/to/token-file
   ```

   トークンファイルには改行を含めないでください。
   Secret Manager が返すバージョン番号を控えます。
   `/internal/session` は `X-Internal-Token` でアプリを識別します。
   API からの照会に `X-App-Id` は不要です。
   Workers から `/auth/*` への proxy は `X-App-Id` が必要です。

4. リポジトリの Dockerfile で linux/amd64 イメージを作ります。
   API と Job は同じイメージの digest を使います。

   ```sh
   docker build --platform linux/amd64 \
     -t us-central1-docker.pkg.dev/react-app-playground-backend/react-app-playground-backend/api:TAG .
   ```

   Artifact Registry の認証と push は別途実行します。
   `production.tfvars.example` を `production.tfvars` にコピーします。
   image、origin 2 件、内部トークンのバージョン番号、共有 DB の 3 変数を設定します。
   サンプルの仮値はそのまま使わないでください。

   ```sh
   terraform -chdir=infra plan -var-file=production.tfvars -out=production.tfplan
   terraform -chdir=infra apply production.tfplan
   ```

5. フロントの接続先を切り替える前に migrate Job を実行します。
   Terraform は Job を作成するだけで、migration を実行しません。

   ```sh
   gcloud run jobs execute react-app-playground-backend-migrate \
     --project react-app-playground-backend --region us-central1 --wait
   ```

   `/readyz` は DB の接続確認だけを行います。
   migration の完了や gateway の認証成功は確認しません。
   migration、実際のログイン、業務 API を確認してから切り替えます。

## 運用前の制約

DB VM とそのバックアップの扱いは、shared-infra の README にあります。
API と Job には削除保護を設定しています。

DB 接続は peering 経由の内部 IP で `sslmode=disable` を使います。
DATABASE_URL の Secret のバージョンは固定です。
shared-infra で新しいバージョンを作ったら、`shared_database_url_secret_version` を替えて apply します。

イメージの更新は auth-gateway と同じく CI が管理します。
Terraform の `image` はワークロード作成時に使います。
既存ワークロードのイメージ差分は Terraform が無視します。
API と Job の両方を CI で同じ digest に更新してください。
ワークロードを再作成する場合は最新の digest を指定します。
GitHub Actions の deploy workflow は今回の作業に含めていません。
WIF は workflow が追加されるまで利用されません。

Cloudflare の proxy、DNS、gateway のアプリ設定は管理外です。
旧 tanstack-playground のリソース削除も本構成には含めません。
