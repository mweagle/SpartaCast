package lambda

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	sparta "github.com/mweagle/Sparta"
	spartaCast "github.com/mweagle/SpartaCast/lambda/spartacast"
)

func testFile(filename string) string {
	return filepath.Join(".", "test", filename)
}
func TestMarkdownEpisodeParse(t *testing.T) {
	data, _ := ioutil.ReadFile("episode.md")
	dataBytes := bytes.NewReader(data)
	logger, _ := sparta.NewLogger("info")

	configEntry := Item{}
	specErr := spartaCast.ParseSpartaConfigSpec(dataBytes, &configEntry, logger)

	if specErr != nil {
		t.Fatalf("Failed to parse: %v", specErr)
	}
	ret, _ := json.MarshalIndent(configEntry, "", " ")

	t.Logf("RESULTS: \n%s\n", string(ret))
	//t.Log(fmt.Sprintf("%#v", configEntry))
	t.Logf("Episode: \n%s\n", configEntry.Episode)
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
	t.Log(fmt.Sprintf("%#v", configEntry))
}
