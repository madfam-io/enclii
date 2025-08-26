module github.com/madfam/enclii/packages/cli

go 1.22

require (
	github.com/madfam/enclii/packages/sdk-go v0.0.0
	github.com/spf13/cobra v1.8.0
	github.com/spf13/viper v1.18.2
	github.com/sirupsen/logrus v1.9.3
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/madfam/enclii/packages/sdk-go => ../sdk-go