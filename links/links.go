package links

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mvdan/xurls"

	"goose/inceptiondb"
)

type Link struct {
	Type  string      `json:"type"` // url|handler|hashtag
	Begin int         `json:"begin"`
	End   int         `json:"end"`
	Text  string      `json:"text"`
	Extra interface{} `json:"extra"`
}

var LinkTypes = []struct {
	name string
	re   *regexp.Regexp
}{
	{"url", xurls.Strict},
	{"handler", regexp.MustCompile(`(^|[^@\w])@(\w{1,15})\b`)},
	{"hashtag", regexp.MustCompile(`(^|[^#\w])#(\w{1,15})\b`)},
	{"code", regexp.MustCompile("`[^`]+`")},
}

func ParseLinks(message string) []*Link {

	result := []*Link{}

	for _, k := range LinkTypes {
		matches := k.re.FindAllStringIndex(message, -1)
		for _, u := range matches {
			begin := u[0]
			end := u[1]
			text := message[begin:end]
			if strings.HasPrefix(text, " ") {
				begin++
				text = message[begin:end]
			}
			if rangeInLinks(result, begin, end) {
				continue
			}
			result = append(result, &Link{
				Type:  k.name,
				Begin: utf8.RuneCountInString(message[0:begin]),
				End:   utf8.RuneCountInString(message[0:end]),
				Text:  text,
			})
		}
	}

	// unique
	return result
}

func Enrich(links []*Link, db *inceptiondb.Client) []*Link {

	result := []*Link{}

	for _, link := range links {
		var skip bool
		switch link.Type {
		case "url":
			skip = enrichLinkUrl(link)
		case "handler":
			skip = enrichLinkHandler(link, db)
		default:
			skip = false
		}
		if skip {
			continue
		}
		result = append(result, link)
	}

	return result
}

func enrichLinkHandler(link *Link, db *inceptiondb.Client) bool {

	mention := link.Text
	mention = strings.TrimPrefix(mention, "@")
	mention = strings.ToLower(mention)

	user := struct {
		ID            string `json:"id"`
		Handle        string `json:"handle"`
		Nick          string `json:"nick"`
		Picture       string `json:"picture"`
		JoinTimestamp int64  `json:"join_timestamp"`
	}{}
	findErr := db.FindOne("users", inceptiondb.FindQuery{
		Index: "by handle",
		Value: mention,
	}, &user)
	if findErr != nil {
		return true // skip
	}

	link.Extra = user

	return false
}

func enrichLinkUrl(link *Link) bool {

	return false
}

func rangeInLinks(links []*Link, begin, end int) bool {
	for _, link := range links {
		if link.Begin <= begin && begin <= link.End {
			return true
		}
		if link.Begin <= end && end <= link.End {
			return true
		}
	}

	return false
}
