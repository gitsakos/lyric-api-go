package genius

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gitsakos/lyric-api-go/goquery_helpers"
)

// Genius Provider.
type Genius struct {
	accessToken string
}

// New creates an instance of genius provider.
func New(accessToken string) *Genius {
	return &Genius{
		accessToken: accessToken,
	}
}

func search(artist, song, accessToken string) (string, error) {
	url := "http://api.genius.com/search?access_token=" + accessToken + "&q=" + url.PathEscape(artist) + "-" + url.PathEscape(song)

	resp, err := http.Get(url)
	if err != nil {
		log.Println("Error on response.\n[ERRO] -", err)
		return "", err
	}
	defer resp.Body.Close()
	defer io.Copy(ioutil.Discard, resp.Body)

	if resp.StatusCode != 200 {
		return "", errors.New("non 200 error code from API, got " + string(resp.StatusCode) + " : " + resp.Status)
	}

	return parse(resp.Body, artist, song)
}

func parse(data io.Reader, artist string, song string) (string, error) {
	var res map[string]interface{}

	if err := json.NewDecoder(data).Decode(&res); err != nil {
		return "", err
	}
	hits := res["response"].(map[string]interface{})["hits"].([]interface{})
	for _, v := range hits {
		h := v.(map[string]interface{})
		if h["type"] == "song" {
			fullTitle := h["result"].(map[string]interface{})["full_title"].(string)
			if err := validateSearchHit(fullTitle, artist, song); err == nil {
				url := h["result"].(map[string]interface{})["url"].(string)
				return url, nil
			} else {
				log.Println("genius search hit did not match artist or song titles ", err)

			}
		}
	}
	return "", errors.New("no song found")
}

func scrape(url string) (string, error) {
	res, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return "", errors.New("non 200 error code from API, got " + string(res.StatusCode) + " : " + res.Status)
	}
	// Create a goquery document from the HTTP response
	document, err2 := goquery.NewDocumentFromReader(res.Body)
	if err2 != nil {
		return "", err2
	}

	selection := document.Find("#lyrics-root").First()
	var result *goquery.Selection
	selection.Children().Each(func(i int, child *goquery.Selection) {
		lyricsContainerVal, found := child.Attr("data-lyrics-container")
		if found && lyricsContainerVal == "true" {
			if result == nil {
				result = child
			} else {
				result.AppendSelection(child)
			}
		}
	})

	if result == nil {
		return "", errors.New("no lyrics container found")
	}

	return strings.TrimSpace(goquery_helpers.RenderSelection(result, "\n")), nil
}

// Fetch Searches Genius API based on Artist and Song. Then parses the result,
// to get a song and obtaines the url and scrapes it to return the lyrics.
func (g *Genius) Fetch(artist, song string) (string, error) {
	u, err := search(artist, song, g.accessToken)
	if err != nil {
		log.Println("error in genius provider during search while attempting genius provider ", err)
		return "", err
	}

	lyric, err2 := scrape(u)
	if err2 != nil {
		log.Println("error in genius provider during scraping while attempting genius provider ", err2)
		return "", err2
	}
	return lyric, nil
}

func validateSearchHit(hitUrl, artist, song string) error {
	//hitUrl = strings.Trim(hitUrl, "https://genius.com/")

	hitUrl = strings.Replace(strings.ToLower(hitUrl), "-", "", -1)
	artist = strings.ToLower(artist)
	song = strings.ToLower(song)

	artistWords := strings.Split(artist, " ")
	for _, word := range artistWords {
		if !strings.Contains(hitUrl, StripNonAlphanumeric(word)) {
			return errors.New("Invalid search result.  Does not contain artist string")
		}
	}
	songWords := strings.Split(artist, " ")
	for _, word := range songWords {
		if !strings.Contains(hitUrl, StripNonAlphanumeric(word)) {
			return errors.New("Invalid search result.  Does not contain song string")
		}
	}

	return nil
}

func StripNonAlphanumeric(source string) string {
	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		log.Fatal(err)
	}
	return reg.ReplaceAllString(source, "")
}
