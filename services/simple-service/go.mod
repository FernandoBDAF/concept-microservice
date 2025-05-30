module github.com/FBDAF/microservices/services/simple-service

go 1.23

toolchain go1.24.0

require (
	github.com/FBDAF/microservices/services/common/config v0.0.0
	github.com/FBDAF/microservices/services/common/errors v0.0.0
	github.com/FBDAF/microservices/services/common/logging v0.0.0
	github.com/FBDAF/microservices/services/common/metrics v0.0.0
	github.com/FBDAF/microservices/services/common/middleware v0.0.0-00010101000000-000000000000
	github.com/FBDAF/microservices/services/common/models v0.0.0
	github.com/gin-gonic/gin v1.10.1
	go.uber.org/zap v1.27.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bytedance/sonic v1.11.6 // indirect
	github.com/bytedance/sonic/loader v0.1.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cloudwego/base64x v0.1.4 // indirect
	github.com/cloudwego/iasm v0.2.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.20.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.7 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.2 // indirect
	github.com/prometheus/client_golang v1.18.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.45.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/arch v0.8.0 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/FBDAF/microservices/services/common/config => ../common/config
	github.com/FBDAF/microservices/services/common/errors => ../common/errors
	github.com/FBDAF/microservices/services/common/logging => ../common/logging
	github.com/FBDAF/microservices/services/common/metrics => ../common/metrics
	github.com/FBDAF/microservices/services/common/middleware => ../common/middleware
	github.com/FBDAF/microservices/services/common/models => ../common/models
	github.com/FBDAF/microservices/services/common/security => ../common/security
)
