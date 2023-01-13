package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/fulldump/box"

	"goose/inceptiondb"
)

func renderHonk(staticsDir string) interface{} {

	template, err := t("honk", staticsDir, "pages/template.gohtml", "pages/honk.gohtml")
	if err != nil {
		panic(err)
	}

	return func(ctx context.Context, w http.ResponseWriter) {

		userHandle := box.GetUrlParameter(ctx, "user-handle")
		honkId := box.GetUrlParameter(ctx, "honk-id")

		selfUrl := `https://goose.blue/user/` + url.PathEscape(userHandle) + `/honk/` + url.PathEscape(honkId)
		w.Header().Set(`Link`, `<`+selfUrl+`>; rel="canonical"`)

		var honk Tweet
		findErr := GetInceptionClient(ctx).FindOne("honks", inceptiondb.FindQuery{
			Index: "by id",
			Value: honkId,
		}, &honk)
		if findErr != nil {
			// todo: render a pretty and user friendly error page
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if honk.Nick != userHandle {
			// todo: render a pretty and user friendly error page
			w.WriteHeader(http.StatusNotFound)
			return
		}

		words := strings.SplitN(honk.Message, " ", 6)

		title := "@" + userHandle + ": " + strings.Join(words[0:len(words)-1], " ") + "..."

		template.Execute(w, map[string]interface{}{
			"title":          title,
			"description":    honk.Message,
			"name":           userHandle,
			"honk":           honk,
			"og_title":       title,
			"og_url":         selfUrl,
			"og_image":       honk.Picture,
			"og_description": honk.Message,
		})
	}
}
