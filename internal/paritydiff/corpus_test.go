package paritydiff

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMaterializePosixExtras(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix_extras needs tab file names and symlinks")
	}
	probe := t.TempDir()
	if err := os.Symlink("x", filepath.Join(probe, "probe")); err != nil {
		t.Skipf("cannot create symlinks here: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "posix_extras")
	if err := Materialize("../../testdata/parity/posix_extras", dst); err != nil {
		t.Fatal(err)
	}
	ranges := filepath.Join(dst, "ranges")
	for _, want := range []string{
		"HJ\tvs\tBTN\t3bet\tcall.txt",
		"link/BB vs SB open call.txt",
		"0link/BTN OPEN 2bb.txt",
		"HJ open.txt",
	} {
		if _, err := os.Stat(filepath.Join(ranges, want)); err != nil {
			t.Errorf("%q: %v", want, err)
		}
	}
	if _, err := os.Stat(filepath.Join(ranges, "HJ__TAB__vs__TAB__BTN__TAB__3bet__TAB__call.txt")); !os.IsNotExist(err) {
		t.Errorf("placeholder should be renamed away, got %v", err)
	}
	for _, link := range []string{"link/BB vs SB open call.txt", "0link"} {
		fi, err := os.Lstat(filepath.Join(ranges, link))
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s should be a symlink (err %v)", link, err)
		}
	}
	for _, sub := range []string{"hands", "ranges_decoy"} {
		if _, err := os.Stat(filepath.Join(dst, sub)); err != nil {
			t.Error(err)
		}
	}
}
