package lambda

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	sparta "github.com/mweagle/Sparta"
	spartaCast "github.com/mweagle/SpartaCast/lambda/spartacast"
)

func testFile(filename string) string {
	return filepath.Join(".", "media", filename)
}
func TestMarkdownEpisodeParse(t *testing.T) {
	data, _ := ioutil.ReadFile("episode1.md")
	dataBytes := bytes.NewReader(data)
	logger, _ := sparta.NewLogger("info")

	configEntry := Item{}
	specErr := spartaCast.ParseSpartaConfigSpec(dataBytes, &configEntry, logger)

	if specErr != nil {
		t.Fatalf("Failed to parse: %v", specErr)
	}
	ret, _ := json.MarshalIndent(configEntry, "", " ")

	t.Logf("Episode: \n%s\n", configEntry)
}

func TestMarkdownFeedParse(t *testing.T) {
	data, _ := ioutil.ReadFile("feed.md	")
	dataBytes := bytes.NewReader(data)
	logger, _ := sparta.NewLogger("info")

	configEntry := Feed{}
	specErr := spartaCast.ParseSpartaConfigSpec(dataBytes, &configEntry, logger)

	if specErr != nil {
		t.Fatalf("Failed to parse: %v", specErr)
	}
	t.Logf("Feed: \n%s\n", configEntry)
}
