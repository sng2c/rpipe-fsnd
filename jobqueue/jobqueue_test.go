package jobqueue

import (
	"errors"
	"fmt"
	"os"
	"path"
	"testing"
)

func TestJob_FilePath(t *testing.T) {

	tmpbase := os.TempDir()
	tmp := path.Join(tmpbase, "mytesttmp")

	os.RemoveAll(tmp)

	err := MkdirAll(tmp, 0777)
	if err != nil {
		t.Error(err)
	}

	stat, err := os.Stat(tmp)
	if err != nil {
		t.Error(err)
	}
	perm := stat.Mode().Perm()
	if perm != 0777 {
		t.Error(errors.New(fmt.Sprintf("is %o", perm)))
	} else {
		t.Logf("OK")
	}

}
