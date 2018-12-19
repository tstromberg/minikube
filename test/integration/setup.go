import (
	"context"
	"regexp"
	"testing"

	"k8s.io/minikube/test/integration/util"
)

var driverFlagRe = regexp.MustCompile(`vm-driver=([\w-]+)`)

func SetupWithTimeout(t *testing.T) (context.Context, util.MinikubeRunner, util.KubectlRunner) {
	t.Helper()

	driver := ""
	matches := driverFlagRe.FindStringSubmatch(*startArgs)
	if len(matches) != 0 {
		driver = matches[1]
	}

	mk := util.NewMinikubeRunner(&util.Config{
		Args:       *args,
		Runtime:    "docker",
		VMDriver:   driver,
		BinaryPath: *binaryPath,
		StartArgs:  *startArgs,
		T:          t,
	})

	return ctx, mk, kc
}
