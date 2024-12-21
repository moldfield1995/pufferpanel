package steamgamedl

import (
	"bufio"
	"fmt"
	"github.com/mholt/archiver/v3"
	"github.com/pufferpanel/pufferpanel/v3"
	"github.com/pufferpanel/pufferpanel/v3/config"
	"github.com/pufferpanel/pufferpanel/v3/utils"
	"github.com/spf13/cast"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var downloader sync.Mutex

const SteamMetadataServerLink = "https://media.steampowered.com/client/"

func init() {
}

type SteamGameDl struct {
	AppId     string
	Username  string
	Password  string
	ExtraArgs []string
}

func (c SteamGameDl) Run(args pufferpanel.RunOperatorArgs) pufferpanel.OperationResult {
	env := args.Environment

	env.DisplayToConsole(true, "Downloading game from Steam")

	rootBinaryFolder := config.BinariesFolder.Value()

	err := downloadDD(rootBinaryFolder, config.DepotDownloaderVersion.Value())
	if err != nil {
		return pufferpanel.OperationResult{Error: err}
	}

	err = downloadMetadata(env)
	if err != nil {
		return pufferpanel.OperationResult{Error: err}
	}

	//generate a login id
	//this is a 32-bit id, which Steam derives from private IP
	//as such, we can kinda send anything we want
	//our approach will be we hash the server id
	loginId := cast.ToString(rand.Int31())

	manifestFolder := filepath.Join(env.GetRootDirectory(), ".manifest")
	_ = os.RemoveAll(manifestFolder)

	cmdArgs := []string{"-app", c.AppId, "-dir", manifestFolder, "-loginid", loginId, "-manifest-only"}
	if c.Username != "" {
		cmdArgs = append(cmdArgs, "-username", c.Username, "-remember-password")
		if c.Password != "" {
			cmdArgs = append(cmdArgs, "-password", c.Password)
		}
	}
	cmdArgs = append(cmdArgs, c.ExtraArgs...)

	ch := make(chan int, 1)
	steps := pufferpanel.ExecutionData{
		Command:   filepath.Join(rootBinaryFolder, "depotdownloader", DepotDownloaderBinary),
		Arguments: cmdArgs,
		Callback: func(exitCode int) {
			ch <- exitCode
		},
	}
	err = env.Execute(steps)
	if err != nil {
		return pufferpanel.OperationResult{Error: err}
	}
	exitCode := <-ch
	if exitCode != 0 {
		err = fmt.Errorf("depotdownloader exited with non-zero code %d", exitCode)
		return pufferpanel.OperationResult{Error: err}
	}

	//download game itself now
	cmdArgs = []string{"-app", c.AppId, "-dir", env.GetRootDirectory(), "-loginid", loginId, "-validate"}
	if c.Username != "" {
		cmdArgs = append(cmdArgs, "-username", c.Username, "-remember-password")
		if c.Password != "" {
			cmdArgs = append(cmdArgs, "-password", c.Password)
		}
	}

	if c.ExtraArgs != nil && len(c.ExtraArgs) > 0 {
		cmdArgs = append(cmdArgs, c.ExtraArgs...)
	}

	steps = pufferpanel.ExecutionData{
		Command:   filepath.Join(rootBinaryFolder, "depotdownloader", DepotDownloaderBinary),
		Arguments: cmdArgs,
		Callback: func(exitCode int) {
			ch <- exitCode
		},
		Environment: map[string]string{
			"TERM": "pufferpanel", //we use a fake TERM because DD will use a display that is not supported by us directly
		},
	}
	err = env.Execute(steps)
	if err != nil {
		return pufferpanel.OperationResult{Error: err}
	}
	exitCode = <-ch
	if exitCode != 0 {
		err = fmt.Errorf("depotdownloader exited with non-zero code %d", exitCode)
		return pufferpanel.OperationResult{Error: err}
	}

	//for each file we download, we need to just... chmod +x the files
	//we rely on the manifests for this
	manifests, err := os.ReadDir(manifestFolder)
	if err != nil {
		return pufferpanel.OperationResult{Error: err}
	}
	for _, manifest := range manifests {
		if manifest.Type().IsDir() || !strings.HasSuffix(manifest.Name(), ".txt") {
			continue
		}
		err = walkManifest(env.GetRootDirectory(), manifest.Name())
		if err != nil {
			return pufferpanel.OperationResult{Error: err}
		}
	}

	return pufferpanel.OperationResult{Error: nil}
}

func downloadMetadata(env pufferpanel.Environment) error {
	response, err := pufferpanel.HttpGet(SteamMetadataLink)
	defer utils.CloseResponse(response)
	if err != nil {
		return err
	}

	metadataName, err := Parse(DownloadOs, response.Body)
	utils.CloseResponse(response)

	if err != nil {
		return err
	}

	err = os.RemoveAll(filepath.Join(env.GetRootDirectory(), ".steam"))
	if err != nil {
		return err
	}

	err = pufferpanel.HttpExtract(SteamMetadataServerLink+metadataName, filepath.Join(env.GetRootDirectory(), ".steam"), archiver.DefaultZip)
	if err != nil {
		return err
	}

	return err
}

func walkManifest(folder, filename string) error {
	file, err := os.Open(filepath.Join(folder, ".manifest", filename))
	defer utils.Close(file)
	if err != nil {
		return err
	}
	data := bufio.NewScanner(file)
	skipCounter := 8
	for data.Scan() {
		line := data.Text()
		if skipCounter > 0 {
			skipCounter--
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 5 || parts[0] == "Size" {
			continue
		}
		if len(parts) > 5 {
			//the filename at the end has spaces, we need to consolidate
			parts[4] = strings.Join(parts[5:], " ")
			parts = parts[0:5]
		}

		//we will only work on 0 files, because this mean no other flags were told
		if parts[3] == "0" {
			fileToUpdate := parts[4]
			_ = os.Chmod(filepath.Join(folder, fileToUpdate), 0755)
		}
	}

	return nil
}
