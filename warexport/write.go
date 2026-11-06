package warexport

import (
	"fmt"
	"os"
	"path/filepath"
)

/*
	Write puts a war's files in a folder named for it under the parent, replacing an earlier export's files and nothing else. Each goes to a temporary name first, so one that can not be written leaves the last export as it was. If one can not then replace its old copy, ie it is open elsewhere, those before it are already new: exporting again puts it right.

Parameters:
  - parent: the folder the user chose for exports
  - warName: the war's name
  - files: from Build

Returns:
  - string: the war's folder
  - error: if a file could not be written
*/
func Write(parent string, warName string, files []File) (string, error) {
	folder := filepath.Join(parent, FolderName(warName))
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", fmt.Errorf("could not make the folder: %w", err)
	}
	temps := make([]string, 0, len(files))
	removeTemps := func() {
		for _, temp := range temps {
			os.Remove(temp)
		}
	}
	for _, file := range files {
		temp := filepath.Join(folder, file.Name+".tmp")
		if err := os.WriteFile(temp, file.Data, 0o644); err != nil {
			removeTemps()
			return "", fmt.Errorf("could not write %s: %w", file.Name, err)
		}
		temps = append(temps, temp)
	}

	for _, file := range files {
		if err := os.Rename(filepath.Join(folder, file.Name+".tmp"), filepath.Join(folder, file.Name)); err != nil {
			removeTemps()
			return "", fmt.Errorf("could not replace %s: %w", file.Name, err)
		}
	}
	return folder, nil
}
