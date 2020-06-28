module gitlab.faza.io/services/finance

go 1.13

require (
	github.com/Netflix/go-env v0.0.0-20200312172415-986dfe862277
	github.com/golang/protobuf v1.3.5
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/joho/godotenv v1.3.0
	github.com/klauspost/compress v1.10.3 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.5.0
	github.com/shopspring/decimal v0.0.0-20200227202807-02e2044944cc
	github.com/stretchr/testify v1.4.0
	gitlab.faza.io/go-framework/acl v0.0.3
	gitlab.faza.io/go-framework/logger v0.0.12
	gitlab.faza.io/go-framework/mongoadapter v0.1.4
	gitlab.faza.io/protos/finance-proto v0.0.2-rs12
	gitlab.faza.io/protos/order v0.0.83-rs4
	gitlab.faza.io/protos/payment-transfer-proto v0.0.14
	gitlab.faza.io/protos/user v0.0.49
	gitlab.faza.io/services/user-app-client v0.0.24
	go.mongodb.org/mongo-driver v1.3.4
	go.uber.org/atomic v1.5.1 // indirect
	go.uber.org/multierr v1.4.0 // indirect
	go.uber.org/zap v1.14.0
	golang.org/x/crypto v0.0.0-20200221231518-2aa609cf4a9d // indirect
	golang.org/x/lint v0.0.0-20200130185559-910be7a94367 // indirect
	golang.org/x/sys v0.0.0-20200223170610-d5e6a3e2c0ae // indirect
	golang.org/x/tools v0.0.0-20200221224223-e1da425f72fd // indirect
	google.golang.org/grpc v1.28.1
	gopkg.in/yaml.v2 v2.2.7 // indirect
	honnef.co/go/tools v0.0.1-2020.1.3 // indirect

)
