# react-app-playground-backend

React app playground の customers、deals、invoices、tasks API を提供する Go backend です。業務データは組織単位で共有され、組織メンバーの admin / editor / viewer 権限で操作を制御します。認証済みユーザーの識別には auth-gateway の `user_id` を使い、スキーマは `migrations/` の SQL が正です。

## 開発

```sh
cp .env.example .env
podman compose up -d --wait db
make migrate-up
go run ./cmd/api
```

backend のリポジトリ直下で実行してください。API と migration はカレントディレクトリの `.env` を自動で読み込むため、fish でも `source` や `export` は不要です。既存の `.env` がある場合は上書きせず編集してください。`DATABASE_URL` を起動したDBに合わせ、API用の `AUTH_GATEWAY_INTERNAL_TOKEN` は auth-gateway と同じ値に設定します。migration が必要とする設定は `DATABASE_URL` だけです。

シェルやCIで設定済みの環境変数は `.env` より優先されます（空文字を設定した場合も同様）。`.env` がない場合は環境変数だけを使用します。`.env` の読み取り・解析に失敗した場合は起動を中止します。`.env` はGitとDockerのビルド対象から除外し、共有する設定項目は `.env.example` に記載します。`make migrate-up` と `go run ./cmd/migrate up` のどちらでも同じ設定を読み込みます。

auth-gateway は別 repository で起動し、frontend は `VITE_USE_MSW=false pnpm --filter @playground/web dev` で起動します。Vite の `/api` と `/auth` proxy により同一 origin で接続します。

Migration は `make migrate-up`、`make migrate-down`（直前の1件のみ）、`make migrate-version` で操作します。CIと `make verify` は `make migration-roundtrip` で同じSQL migrationの up -> down 1 -> up を実行し、`make integration` でPostgreSQL統合テストを実行します。統合テストは `DATABASE_URL` の全テーブルを削除して状態を初期化するため、使い捨てのテスト用PostgreSQLだけを指定してください。直接実行する場合は `PLAYGROUND_INTEGRATION_ALLOW_RESET=1 DATABASE_URL=... go test -tags=integration ./test/integration` を明示します。logout は auth-gateway の POST `/auth/logout` を使い、GET logout は状態を変更しません。

## API

`/healthz`、`/readyz`、organizations と members の管理 API、および customers / deals / invoices / tasks の endpoint を提供します。組織スコープが必要な endpoint には `X-Organization-ID`（UUID）を指定します。業務一覧は `page` と `pageSize` を指定した場合に `{items,total,page,pageSize}` 形式で返り、`q`、`sort`、`order`、各リソースの status、`customerId` で絞り込めます。パラメータを指定しない場合は既存の配列形式を維持します。`GET /api/invoices/summary` と `GET /api/dashboard` は選択組織だけを集計します。

`migrations/000002_organizations` は既存の `owner_user_id` ごとに個別組織と admin メンバーを作成し、既存レコードを保持したまま組織スコープへ移行します。down migration は共有メンバー、新規組織、所有者情報を復元できない組織が存在する場合に失敗し、共有データを黙って削除しません。旧来の単独所有組織へ安全に戻せる場合だけ rollback できます。
