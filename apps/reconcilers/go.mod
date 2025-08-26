module github.com/madfam/enclii/apps/reconcilers

go 1.22

require (
	github.com/madfam/enclii/packages/sdk-go v0.0.0
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.8.0
	k8s.io/client-go v0.29.0
	k8s.io/apimachinery v0.29.0
	k8s.io/api v0.29.0
	sigs.k8s.io/controller-runtime v0.16.3
)

replace github.com/madfam/enclii/packages/sdk-go => ../../packages/sdk-go