# react-app-playground-backend

React app playground の customers、deals、invoices、tasks API を提供する Go backend です。業務データは auth-gateway の `user_id` 単位で分離され、スキーマは `migrations/` の SQL が正です。

## 開発

```sh
cp .env.example .env
set -a; . ./.env; set +a
make db-up
make migrate-up
go run ./cmd/api
```

auth-gateway は別 repository で起動し、frontend は `VITE_USE_MSW=false pnpm --filter @playground/web dev` で起動します。Vite の `/api` と `/auth` proxy により同一 origin で接続します。

Migration は `make migrate-up`、`make migrate-down`（直前の1件のみ）、`make migrate-version` で操作します。CIと `make verify` は `make migration-roundtrip` で同じSQL migrationの up -> down 1 -> up を実行し、`make integration` でPostgreSQL統合テストを実行します。logout は auth-gateway の POST `/auth/logout` を使い、GET logout は状態を変更しません。

## API

`/healthz`、`/readyz` と、現在 frontend が利用する customers / deals / invoices / tasks の endpoint だけを提供します。projects API、業務データの server-side pagination、OpenAPI、Kubernetes 配置は含みません。

## アプリ別の認証接続

このバックエンドはgatewayの `/internal/react-app-playground/session` を呼びます。`AUTH_GATEWAY_INTERNAL_TOKEN` はgatewayの `applications` 内の `react-app-playground` の `internal_token` と一致させてください。他アプリのトークンは使用できません。

開発用フロントのURLは `http://react.localtest.me:5174` です。`PUBLIC_ORIGIN` も同じ値にします。gatewayの接続先は `http://localhost:8800` です。業務DBの変更やmigrationは今回の接続先変更には不要です。

開発時の `SERVER_ADDR` は `:18080` です。ZITADELが使用するホスト側ポート `8080` との競合を避けます。
