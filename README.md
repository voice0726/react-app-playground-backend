# react-app-playground-backend

React app playground の customers、deals、invoices、tasks、tickets API を提供する Go backend です。customers、deals、invoices、tasks は組織単位で共有され、組織メンバーの admin / editor / viewer 権限で操作を制御します。tickets は auth-gateway の `user_id` 単位で分離されます。認証済みユーザーの識別には auth-gateway の `user_id` を使い、スキーマは `migrations/` の SQL が正です。

## 開発

```sh
cp .env.example .env
podman compose up -d --wait db
make migrate-up
go run ./cmd/api
```

backend のリポジトリ直下で実行してください。API と migration はカレントディレクトリの `.env` を自動で読み込むため、fish でも `source` や `export` は不要です。既存の `.env` がある場合は上書きせず編集してください。`DATABASE_URL` を起動したDBに合わせ、API用の `AUTH_GATEWAY_INTERNAL_TOKEN` は auth-gateway と同じ値に設定します。migration が必要とする設定は `DATABASE_URL` だけです。

シェルやCIで設定済みの環境変数は `.env` より優先されます（空文字を設定した場合も同様）。`.env` がない場合は環境変数だけを使用します。`.env` の読み取り・解析に失敗した場合は起動を中止します。`.env` はGitとDockerのビルド対象から除外し、共有する設定項目は `.env.example` に記載します。`make migrate-up` と `go run ./cmd/migrate up` のどちらでも同じ設定を読み込みます。

auth-gateway は別 repository で起動し、frontend は `VITE_USE_MSW=false pnpm --filter @playground/web dev` で起動します。Vite の `/api` と `/auth` proxy により同一 origin で接続します。logout は auth-gateway の POST `/auth/logout` を使い、GET logout は状態を変更しません。

Migration は `make migrate-up`、`make migrate-down`（直前の1件のみ）、`make migrate-version` で操作します。`make verify` は fmt → vet → test → migration-roundtrip（up → down 1 → up）→ integration（PostgreSQL 統合テスト）→ build を順に実行します。CI は同じ検証に加えて gofmt の差分確認と `docker build` を実行します。統合テストは `DATABASE_URL` の全テーブルを削除して状態を初期化するため、使い捨てのテスト用PostgreSQLだけを指定してください。直接実行する場合は `PLAYGROUND_INTEGRATION_ALLOW_RESET=1 DATABASE_URL=... go test -tags=integration ./test/integration` を明示します。

## API

`/healthz` と `/readyz` は認証なしです。業務 API は `/api` 配下にあり、auth-gateway のセッション確認を通り、POST/PUT/PATCH/DELETE ではさらに Origin チェックを通ります。

| リソース | endpoint |
| --- | --- |
| organizations | `GET/POST /api/organizations`、`GET/POST /api/organizations/:id/members`、`PATCH/DELETE /api/organizations/:id/members/:userId` |
| customers | `GET/POST /api/customers`、`GET/PATCH/DELETE /api/customers/:id` |
| deals | `GET/POST /api/deals`、`GET/PATCH /api/deals/:id` |
| invoices | `GET/POST /api/invoices`、`GET /api/invoices/summary`、`GET/PATCH /api/invoices/:id` |
| dashboard | `GET /api/dashboard` |
| tasks | `GET/POST /api/tasks`、`PATCH/DELETE /api/tasks/:id` |
| tickets | `GET/POST /api/tickets`、`GET/PUT/PATCH/DELETE /api/tickets/:id`、`POST /api/tickets/:id/comments`、`PUT/DELETE /api/tickets/:id/comments/:commentId` |

組織スコープが必要な endpoint には `X-Organization-ID`（UUID）を指定します。customers / deals / invoices / tickets の一覧は `page` と `pageSize` を指定した場合に `{items,total,page,pageSize}` 形式で返ります。customers / deals / invoices は `q`、`sort`、`order`、各リソースの status、`customerId` でも絞り込め、パラメータを指定しない場合は既存の配列形式を維持します。`GET /api/invoices/summary` と `GET /api/dashboard` は選択組織だけを集計します。projects API、OpenAPI、Kubernetes 配置は含みません。

`migrations/000003_organizations` は既存の `owner_user_id` ごとに個別組織と admin メンバーを作成し、既存レコードを保持したまま組織スコープへ移行します。down migration は共有メンバー、新規組織、所有者情報を復元できない組織が存在する場合に失敗し、共有データを黙って削除しません。旧来の単独所有組織へ安全に戻せる場合だけ rollback できます。

## アプリ別の認証接続

このバックエンドはgatewayの `/internal/session` を呼びます。`AUTH_GATEWAY_INTERNAL_TOKEN` はgatewayの `applications` 内の `react-app-playground` の `internal_token` と一致させてください。他アプリのトークンは使用できません。内部照会ではトークンでアプリを識別するため、`X-App-Id` は不要です。`/auth/*` を呼ぶフロント側の proxy では `X-App-Id: react-app-playground` が必要です。

開発用フロントのURLは `http://react.localtest.me:5174` です。`PUBLIC_ORIGIN` も同じ値にします。gatewayの接続先は `http://localhost:8800` です。業務DBの変更やmigrationは今回の接続先変更には不要です。

開発時の `SERVER_ADDR` は `:18080` です。ZITADELが使用するホスト側ポート `8080` との競合を避けます。

## GCP デプロイ準備

専用プロジェクト `react-app-playground-backend` の Terraform は [infra/README.md](infra/README.md) にあります。API と migrate Job は同じイメージを使います。Job は `/app/migrate up` を実行します。
