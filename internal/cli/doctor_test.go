package cli

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/amterp/kan/internal/service"
)

func TestDoctor_BoardIDSelectsTheSameBoardAsItsName(t *testing.T) {
	root := openFixtureProject(t, "v15")
	before := snapshotProject(t, root)
	byName := captureStdout(t, func() { runDoctor("main", false, false, true, false, false) })
	byID := captureStdout(t, func() { runDoctor("board-test-123", false, false, true, false, false) })
	var nameReport, idReport service.DiagnosticReport
	if err := json.Unmarshal([]byte(byName), &nameReport); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(byID), &idReport); err != nil {
		t.Fatal(err)
	}
	if len(idReport.Boards) != 1 || idReport.Boards[0].Name != "main" || !reflect.DeepEqual(nameReport, idReport) {
		t.Fatalf("board name and ID differ: %s\n%s", byName, byID)
	}
	assertProjectUnchanged(t, before, snapshotProject(t, root))
}
