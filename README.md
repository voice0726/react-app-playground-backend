# react-app-playground-backend

React app playground の customers、deals、invoices、tasks、tickets API を提供する Go backend です。業務データは auth-gateway の `user_id` 単位で分離され、スキーマは `migrations/` の SQL が正です。

## 開発

```sh
cp .env.example .env
set -a; . ./.env; set +a
make db-up
make migrate-up
go run ./cmd/api
```

auth-gateway は別 repository で起動し、frontend は `VITE_USE_MSW=false pnpm --filter @playground/web dev` で起動します。Vite の `/api` と `/auth` proxy により同一 origin で接続します。logout は auth-gateway の POST `/auth/logout` を使い、GET logout は状態を変更しません。

Migration は `make migrate-up`、`make migrate-down`（直前の1件のみ）、`make migrate-version` で操作します。`make verify` は fmt → vet → test → migration-roundtrip（up → down 1 → up）→ integration（PostgreSQL 統合テスト）→ build を順に実行します。CI は同じ検証に加えて gofmt の差分確認と `docker build` を実行します。

## API

`/healthz` と `/readyz` は認証なしです。業務 API は `/api` 配下にあり、auth-gateway のセッション確認と Origin チェックを通ります。

| リソース | endpoint |
| --- | --- |
| customers | `GET/POST /api/customers`、`GET/PATCH/DELETE /api/customers/:id` |
| deals | `GET/POST /api/deals`、`GET/PATCH /api/deals/:id` |
| invoices | `GET/POST /api/invoices`、`GET/PATCH /api/invoices/:id` |
| tasks | `GET/POST /api/tasks`、`PATCH/DELETE /api/tasks/:id` |
| tickets | `GET/POST /api/tickets`、`GET/PUT/PATCH/DELETE /api/tickets/:id`、`POST /api/tickets/:id/comments`、`PUT/DELETE /api/tickets/:id/comments/:commentId` |

projects API、業務データの server-side pagination、OpenAPI、Kubernetes 配置は含みません。

## アプリ別の認証接続

このバックエンドはgatewayの `/internal/session` を呼びます。`AUTH_GATEWAY_INTERNAL_TOKEN` はgatewayの `applications` 内の `react-app-playground` の `internal_token` と一致させてください。他アプリのトークンは使用できません。内部照会ではトークンでアプリを識別するため、`X-App-Id` は不要です。`/auth/*` を呼ぶフロント側の proxy では `X-App-Id: react-app-playground` が必要です。

開発用フロントのURLは `http://react.localtest.me:5174` です。`PUBLIC_ORIGIN` も同じ値にします。gatewayの接続先は `http://localhost:8800` です。業務DBの変更やmigrationは今回の接続先変更には不要です。

開発時の `SERVER_ADDR` は `:18080` です。ZITADELが使用するホスト側ポート `8080` との競合を避けます。

## GCP デプロイ準備

専用プロジェクト `react-app-playground-backend` の Terraform は [infra/README.md](infra/README.md) にあります。API と migrate Job は同じイメージを使います。Job は `/app/migrate up` を実行します。
