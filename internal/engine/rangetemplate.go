package engine

import (
	"context"
	_ "embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// HowToFillName is the instruction file written next to the template.
const HowToFillName = "HOW-TO-FILL.md"

//go:embed howto/HOW-TO-FILL.md
var howToFill []byte

// openFile is os.OpenFile; tests replace it to make writes fail.
var openFile = os.OpenFile

// MappedSkip is a template file that was not written because another file of the folder already maps to
// the same node (a file with a bet size, other letter case or extra blanks in its name).
type MappedSkip struct {
	Name     string // the template file that was not written
	Existing string // the file (path below the folder) that maps to the same node
}

// existingByKey lists the .txt files below dir (as LoadRanges does), as paths relative to dir, by the node they map to.
func existingByKey(dir string) (map[RangeKey]string, error) {
	paths, err := txtFiles(context.Background(), os.DirFS(dir))
	if err != nil {
		return nil, err
	}
	out := map[RangeKey]string{}
	for _, p := range paths {
		if key, ok := ParseFilename(fileStem(baseName(p))); ok {
			if _, taken := out[key]; !taken {
				out[key] = p
			}
		}
	}
	return out, nil
}

// InitTemplate writes an empty range file for every standard name, plus HOW-TO-FILL.md, into dir (created if
// needed). Existing files are never touched or followed: a name that exists (even as a dangling symbolic
// link) is skipped. A standard name whose node is already mapped by another file of the folder is not
// written either and is reported in skippedMapped. Names carry the .txt suffix.
//
// A file that cannot be written completely is removed again; on an error the lists hold what was done so far.
func InitTemplate(dir string) (created, skipped []string, skippedMapped []MappedSkip, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, nil, err
	}
	existing, err := existingByKey(dir)
	if err != nil {
		return nil, nil, nil, err
	}
	write := func(name string, data []byte) error {
		path := filepath.Join(dir, name)
		f, err := openFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			skipped = append(skipped, name)
			return nil
		}
		if err != nil {
			return err
		}
		_, werr := f.Write(data)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			_ = os.Remove(path)
			return werr
		}
		created = append(created, name)
		return nil
	}
	for _, k := range StandardSet() {
		name := CanonicalName(k) + ".txt"
		if other, ok := existing[k]; ok && other != name {
			skippedMapped = append(skippedMapped, MappedSkip{name, other})
			continue
		}
		if err := write(name, nil); err != nil {
			return created, skipped, skippedMapped, err
		}
	}
	err = write(HowToFillName, howToFill)
	return created, skipped, skippedMapped, err
}
