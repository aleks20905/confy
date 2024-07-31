package confy

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const updateWarning = `!!!!!!!!!!
! WARNING: .%sinf0 was probably updated,
! Check and update %s as necessary
! and remove the last "deprecated" paragraph to disable this message!
!!!!!!!!!!
`

const configHeader = `# %s configuration
# 
# This config has https://github.com/schachmat/ingo syntax.
# Empty lines or lines starting with # will be ignored.
# All other lines must look like "KEY=VALUE" (without the quotes).
# The VALUE must not be enclosed in quotes as well!
`

var openOrCreate = os.OpenFile

// Parse should be called by the user instead of `flag.Parse()` after all flags

func Parse(appName string) error {
	if flag.Parsed() {
		return fmt.Errorf("flags have been parsed already")
	}

	cPath, err := getConfigFilePath(appName)
	if err != nil {
		return err
	}

	oldConf, err := ioutil.ReadFile(cPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to read %s config file %v: %v", appName, cPath, err)
	}

	obsoleteKeys := parseConfig(bytes.NewReader(oldConf))
	if len(obsoleteKeys) > 0 {
		log.Printf(updateWarning, appName, cPath)
	}

	newConf := new(bytes.Buffer)
	fmt.Fprintf(newConf, configHeader, appName)
	saveConfig(newConf, obsoleteKeys)

	if !bytes.Equal(oldConf, newConf.Bytes()) {
		if err := ioutil.WriteFile(cPath, newConf.Bytes(), 0666); err != nil {
			return fmt.Errorf("failed to write %s: %v", cPath, err)
		}
	}

	flag.Parse()
	return nil
}

func getConfigFilePath(appName string) (string, error) {
	envname := strings.ToUpper(appName) + "INF0"
	cPath := os.Getenv(envname)
	if cPath == "" {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("%v\nYou can set the environment variable %s to point to your config file as a workaround", err, envname)
		}
		cPath = filepath.Join(usr.HomeDir, "."+strings.ToLower(appName)+"inf0")
	}
	return cPath, nil
}

func parseConfig(r io.Reader) map[string]string {
	obsKeys := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		i := strings.IndexAny(line, "=:")
		if i == -1 {
			continue
		}
		key, val := strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])

		if err := flag.Set(key, val); err != nil {
			obsKeys[key] = val
		}
	}
	return obsKeys
}

func saveConfig(w io.Writer, obsKeys map[string]string) {
	deduped := make(map[flag.Value]flag.Flag)
	flag.VisitAll(func(f *flag.Flag) {
		if cur, ok := deduped[f.Value]; !ok || utf8.RuneCountInString(f.Name) > utf8.RuneCountInString(cur.Name) {
			deduped[f.Value] = *f
		}
	})
	flag.VisitAll(func(f *flag.Flag) {
		if cur, ok := deduped[f.Value]; ok && cur.Name == f.Name {
			_, usage := flag.UnquoteUsage(f)
			usage = strings.Replace(usage, "\n    \t", "\n# ", -1)
			fmt.Fprintf(w, "\n# %s (default %v)\n", usage, f.DefValue)
			fmt.Fprintf(w, "%s=%v\n", f.Name, f.Value.String())
		}
	})

	if len(obsKeys) > 0 {
		fmt.Fprintln(w, "\n\n# The following options are probably deprecated and not used currently!")
		for key, val := range obsKeys {
			fmt.Fprintf(w, "%v=%v\n", key, val)
		}
	}
}
