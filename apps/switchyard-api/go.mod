module github.com/madfam/enclii/apps/switchyard-api

go 1.22

require (
	github.com/gin-gonic/gin v1.10.0
	github.com/golang-jwt/jwt/v5 v5.2.0
	github.com/golang-migrate/migrate/v4 v4.17.1
	github.com/google/uuid v1.6.0
	github.com/lib/pq v1.10.9
	github.com/ory/dockertest/v3 v3.10.0
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.8.0
	github.com/spf13/viper v1.18.2
	github.com/go-playground/validator/v10 v10.16.0
	github.com/redis/go-redis/v9 v9.3.1
	github.com/prometheus/client_golang v1.17.0
	golang.org/x/time v0.5.0
	go.opentelemetry.io/otel v1.21.0
	go.opentelemetry.io/otel/exporters/jaeger v1.17.0
	go.opentelemetry.io/otel/sdk v1.21.0
	go.opentelemetry.io/otel/semconv/v1.17.0 v1.17.0
	go.opentelemetry.io/otel/trace v1.21.0
	k8s.io/client-go v0.29.0
	k8s.io/apimachinery v0.29.0
	k8s.io/api v0.29.0
	github.com/stretchr/testify v1.8.4
)