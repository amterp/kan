package resolver

import (
	"os"
	"testing"

	"github.com/amterp/kan/testutil"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.RunWithIsolatedHome(m))
}
