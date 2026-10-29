// Command sdegen extracts what fits need from CCP's static data export (SDE) into static_data/fit_types.json and static_data/dogma.json.
//
//	go run ./cmd/sdegen [-sde-dir DIR | -sde-zip FILE] [-out static_data/fit_types.json] [-dogma-out static_data/dogma.json] [-pyfa DIR]
//
// With -sde-dir it reads an extracted JSONL SDE, with -sde-zip a downloaded one; without either, it downloads the latest JSONL zip to a temp file and reads it from there.
package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"time"
)

// LATEST_SDE_ZIP_URL redirects to the zip of the newest JSONL SDE build
const LATEST_SDE_ZIP_URL = "https://developers.eveonline.com/static-data/eve-online-static-data-latest-jsonl.zip"

func main() {
	sdeDir := flag.String("sde-dir", "", "an extracted JSONL SDE folder; the latest SDE is downloaded when empty")
	sdeZip := flag.String("sde-zip", "", "a downloaded JSONL SDE zip, read without extracting it")
	out := flag.String("out", "static_data/fit_types.json", "the fit types file to write")
	dogmaOut := flag.String("dogma-out", "static_data/dogma.json", "the dogma file to write")
	pyfaDir := flag.String("pyfa", "", "a Pyfa clone, whose skill= arguments the bonus skills are checked against (a dev check, only read)")
	flag.Parse()

	if err := run(*sdeDir, *sdeZip, *out, *dogmaOut, *pyfaDir); err != nil {
		fmt.Fprintln(os.Stderr, "sdegen:", err)
		os.Exit(1)
	}
}

// run opens the SDE, extracts the fit types and the dogma, and writes them to out and dogmaOut
func run(sdeDir string, zipPath string, out string, dogmaOut string, pyfaDir string) error {
	var sde fs.FS
	if sdeDir != "" {
		sde = os.DirFS(sdeDir)
	} else {
		if zipPath == "" {
			downloaded, err := downloadLatest()
			if err != nil {
				return err
			}
			defer os.Remove(downloaded)
			zipPath = downloaded
		}

		archive, err := zip.OpenReader(zipPath)
		if err != nil {
			return fmt.Errorf("opening the SDE zip: %w", err)
		}
		defer archive.Close()
		sde = archive
	}

	extract, err := Extract(sde)
	if err != nil {
		return err
	}

	text, err := marshalExtract(extract)
	if err != nil {
		return err
	}
	if err := writeAtomically(out, text); err != nil {
		return err
	}
	fmt.Printf("SDE build %d: wrote %d types to %s (%.2f MB)\n", extract.SDEBuild, len(extract.Types), out, float64(len(text))/1024/1024)

	dogma, source, err := ExtractDogma(sde, extract)
	if err != nil {
		return err
	}
	dogmaText, err := marshalDogma(dogma)
	if err != nil {
		return err
	}
	if err := writeAtomically(dogmaOut, dogmaText); err != nil {
		return err
	}
	fmt.Printf("wrote %d types, %d attributes, %d effects, %d skills, and %d bonus attributes to %s (%.2f MB)\n", len(dogma.Types), len(dogma.Attributes), len(dogma.Effects), len(dogma.Skills), len(dogma.BonusSkills), dogmaOut, float64(len(dogmaText))/1024/1024)
	for _, line := range source.modeReport {
		fmt.Println("  modes:", line)
	}
	reportBonuses(source)
	if pyfaDir != "" {
		if err := reportPyfa(source, pyfaDir); err != nil {
			return fmt.Errorf("reading Pyfa: %w", err)
		}
	}
	return nil
}

// writeAtomically writes beside the target first, so a failure never leaves a truncated file
func writeAtomically(path string, text []byte) error {
	temp := path + ".new"
	if err := os.WriteFile(temp, text, 0o644); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// downloadLatest saves the latest SDE zip to a temp file and returns its path
func downloadLatest() (string, error) {
	fmt.Println("Downloading", LATEST_SDE_ZIP_URL)
	client := &http.Client{Timeout: 10 * time.Minute}
	request, err := http.NewRequest(http.MethodGet, LATEST_SDE_ZIP_URL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "eve-gosleuth sdegen (+https://github.com/lynkfox/go_eve_sleuth)")
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("downloading the SDE: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading the SDE: %s", response.Status)
	}

	file, err := os.CreateTemp("", "sde-*.zip")
	if err != nil {
		return "", err
	}
	size, err := io.Copy(file, response.Body)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(file.Name())
		return "", fmt.Errorf("saving the SDE: %w", err)
	}
	fmt.Printf("Downloaded %.1f MB\n", float64(size)/1024/1024)
	return file.Name(), nil
}
