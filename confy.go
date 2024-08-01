// Package confy provides a drop-in replacement for flag.Parse() with flags
// persisted in a user editable configuration file.
package confy

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"
	"path"
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
# Empty lines or lines starting with # will be ignored.
# All other lines must look like "KEY=VALUE" (without the quotes).
# The VALUE must not be enclosed in quotes as well!
`

var openOrCreate = os.OpenFile

func Parse(appName string) error {
	if flag.Parsed() {
		return fmt.Errorf("flags have been parsed already")
	}

	cPath, err := getConfigPath(appName)
	if err != nil {
		return err
	}

	cf, err := openOrCreate(cPath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("unable to open %s config file %v for reading and writing: %v", appName, cPath, err)
	}
	defer cf.Close()

	// read config to buffer and parse
	oldConf := new(bytes.Buffer)
	obsoleteKeys := parseConfig(io.TeeReader(cf, oldConf))
	if len(obsoleteKeys) > 0 {
		fmt.Fprintf(os.Stderr, updateWarning, appName, cPath)
	}

	// write updated config to another buffer
	newConf := new(bytes.Buffer)
	fmt.Fprintf(newConf, configHeader, appName)
	saveConfig(newConf, obsoleteKeys)

	// only write the file if it changed
	if !bytes.Equal(oldConf.Bytes(), newConf.Bytes()) {
		if ofs, err := cf.Seek(0, 0); err != nil || ofs != 0 {
			return fmt.Errorf("failed to seek to beginning of %s: %v", cPath, err)
		} else if err = cf.Truncate(0); err != nil {
			return fmt.Errorf("failed to truncate %s: %v", cPath, err)
		} else if _, err = newConf.WriteTo(cf); err != nil {
			return fmt.Errorf("failed to write %s: %v", cPath, err)
		}
	}

	flag.Parse()
	return nil
}

func getConfigPath(appName string) (string, error) {
	envname := strings.ToUpper(appName) + "INF0"
	cPath := os.Getenv(envname)
	if cPath == "" {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("%v\nYou can set the environment variable %s to point to your config file as a workaround", err, envname)
		}
		cPath = path.Join(usr.HomeDir, "."+strings.ToLower(appName)+"inf0")
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

		// find first assignment symbol and parse key, val
		i := strings.IndexAny(line, "=:")
		if i == -1 {
			continue
		}
		key, val := strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])

		if err := flag.Set(key, val); err != nil {
			obsKeys[key] = val
			continue
		}
	}
	return obsKeys
}

func saveConfig(w io.Writer, obsKeys map[string]string) {
	// find flags pointing to the same variable. We will only write the longest
	// named flag to the config file, the shorthand version is ignored.
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

	// if we have obsolete keys left from the old config, preserve them in an
	// additional section at the end of the file
	if obsKeys != nil && len(obsKeys) > 0 {
		fmt.Fprintln(w, "\n\n# The following options are probably deprecated and not used currently!")
		for key, val := range obsKeys {
			fmt.Fprintf(w, "%v=%v\n", key, val)
		}
	}
}
