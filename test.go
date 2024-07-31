package confy

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	// Create a temporary directory to store the config file
	dir, err := ioutil.TempDir("", "confy_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Set environment variable to use the temp config file
	configFilePath := filepath.Join(dir, ".myappconfig")
	os.Setenv("MYAPPRC", configFilePath)

	// Create a temporary config file
	configContent := `port=8080
		host=localhost
		debug=false
		log-level=info`
	if err := ioutil.WriteFile(configFilePath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	// Define the flags
	var (
		port     = flag.Int("port", 0, "Port to run the server on")
		host     = flag.String("host", "", "Host address for the server")
		debug    = flag.Bool("debug", false, "Enable debug mode")
		logLevel = flag.String("log-level", "", "Log level for the application")
	)

	// Reset the command-line flags for testing
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Call the Parse function
	if err := Parse("myapp"); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Verify the flags are set correctly
	if *port != 8080 {
		t.Errorf("Expected port 8080, got %d", *port)
	}
	if *host != "localhost" {
		t.Errorf("Expected host 'localhost', got '%s'", *host)
	}
	if *debug != false {
		t.Errorf("Expected debug false, got %v", *debug)
	}
	if *logLevel != "info" {
		t.Errorf("Expected log-level 'info', got '%s'", *logLevel)
	}
}
