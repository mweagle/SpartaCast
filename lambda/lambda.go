package lambda

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/mweagle/SpartaCast/markson"
	"github.com/sirupsen/logrus"
)

const (
	// PublicKeyPath is the root of files that are
	// public
	PublicKeyPath = "public"

	// KeyComponentFeed is the component for the mp3 output
	KeyComponentFeed = "feed"

	// KeyComponentMetadata is the component for the metadata JSON output
	KeyComponentMetadata = "metadata"

	// FeedConfigName is the name of the feed
	FeedConfigName = "feed.md"

	// KeyProperties is the property name that's in the header
	KeyProperties = "properties"

	// ContentTypeMP3 is the IANA media type for MP3
	ContentTypeMP3 = "audio/mpeg"
)

func manifestKeyPath(baseKeyName string) string {
	return fmt.Sprintf("%s/%s/%s.json",
		PublicKeyPath,
		KeyComponentMetadata,
		baseKeyName)
}

// ParseSpartaConfigSpec returns an EpisodeSpec input
// and returns the data
func ParseSpartaConfigSpec(input io.Reader,
	configEntry interface{},
	logger *logrus.Logger) error {

	return markson.UnmarshalMarkson(input,
		KeyProperties,
		configEntry,
		logger)
}

// Feed represents the feed information from a Markdown file.
type Feed struct {
	Title          string `json:"title,omitempty"`
	AuthorName     string `json:"authorname"`
	AuthorEmail    string `json:"authoremail"`
	Image          string `json:"image,omitempty"`
	Link           string `json:"link,omitempty"`
	Description    string `json:"description,omitempty"`
	Category       string `json:"category,omitempty"`
	Subcategory    string `json:"subcategory,omitempty"`
	Cloud          string `json:"cloud,omitempty"`
	Copyright      string `json:"copyright,omitempty"`
	Docs           string `json:"docs,omitempty"`
	Generator      string `json:"generator,omitempty"`
	Language       string `json:"language,omitempty"`
	LastBuildDate  string `json:"lastBuildDate,omitempty"`
	ManagingEditor string `json:"managingEditor,omitempty"`
	PubDate        string `json:"pubDate,omitempty"`
	Rating         string `json:"rating,omitempty"`
	SkipHours      string `json:"skipHours,omitempty"`
	SkipDays       string `json:"skipDays,omitempty"`
	SubTitle       string `json:"subtitle"`
	TTL            string `json:"ttl,omitempty"`
	WebMaster      string `json:"webMaster,omitempty"`
	IAuthor        string `json:"itunes:author,omitempty"`
	IExplicit      string `json:"itunes:explicit,omitempty"`
	IComplete      string `json:"itunes:complete,omitempty"`
}

// Item represents an item
type Item struct {
	SelfLink            string `json:"selfLink"`
	Image               string `json:"image,omitempty"`
	GUID                string `json:"guid"`
	Title               string `json:"title"`
	Link                string `json:"link"`
	EnclosureLink       string `json:"enclosureLink"`
	EnclosureByteLength int64  `json:"enclosureByteLength"`
	Description         string `json:"description"`
	AuthorName          string `json:"authorname"`
	AuthorEmail         string `json:"authoremail"`
	Category            string `json:"category"`
	Comments            string `json:"comments"`
	Source              string `json:"source"`
	PubDate             string `json:"pubDate"`
	SubTitle            string `json:"subtitle"`
	IExplicit           string `json:"itunes:explicit"`
	IIsClosedCaptioned  string `json:"itunes:isClosedCaptioned"`
	IOrder              string `json:"itunes:order"`
	PollyVoiceID        string `json:"polly:voiceID"`
	PollyEngineType     string `json:"polly:engineType"`
	PollyLanguageCode   string `json:"polly:languageCode"`
	Episode             string `json:"episode"`
}

func keyPathFromS3URI(s3URI string, bucketName string) (string, error) {

	parsedURI, parsedURIErr := url.Parse(s3URI)
	if parsedURIErr != nil {
		return "", parsedURIErr
	}
	strippedPath := strings.TrimPrefix(parsedURI.Path,
		fmt.Sprintf("/%s/", bucketName))

	return strippedPath, nil

}
